package lru

import (
	"container/list"
)

// Cache is a LRU cache. It is not safe for concurrent access.
// 这是存储数据的最底层数据结构 每个Cache 实例包含一个双向链表，
// 双向链表的节点存储键值对中的值，为了方便删除节点的时候顺便删除键，
// 双向链表中也存储值对应的键
type Cache struct {
	maxBytes int64                    // 一个Cache 实例所允许的最大存储值
	nbytes   int64                    // 当前Cache 实例已经使用的值
	ll       *list.List               // 双向链表，每个节点存储一个数据点
	cache    map[string]*list.Element // Cache 实例中的数据入口，是一个map 类型，
	// 根据传给实例的key 访问双向链表中的节点
	// optional and executed when an entry is purged.
	OnEvicted func(key string, value Value)
}

// Cache 实例中存储节点的数据结构
type entry struct {
	key   string
	value Value
}

// 双向链表节点中值的类型，要求其实现了Value接口
type Value interface {
	Len() int
}

// Cache 的构造函数，需要传入每个实例所拥有的最大值的
func New(maxBytes int64, OnEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache:     make(map[string]*list.Element),
		OnEvicted: OnEvicted,
	}
}

// 在一个Cache 实例中查找相应的值，参数是一个key
func (c *Cache) Get(key string) (value Value, ok bool) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		return kv.value, true
	}
	return
}

func (c *Cache) RemoveOldest() {
	ele := c.ll.Back()
	if ele != nil {
		c.ll.Remove(ele)
		kv := ele.Value.(*entry)                               // 将双向链表中list.Element 类型的值，转换成entry类型
		delete(c.cache, kv.key)                                // 删除掉 key
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len()) // 更新Cache 实例中现在当下的内存占用大小

		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// 往Cache 实例里面新增一个键值对
func (c *Cache) Add(key string, value Value) {
	if ele, ok := c.cache[key]; ok {
		c.ll.MoveToFront(ele)
		kv := ele.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	} else {
		ele := c.ll.PushFront(&entry{key, value})
		c.cache[key] = ele
		c.nbytes += int64(len(key)) + int64(value.Len())
	}

	// 检测插入新数据后是否超过了一个Cache 实例所允许的最大值，是则删除掉头节点
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.RemoveOldest()
	}
}

// 为了了解每一个Cache中的双向链表，我们定一个返回双向链表长度的函数
func (c *Cache) Len() int {
	return c.ll.Len()
}
