/*
问题描述：对于分布式缓存，当一个节点收到请求，如果该节点没有缓存值，
该从谁那里获取数据？自己？还是1，2，3号节点？
假设采用随机的算法选取某个节点获取，那第二次只有1/10的可能性从该节点（最优），
9/10的可能性选其他节点（耗时间），这样做一是耗时间，二是各个节点上缓存相同的值，浪费存储空间

以上问题可以这样解决：对于给定的key，每一次都选取同一个节点。把key的每一个字符ASCII 码加起来，
除以10（假设有10个节点）取余数，此时能保证对于同一个key 都访问同一个节点

引入的问题是：节点变化了怎么办？hash(key) % 10 变成了 hash(key) % 9， 意味着几乎缓存值对应
的节点都发生了改变，即几乎所有的缓存值都失效了。节点接收到对应的请求时，均需要重新去数据源获取数据，
容易引起 缓存雪崩（缓存同一时间全部失效）
*/

package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash maps bytes to uint32
type Hash func(data []byte) uint32

// Map constains all hashed keys
type Map struct {
	hash     Hash
	replicas int            // 虚拟节点倍数
	keys     []int          // sorted， 哈希环
	hashMap  map[int]string // 虚拟节点与真实节点的映射表，键是虚拟节点的哈希值，值是真实节点的名称
}

// New creates a Map instance
func New(replicas int, fn Hash) *Map {
	m := &Map{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}

	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}

	return m
}

// 每添加一个真实的节点，就添加 m.replicas 个虚拟节点，
// 虚拟节点的名称就是 strconv.Itoa(i) + key
// 使用m.hash() 计算虚拟几点的哈希值，使用append(m.keys, hash) 添加到环上
// 在hasMap 中添加虚拟节点和真实节点的映射关系
// Add adds some keys to the hash
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	hash := int(m.hash([]byte(key)))
	// Binary search for appropriate replica.
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})

	return m.hashMap[m.keys[idx%len(m.keys)]]
}
