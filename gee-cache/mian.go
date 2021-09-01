package main

import (
	"flag"
	"fmt"
	"hjb-geechche/geecache"
	"log"
	"net/http"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *geecache.Group {
	return geecache.NewGroup("scores", 2<<10, geecache.GetterFunc(
		func(key string) ([]byte, error) {
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

/*
根据第一次go build -o server ./server -port=8001 在 http://localhost:8001上开启cacheserver (此时是作为服务端)
然后在本节点http://localhost:8001 将节点 http://localhost:8001,http://localhost:8002,http://localhost:8003 注册到自己的HTTPPool上
三次使用 go build 完成在所有节点上开启cacheserver,并将其余节点注册到各自的HTTPPool 上
*/
func startCacheServer(addr string, addrs []string, gee *geecache.Group) {
	peers := geecache.NewHTTPPool(addr)
	peers.Set(addrs...)
	gee.RegisterPeers(peers)
	log.Println("geecache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

func startAPIServer(apiAddr string, gee *geecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := gee.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())
		}))

	log.Println("fonted server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func main() {

	/*
		#!/bin/bash
		trap "rm server;kill 0" EXIT

		go build -o server
		./server -port=8001 &
		./server -port=8002 &
		./server -port=8003 -api=1 &

		sleep 2
		echo ">>> start test"
		curl "http://localhost:9999/api?key=Tom" &
		curl "http://localhost:9999/api?key=Tom" &
		curl "http://localhost:9999/api?key=Tom" &

		wait
	*/

	/*
		总体思想：
		现在有四个节点：http://localhost:8001,http://localhost:8002,http://localhost:8003,http://localhost:9999
		http://localhost:9999 作为一个api 节点，即与用户交互的点，用户只知道这个节点存在，不知道前面三个节点存在，所以用户会向
		http://localhost:9999 发起获取缓存的请求，由http://localhost:9999 接收请求，并根据请求决定去哪个节点获取，或者本地获取（请求长什么样？怎么处理）

	*/
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	gee := createGroup()
	if api {
		go startAPIServer(apiAddr, gee)
	}

	// 依次开启http://localhost:8001，http://localhost:8002，http://localhost:8003的缓存服务，在http://localhost:8003的时候api为true，此时开启本地http://localhost:9999服务
	startCacheServer(addrMap[port], []string(addrs), gee)
}
