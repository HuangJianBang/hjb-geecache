/*
现在我们已经为每台节点实现了HTTP服务端功能，可以接受来自其他节点的请求
也已经制定好了策略：根据请求选取节点的策略，consistenhash
问题来了：我现在根据策略选好了应该访问哪个节点，现在我需要和目标节点通信，
即怎么将的请求茶传送出去？现在开始为每个节点添加HTTP客户端的功能，
这样每个节点可以通过HTTP客户端发起HTTP请求去访问目标节点
*/
package geecache

import pb "hjb-geechche/geecache/geecachepb"

/*
PeerPicker 中的 PickPeer() 方法用于根据传入的 key 选择相应节点PeerGetter
PeerGetter 的Get() 方法用于从对应group 查找缓存值.PeerGetter对应于本项目中的
HTTP客户端
*/
// PeerPicker is the interface that must be implemented to locate
// the peer that owns a specific key
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter is the interface that must be implemented by a peer.
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
