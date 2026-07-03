package db0105

import (
	"errors"
	"io"
)

type Entry struct {
	key     []byte
	val     []byte
	deleted bool
}

// 你来实现（新格式：| crc32 4B | key 大小 4B | val 大小 4B | 是否删除 1B | key | val |）：
//  1. 分配 4+4+4+1 + len(key) + len(val)（deleted 时 val 为空、vlen 记 0）
//  2. 填 key 大小(data[4:8])、copy key；deleted 则置删除标志字节为 1，否则填 val 大小 + copy val
//  3. 最后算校验码：crc32.ChecksumIEEE(data[4:])（覆盖 crc 之后的全部字节），写进 data[0:4]
//     crc 放最前面、算的是它后面的内容——解码时先读到 crc 再逐字节校验
func (ent *Entry) Encode() []byte

var ErrBadSum = errors.New("bad checksum")

// 你来实现（能识别「上次没写完」的半条记录并报错）：
//  1. io.ReadFull 读 13 字节 header（4+4+4+1），解出 klen、vlen、deleted
//  2. io.ReadFull 读 klen+vlen 数据段
//  3. 重新算 crc32：喂 header[4:] 再喂 data，和 header[0:4] 里存的比——不等就 return ErrBadSum
//  4. 校验过了再切出 key（deleted 时不取 val）
//     返回错误分三种：正常读到尾 io.EOF；读一半没了 io.ErrUnexpectedEOF；校验失败 ErrBadSum
func (ent *Entry) Decode(r io.Reader) error

// UzBVUkNF https://systems-programming.org/
