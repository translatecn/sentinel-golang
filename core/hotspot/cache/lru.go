package cache

import (
	"container/list"
	"github.com/pkg/errors"
)

type EvictCallback func(key interface{}, value interface{}) // 驱逐回调

// LRU 非线程安全
type LRU struct {
	size      int
	evictList *list.List
	items     map[interface{}]*list.Element // value 是一个指针
	onEvict   EvictCallback
}

type entry struct {
	key   interface{}
	value interface{}
}

func NewLRU(size int, onEvict EvictCallback) (*LRU, error) {
	if size <= 0 {
		return nil, errors.New("must provide a positive size")
	}
	c := &LRU{
		size:      size,
		evictList: list.New(),
		items:     make(map[interface{}]*list.Element, 64),
		onEvict:   onEvict,
	}
	return c, nil
}

// Purge 用于完全清除缓存。
func (c *LRU) Purge() {
	for k, v := range c.items {
		if c.onEvict != nil {
			c.onEvict(k, v.Value.(*entry).value)
		}
		delete(c.items, k)
	}
	c.evictList.Init()
}

func (c *LRU) Add(key, value interface{}) {
	// Check for existing item
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*entry).value = value
		return
	}

	// Add new item
	ent := &entry{key, value}
	entry := c.evictList.PushFront(ent)
	c.items[key] = entry

	evict := c.evictList.Len() > c.size
	// Verify size not exceeded
	if evict {
		c.removeOldest()
	}
	return
}

// AddIfAbsent adds item only if key is not existed.
func (c *LRU) AddIfAbsent(key interface{}, value interface{}) (priorValue interface{}) {
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		if ent.Value == nil {
			return nil
		}
		return ent.Value.(*entry).value
	}
	// Add new item
	ent := &entry{key, value}
	entry := c.evictList.PushFront(ent)
	c.items[key] = entry

	evict := c.evictList.Len() > c.size
	// Verify size not exceeded
	if evict {
		c.removeOldest()
	}
	return nil
}

// Get looks up a key's value from the cache.
func (c *LRU) Get(key interface{}) (value interface{}, isFound bool) {
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		if ent.Value.(*entry) == nil {
			return nil, false
		}
		return ent.Value.(*entry).value, true
	}
	return
}

func (c *LRU) Contains(key interface{}) (ok bool) {
	_, ok = c.items[key]
	return ok
}

// Peek returns the key value (or undefined if not found) without updating
// the "recently used"-ness of the key.
func (c *LRU) Peek(key interface{}) (value interface{}, isFound bool) {
	var ent *list.Element
	if ent, isFound = c.items[key]; isFound {
		return ent.Value.(*entry).value, isFound
	}
	return nil, isFound
}

func (c *LRU) Remove(key interface{}) (isFound bool) {
	var ent *list.Element
	if ent, isFound = c.items[key]; isFound {
		c.removeElement(ent)
		return
	}
	return
}

func (c *LRU) RemoveOldest() (key interface{}, value interface{}, ok bool) {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
		kv := ent.Value.(*entry)
		return kv.key, kv.value, true
	}
	return nil, nil, false
}

func (c *LRU) GetOldest() (key interface{}, value interface{}, ok bool) {
	ent := c.evictList.Back()
	if ent != nil {
		kv := ent.Value.(*entry)
		return kv.key, kv.value, true
	}
	return nil, nil, false
}

func (c *LRU) Keys() []interface{} {
	keys := make([]interface{}, len(c.items))
	i := 0
	for ent := c.evictList.Back(); ent != nil; ent = ent.Prev() {
		keys[i] = ent.Value.(*entry).key
		i++
	}
	return keys
}

func (c *LRU) Len() int {
	return c.evictList.Len()
}

func (c *LRU) Resize(size int) (evicted int) {
	diff := c.Len() - size
	if diff < 0 {
		diff = 0
	}
	for i := 0; i < diff; i++ {
		c.removeOldest()
	}
	c.size = size
	return diff
}

func (c *LRU) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

func (c *LRU) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	if e.Value == nil {
		return
	}
	kv, ok := e.Value.(*entry)
	if !ok {
		return
	}
	delete(c.items, kv.key)
	if c.onEvict != nil {
		c.onEvict(kv.key, kv.value)
	}
}
