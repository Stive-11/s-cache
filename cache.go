package cache

import (
	"fmt"
	"hash/fnv"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type shardmap struct {
	shards     []*lockMap
	shardCount uint64
}

type lockMap struct {
	sync.RWMutex
	m map[uint64]Item
}

type Item struct {
	Object     interface{}
	Expiration int64
}

type Cache struct {
	*cache
	// If this is confusing, see the comment at the bottom of New()
}

type cache struct {
	defaultExpiration time.Duration
	shards            shardmap
	janitor           *janitor
	Statistic         stats
}

type stats struct {
	ItemsCount, GetCount, SetCount, ReplaceCount, DeleteCount, AddCount, DeleteExpired int32
}

func newShardMap() shardmap {
	count := uint64(10)

	smap := shardmap{
		shards:     make([]*lockMap, count),
		shardCount: count,
	}
	for i, _ := range smap.shards {
		smap.shards[i] = &lockMap{m: make(map[uint64]Item)}
	}
	return smap
}

// Returns true if the item has expired.
func (item Item) expired() bool {
	if item.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > item.Expiration
}

func (c *cache) GetShard(key uint64) *lockMap {
	return c.shards.shards[key%c.shards.shardCount]
}

func calcHash(str string) uint64 {
	hash := fnv.New64a()
	hash.Write([]byte(str))
	return hash.Sum64()
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache) Set(k string, x interface{}, d time.Duration) {
	// "Inlining" of set
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}

	key := calcHash(k)
	shard := c.GetShard(key)
	atomic.AddInt32(&c.Statistic.SetCount, 1)
	atomic.AddInt32(&c.Statistic.ItemsCount, 1)
	shard.Lock()
	shard.m[key] = Item{
		Object:     x,
		Expiration: e,
	}
	shard.Unlock()
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache) Add(k string, x interface{}, d time.Duration) error {
	_, found := c.Get(k)
	if found {
		return fmt.Errorf("Item %s already exists", k)
	}
	atomic.AddInt32(&c.Statistic.AddCount, 1)
	atomic.AddInt32(&c.Statistic.ItemsCount, 1)
	c.Set(k, x, d)
	return nil
}

// Set a new value for the cache key only if it already exists, and the existing
// item hasn't expired. Returns an error otherwise.
func (c *cache) Replace(k string, x interface{}, d time.Duration) error {
	_, found := c.Get(k)
	if !found {
		return fmt.Errorf("Item %s doesn't exist", k)
	}
	atomic.AddInt32(&c.Statistic.ReplaceCount, 1)
	atomic.AddInt32(&c.Statistic.ItemsCount, 1)
	c.Set(k, x, d)
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k string) (interface{}, bool) {
	// "Inlining" of get and expired
	key := calcHash(k)
	shard := c.GetShard(key)
	shard.RLock()
	item, found := shard.m[key]
	shard.RUnlock()

	if !found {
		return nil, false
	}

	if item.expired() {
		return nil, false
	}

	atomic.AddInt32(&c.Statistic.GetCount, 1)
	return item.Object, true
}

func (c *cache) Delete(k string) (interface{}, bool) {
	key := calcHash(k)
	shard := c.GetShard(key)
	v, f := shard.m[key]

	if f {
		atomic.AddInt32(&c.Statistic.ItemsCount, -1)
		atomic.AddInt32(&c.Statistic.DeleteCount, 1)
		delete(shard.m, key)
		return v.Object, true
	}
	return nil, false
}

// Delete all expired items from the cache.
func (c *cache) DeleteExpired() {
	now := time.Now().UnixNano()
	for i, _ := range c.shards.shards {
		sh := c.shards.shards[i]
		sh.Lock()
		for k, v := range sh.m {
			if v.Expiration > 0 && now > v.Expiration {
				atomic.AddInt32(&c.Statistic.DeleteExpired, 1)
				atomic.AddInt32(&c.Statistic.ItemsCount, -1)
				delete(sh.m, k)
			}
		}
		sh.Unlock()
	}

}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up. Equivalent to len(c.Items()).
func (c *cache) ItemCount() int { //TODO maybe get from statistics ?
	size := 0
	for _, m := range c.shards.shards {
		m.RLock()
		size += len(m.m)
		m.RUnlock()
	}
	return size
}

// Delete all items from the cache.
func (c *cache) Flush() {
	c.shards = newShardMap() //TODO init with params
	c.Statistic.ItemsCount = 0
}

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) Run(c *cache) {
	j.stop = make(chan bool)
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c *Cache) {
	c.janitor.stop <- true
}

func runJanitor(c *cache, ci time.Duration) {
	j := &janitor{
		Interval: ci,
	}
	c.janitor = j
	go j.Run(c)
}

func newCache(de time.Duration) *cache {
	if de == 0 {
		de = -1
	}
	c := &cache{
		defaultExpiration: de,
		shards:            newShardMap(),
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration) *Cache {
	c := newCache(de)
	// This trick ensures that the janitor goroutine (which--granted it
	// was enabled--is running DeleteExpired on c forever) does not keep
	// the returned C object from being garbage collected. When it is
	// garbage collected, the finalizer stops the janitor goroutine, after
	// which c can be collected.
	C := &Cache{c}
	if ci > 0 {
		runJanitor(c, ci)
		runtime.SetFinalizer(C, stopJanitor)
	}
	return C
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	return newCacheWithJanitor(defaultExpiration, cleanupInterval)
}
