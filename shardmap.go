package cache

import (
	"hash"
	"hash/fnv"
	"sync"
	"time"
)

//TODO shard cache for cahing

type shardmap struct {
	shards []LockMap
	hash   hash.Hash64
}

type LockMap struct {
	sync.RWMutex
	m map[uint64]Item
}

type Item struct {
	Object     interface{}
	Expiration int64
}

func NewShardMap() shardmap {
	//TODO param shards
	shards := 10
	return shardmap{
		shards: make([]LockMap, 10),
		hash:   fnv.New64a(),
	}
}

// Returns true if the item has expired.
func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

func (s *shardmap) Add(key string, value Item) error {

}

func (s *shardmap) Replace(key string, value Item) error {

}

func (s *shardmap) Set(key string, value Item) error {

}

func (s *shardmap) Delete(key string) error {

}

func (s *shardmap) Flush() error {

}

func (s *shardmap) calcHash(str string) (res uint64) {
	s.hash.Write([]byte(str))
	res = s.hash.Sum64()
	s.hash.Reset()
	return
}
