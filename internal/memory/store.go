package memory

import (
	"encoding/json"
	bolt "go.etcd.io/bbolt"
	"gollm-mini/internal/helper"
	"gollm-mini/internal/types"
	"sync"
)

const (
	dbPath       = "memory.db"
	maxCtxTok    = 3000
	bucketPrefix = "session_"
)

var (
	db   *bolt.DB
	once sync.Once
)

func open() *bolt.DB {
	once.Do(func() {
		db, _ = bolt.Open(dbPath, 0600, nil)
	})
	return db
}

func bucketName(id string) []byte { return []byte(bucketPrefix + id) }

// Load returns history truncated to maxCtxTok tokens (oldest first)
func Load(id string) ([]types.Message, error) {
	var msgs []types.Message
	db := open()
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName(id))
		if b == nil {
			return nil
		}

		// 新格式：整个对话历史保存在 history
		if data := b.Get([]byte("history")); data != nil {
			return json.Unmarshal(data, &msgs)
		}

		// 兼容旧格式：逐条存
		return b.ForEach(func(_, v []byte) error {
			var m types.Message
			if err := json.Unmarshal(v, &m); err != nil {
				return err
			}
			msgs = append(msgs, m)
			return nil
		})
	})

	// 截断
	return helper.TruncateMessages(msgs, maxCtxTok), err
}

// Append writes user & assistant message pair
func Append(sessionID string, msgs []types.Message) error {
	return open().Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte(bucketPrefix + sessionID))

		// 读取旧历史
		var hist []types.Message
		_ = json.Unmarshal(b.Get([]byte("history")), &hist)

		hist = append(hist, msgs...)
		data, _ := json.Marshal(hist)
		return b.Put([]byte("history"), data)
	})
}

func Delete(sessionID string) error {
	return open().Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket(bucketName(sessionID))
	})
}
