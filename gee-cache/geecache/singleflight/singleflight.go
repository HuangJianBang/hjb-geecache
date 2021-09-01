/*
我们现在拥有这样一个机制：我们有不同的节点，当某个节点收到某个key的请求的时候，
带有相同key请求的请求总是能够被转发到同一个节点，那么问题来了：当一个相同的请求被并发
很多次的时候，相同的请求会被同时转移到同一个目标节点，会造成缓存雪崩、缓存击穿、缓存穿透

采取的总体思路：若访问量不高，对相同的访问请求，每一次的请求都会被处理，
但是串行的处理，并不会影响性能。若是同一个请求大量的并发处理，则会触发机制，相同的请求智慧返回一次值，
若是来自其他客户的请求，相同的请求也是共享一个相同的值并不会造成节点的多次查找
*/

package singleflight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex // protect m
	m  map[string]*call
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val, c.err
}
