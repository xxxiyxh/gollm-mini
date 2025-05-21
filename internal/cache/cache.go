package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"gollm-mini/internal/types"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const bucket = "prompt_cache"

const (
	TTL       = 24 * time.Hour // 每条缓存有效期
	MaxEntry  = 100_000        // 总缓存条数上限
	EvictSize = 500            // 超限时每次清理数量
)

// ---- 单例 DB ----
var (
	db   *bolt.DB
	once sync.Once
)

// open 单例
func openDB() *bolt.DB {
	once.Do(func() {
		db, _ = bolt.Open("prompt_cache.db", 0600, &bolt.Options{Timeout: 1 * time.Second})
		_ = db.Update(func(tx *bolt.Tx) error {
			_, _ = tx.CreateBucketIfNotExists([]byte(bucket))
			return nil
		})
	})
	return db
}

// KeyFromMessages 根据 provider+model+messages 生成 SHA256
func KeyFromMessages(provider, model string, msgs any) string {
	b, _ := json.Marshal(msgs)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%s|%s|%x", provider, model, sum)
}

// Value 保存模型文本+Usage
type Value struct {
	Text  string      `json:"text"`
	Usage types.Usage `json:"usage"`
	At    time.Time   `json:"at"`
}

// Get 查询缓存
func Get(key string) (val Value, ok bool) {
	db := openDB()
	_ = db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bucket)).Get([]byte(key))
		if v == nil {
			return nil
		}
		_ = json.Unmarshal(v, &val)

		// TTL 判定
		if time.Since(val.At) > TTL {
			ok = false
			return nil
		}
		ok = true
		return nil
	})
	return
}

// Put 写入缓存
func Put(key string, val Value) {
	db := openDB()
	_ = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))

		// 检查是否超限
		stats := b.Stats()
		if stats.KeyN >= MaxEntry {
			evictOldest(b)
		}

		val.At = time.Now()
		data, _ := json.Marshal(val)
		return b.Put([]byte(key), data)
	})
}

func evictOldest(b *bolt.Bucket) {
	c := b.Cursor()
	count := 0
	for k, v := c.First(); k != nil && count < EvictSize; k, v = c.Next() {
		var val Value
		if err := json.Unmarshal(v, &val); err == nil {
			// 删除过期 or 最老的
			if time.Since(val.At) > TTL {
				_ = b.Delete(k)
				count++
			}
		}
	}

	if count < EvictSize {
		c := b.Cursor()
		for k, _ := c.First(); k != nil && count < EvictSize; k, _ = c.Next() {
			_ = b.Delete(k)
			count++
		}
	}
}

func ClearAll() error {
	db := openDB()
	return db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket([]byte(bucket))
	})
}

func DeleteKey(key string) error {
	db := openDB()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Delete([]byte(key))
	})
}

func DeletePrefix(prefix string) error {
	db := openDB()
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil && bytes.HasPrefix(k, []byte(prefix)); k, _ = c.Next() {
			_ = b.Delete(k)
		}
		return nil
	})
}
