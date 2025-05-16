package optimizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

const recordBucket = "opt_records"

type Record struct {
	Template string    `json:"template"` // "sum:1"
	Input    string    `json:"input"`
	Answer   string    `json:"answer"`
	Score    float64   `json:"score"`
	Provider string    `json:"provider"`
	Model    string    `json:"model"`
	At       time.Time `json:"at"`
}

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, nil)
	return &Store{db: db}, err
}

func (s *Store) Save(rec Record) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(recordBucket))
		key := fmt.Sprintf("%s/%d", rec.Template, time.Now().UnixNano())
		data, _ := json.Marshal(rec)
		return b.Put([]byte(key), data)
	})
}

func (s *Store) List(template string) ([]Record, error) {
	var list []Record
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(recordBucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(template + ":")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var r Record
			_ = json.Unmarshal(v, &r)
			list = append(list, r)
		}
		return nil
	})
	return list, err
}
