package db0601

import (
	"encoding/binary"
	"os"
)

type SortedKV interface {
	Size() int
	Iter() (SortedKVIter, error)
}

type SortedKVIter interface {
	Valid() bool
	Key() []byte
	Val() []byte
	Next() error
	Prev() error
}

type SortedFile struct {
	FileName string
	fp       *os.File
}

func (file *SortedFile) Close() error {
	return file.fp.Close()
}

// 你来实现（把内存里的排序数组序列化成 SSTable 文件。文件格式:
//
//	 [ nkeys(8B) | 偏移量1(8B) | ... | 偏移量n(8B) | KV1 | KV2 | ... | KVn ]
//	 每个 KV = [ keylen(4B) | vallen(4B) | key 数据 | val 数据 ]；偏移量数组第 i 项指向第 i 个 KV 的起点。
//	 写到指定位置用 fp.WriteAt(data, offset)，不用管文件的"当前位置"）:
//	1. createFileSync(file.FileName) 建文件(内部带 fsync)，把 *os.File 存进 file.fp，出错 return err
//	2. buf 备一个 [4+4]byte 即可(8B 也够写 uint64)。因为 kv.Size() 已知总数，先把 nkeys 写到文件开头:
//	   PutUint64(buf[:8], kv.Size()) + WriteAt(buf[:8], 0)
//	3. 算首个 KV 的偏移量 offset = 8 + 8*kv.Size()(跳过头部的 nkeys + n 个 8B 偏移量)
//	4. iter := kv.Iter()；for ; err==nil && iter.Valid(); err = iter.Next() 遍历，另用 nkeys 计数:
//	   a. 把当前 offset 写进偏移量数组第 nkeys 项(位置 8+8*nkeys):PutUint64(buf[:8], offset)+WriteAt
//	   b. 在 offset 处写 keylen、vallen(PutUint32 到 buf[0:4]、buf[4:8]，一次 WriteAt(buf[:8]))，offset += 8
//	   c. 在 offset 处 WriteAt(key)，offset += len(key)；再 WriteAt(val)，offset += len(val)
//	   d. nkeys++
//	5. 循环里每个 WriteAt / iter.Next 的 err 都要检查并 return err
//	6. check(nkeys == kv.Size()) 断言写出的数量和声称的一致，最后 return file.fp.Sync() 落盘
func (file *SortedFile) CreateFromSorted(kv SortedKV) (err error) {
	file.fp, err = createFileSync(file.FileName)
	if err != nil {
		return err
	}

	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(kv.Size()))
	if _, err = file.fp.WriteAt(buf[:8], 0); err != nil {
		return err
	}

	offset := uint64(8 + 8*kv.Size())
	iter, err := kv.Iter()
	if err != nil {
		return err
	}

	nkeys := 0
	for ; iter.Valid(); err = iter.Next() {
		if err != nil {
			return err
		}

		key := iter.Key()
		val := iter.Val()

		binary.LittleEndian.PutUint64(buf[:8], offset)
		if _, err = file.fp.WriteAt(buf[:8], int64(8+8*nkeys)); err != nil {
			return err
		}

		binary.LittleEndian.PutUint32(buf[0:4], uint32(len(key)))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(len(val)))
		if _, err = file.fp.WriteAt(buf[:8], int64(offset)); err != nil {
			return err
		}
		offset += 8

		if _, err = file.fp.WriteAt(key, int64(offset)); err != nil {
			return err
		}
		offset += uint64(len(key))

		if _, err = file.fp.WriteAt(val, int64(offset)); err != nil {
			return err
		}
		offset += uint64(len(val))

		nkeys++
	}
	if err != nil {
		return err
	}

	check(nkeys == kv.Size())
	return file.fp.Sync()
}

// UzBVUkNF https://systems-programming.org/
