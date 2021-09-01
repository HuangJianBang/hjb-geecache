/*
分布式缓存需要实现节点健通信，建立机遇HTTP的通信机制是比较常见和简单的做法
如果一个节点启动了HTTP 服务，那么这个节点就可以被其他节点访问。
现在我们为单机节点搭建HTTP Server，实际操作中其他单机节点上也安装有HTTP服务，
即运行有我们写的geeache程序
*/

package geecache

import (
	"fmt"
	"hjb-geechche/geecache/consistenthash"
	pb "hjb-geechche/geecache/geecachepb"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// 新建一个客户端的类
type httpGetter struct {
	baseURL string // 表示要访问节点的的地址，比http://example.com/_geecache/
}

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)

	res, err := http.Get(u)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server return: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	return nil
}

var _ PeerGetter = (*httpGetter)(nil)

// HTTPPOOL implements PeerPicker for a pool of HTTP peers
type HTTPPool struct {
	// this peer's base URL, e.g. "https://example.net:8000"
	self     string
	basePath string //向https://example.net:8000/basePath发出请求，才会被节点响应（不一定是本机的节点响应）

	mu          sync.Mutex // guards peers and httpGetters
	peers       *consistenthash.Map
	httpGetters map[string]*httpGetter //keyed by e.g. "http://10.0.0.2:8000"
}

// Set 实例化了一致性哈希算法，并且添加了传入的节点
// 并且为每一个节点创建了一个HTTP客户端 httpGetter
// PickerPeer() 包装了一致性哈希算法的 Get()方法，根据具体的key选择节点,选择节点，返回节点对应的HTTP客户端
// 至此，HTTPool 既具备了提供HTTP服务能力，也具备了根据具体 key，创建HTTP客户端从远程节点获取缓存能力
// Set updates the pool's list of peers
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// Picker pick a peer according to key
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}

	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

// 相当于HTTP服务端
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServerHTTP handle all http request
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 判断访问路径的前缀是否为 basePath ，不是则返回错误
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}

	p.Log("%s %s", r.Method, r.URL.Path)
	// 约定节点收到的请求地址是 以下格式的，包括节点上的group name，key
	// /<basepath>/<groupname>/<key> required
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	// 从本节点上获取对应的group
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	// 找到相应的group,再根据key 找相应的 value
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the value to the response body as a proto message
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 设置返回内容的格式，并将group 中查找到的内容写入返回
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(body)
}
