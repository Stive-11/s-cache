package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

//TODO shard cache for cahing

type shardmap struct {
	shards     []LockMap
	shardCount uint64
}

type LockMap struct {
	sync.RWMutex
	m map[uint64]*Item
}

type Item struct {
	Object     interface{}
	Expiration int64
}

func NewShardMap() shardmap {
	//TODO param shards
	return shardmap{
		shards:     make([]LockMap, 10),
		shardCount: uint64(10),
	}
}

func (s *shardmap) Size() int {
	size := 0
	for _, m := range s.shards {
		m.RLock()
		size += len(m.m)
		m.RUnlock()
	}
	return size
}

// Returns true if the item has expired.
func (item Item) Expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

func (s *shardmap) Add(key string, value Item) error {
	return nil
}

func (s *shardmap) Replace(key string, value Item) error {
	return nil
}

func (s *shardmap) Set(key string, value Item) error {
	return nil
}

func (s *shardmap) Get(key string) (*Item, error) {
	return nil, nil
}

func (s *shardmap) Delete(key string) (*Item, error) {
	return nil, nil
}

func (s *shardmap) Flush() error {
	return nil
}

func (s *shardmap) getShard(key uint64) LockMap {
	return s.shards[key%s.shardCount]
}

func calcHash(str string) uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(str))
	return hash.Sum64()
}
