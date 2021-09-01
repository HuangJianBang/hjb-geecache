/*
这部分开始定义Group 数据结构。Group 是对外开放，即与外面进行交互的窗口

设计Group 的时候我们需要考虑这样一个问题：假如要访问的数据本地不存在，那该去哪里访问？
这里的做法是：由编写程序的的人员（即调用geecache 的程序人员决定）
所以这里定义一个回调，当缓存不存在，由该回调函数决定去哪里寻找缓存值
*/
package geecache

import (
	"fmt"
	pb "hjb-geechche/geecache/geecachepb"
	"hjb-geechche/geecache/singleflight"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type Group struct {
	name      string
	getter    Getter
	mainCache cache
	peers     PeerPicker

	// use singleflight.Group to make sure that each key is only fetched once
	loader *singleflight.Group
}

// RegisterPeers register a PeerPicker for choosing remote peer
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}

	g.peers = peers
}

func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	return g.load(key)
}

// 使用PickPeer()方法选择节点，若非本机节点，则调用 getFromPe() 从远程获取
// 若是本机节点或者失败，则回退到getLocally()
func (g *Group) load(key string) (value ByteView, err error) {
	// each key is only fetched once (eitherlocally or remotely)
	// regardless of the number of concurrent callers
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[GeeCache] Failed to get from peer", err)
			}
		}

		return g.getLocally(key)
	})

	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 使用 PeerGetter 接口的httpGetter从远程访问节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}

	res := &pb.Response{}
	err := peer.Get(req, res)

	if err != nil {
		return ByteView{}, err
	}

	return ByteView{b: res.Value}, nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	value := ByteView{b: cloneBytes(bytes)}
	g.populateCache(key, value)
	return value, nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// 每次新增加一个group 实例，都要在全局变量
// groups 这里注册
var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}

	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleflight.Group{},
	}

	groups[name] = g
	return g
}

/*假设小明在项目中导入了geecahe。 则他会这样写：
import geecache
geecache.GetGroup(groupName)
*/

// 返回在全局变量groups 中注册的name (Group 类型)
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}
