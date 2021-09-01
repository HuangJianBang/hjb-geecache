package geecache

/*
在支持单机的并发缓存之前，我们先定义一个只读的数据结构来表示缓存的值
因为我们是不能对缓存的值进行修改的
*/

type ByteView struct {
	b []byte
}

// 因为我们要求一个Cache实例中双向链表节点的值的类型必须实现Value接口，
// 所以我们这个只读类型的缓存值要实现Len()方法
func (v ByteView) Len() int {
	return len(v.b)
}

func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
