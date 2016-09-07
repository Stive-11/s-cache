package cache

import (
	"fmt"
	"runtime"
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

type Cache struct {
	*cache
	// If this is confusing, see the comment at the bottom of New()
}

type cache struct {
	defaultExpiration time.Duration
	items             shardmap
	janitor           *janitor
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

	c.items.Add(k, Item{
		Object:     x,
		Expiration: e,
	})
}

// Add an item to the cache only if an item doesn't already exist for the given
// key, or if the existing item has expired. Returns an error otherwise.
func (c *cache) Add(k string, x interface{}, d time.Duration) error {
	_, found := c.Get(k)
	if found {
		return fmt.Errorf("Item %s already exists", k)
	}
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
	c.Set(k, x, d)
	return nil
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k string) (interface{}, bool) {
	// "Inlining" of get and Expired
	item, _ := c.items.Get(k)
	if item != nil {
		return nil, false
	}
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return nil, false
		}
	}
	return item.Object, true
}

func (c *cache) Delete(k string) (interface{}, bool) {

	v, _ := c.items.Delete(k)
	if v != nil {
		return v.Object, true
	}
	return nil, false
}

// Delete all expired items from the cache.
//TODO move to shard map
func (c *cache) DeleteExpired() {
	//	now := time.Now().UnixNano()
	//	for k, v := range c.items {
	//	// "Inlining" of expired
	//if v.Expiration > 0 && now > v.Expiration {
	//c.Delete(k)
	//}
	//}
	//
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up. Equivalent to len(c.Items()).
func (c *cache) ItemCount() int {
	return c.items.Size()
}

// Delete all items from the cache.
func (c *cache) Flush() {
	c.items = NewShardMap() //TODO init with params
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
		items:             NewShardMap(),
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
