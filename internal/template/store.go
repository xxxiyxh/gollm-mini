package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
)

const bucket = "prompts"

type Template struct {
	Name    string   `json:"name"`
	Version int      `json:"version"`
	System  string   `json:"system"` // 系统指令
	Content string   `json:"content"`
	Vars    []string `json:"vars,omitempty"`
	Parts
	CreatedAt time.Time `json:"created_at"`
}

type Store struct{ db *bolt.DB }

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0600, nil)
	return &Store{db: db}, err
}

func (s *Store) Save(tpl Template) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(bucket))
		data, _ := json.Marshal(tpl)
		return b.Put([]byte(tplKey(tpl.Name, tpl.Version)), data)
	})
}

func (s *Store) Get(name string, version int) (Template, error) {
	var tpl Template
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return errors.New("no bucket")
		}
		v := b.Get([]byte(tplKey(name, version)))
		if v == nil {
			return errors.New("not found")
		}
		return json.Unmarshal(v, &tpl)
	})
	return tpl, err
}

func (s *Store) Latest(name string) (Template, error) {
	var latest Template
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return errors.New("no bucket")
		}
		c := b.Cursor()
		prefix := []byte(name + ":")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			_ = json.Unmarshal(v, &latest)
		}
		return nil
	})
	if latest.Name == "" {
		return latest, errors.New("not found")
	}
	return latest, err
}

// List 返回同名模板所有版本（按版本升序）
func (s *Store) List(name string) ([]Template, error) {
	var list []Template
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		prefix := []byte(name + ":")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var t Template
			_ = json.Unmarshal(v, &t)
			list = append(list, t)
		}
		return nil
	})
	return list, err
}

// Delete (name, version) 删除指定版本
func (s *Store) Delete(name string, version int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(tplKey(name, version)))
	})
}

// ListAllLatest 返回“每个模板名的最新版本”切片
func (s *Store) ListAllLatest() ([]Template, error) {
	latest := make(map[string]Template)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var t Template
			_ = json.Unmarshal(v, &t)
			if cur, ok := latest[t.Name]; !ok || t.Version > cur.Version {
				latest[t.Name] = t
			}
		}
		return nil
	})

	// 组装为切片
	list := make([]Template, 0, len(latest))
	for _, v := range latest {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, err
}

func tplKey(name string, ver int) string { return fmt.Sprintf("%s:%d", name, ver) }
