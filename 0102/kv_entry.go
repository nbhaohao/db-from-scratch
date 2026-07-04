package db0102

import (
	"encoding/binary"
	"io"
)

type Entry struct {
	key []byte
	val []byte
}

// 你来实现（格式：| key 大小 4B | val 大小 4B | key 数据 | val 数据 |）：
//  1. 一次性分配好长度：4 + 4 + len(key) + len(val)
//  2. 前 8 字节写两个长度：binary.LittleEndian.PutUint32(data[0:4], ...) 和 data[4:8]
//  3. copy key 到 data[8:]，再 copy val 到 data[8+len(key):]
//     变长数据（key/val）必须把「长度」放前面，读取方才知道该读多少（0102 的核心）
func (ent *Entry) Encode() []byte {
	data := make([]byte, 8+len(ent.key)+len(ent.val))
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(ent.key)))
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(ent.val)))
	copy(data[8:], ent.key)
	copy(data[8+len(ent.key):], ent.val)
	return data
}

// 你来实现（Decode 参数是 io.Reader 接口，不是具体类型——这一步从内存读，下一步从文件读，代码不变）：
//  1. 先 io.ReadFull 读 8 字节 header，解出 klen、vlen 两个长度
//  2. 再 io.ReadFull 读 klen+vlen 字节的数据段
//  3. 按长度切开：ent.key = data[:klen]，ent.val = data[klen:]
//     用 io.ReadFull 而不是 r.Read：Read 允许只返回一部分，ReadFull 会读满或报错
func (ent *Entry) Decode(r io.Reader) error {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	klen := binary.LittleEndian.Uint32(header[0:4])
	vlen := binary.LittleEndian.Uint32(header[4:8])
	data := make([]byte, klen+vlen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	ent.key = data[:klen]
	ent.val = data[klen:]
	return nil
}

// UzBVUkNF https://systems-programming.org/
