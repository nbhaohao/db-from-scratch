package db0602

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
	nkeys    int
}

func (file *SortedFile) Close() error {
	return file.fp.Close()
}

func (file *SortedFile) CreateFromSorted(kv SortedKV) (err error) {
	if file.fp, err = createFileSync(file.FileName); err != nil {
		return err
	}
	if err = file.writeSortedFile(kv); err != nil {
		_ = file.Close()
	}
	return err
}

func (file *SortedFile) writeSortedFile(kv SortedKV) (err error) {
	var buf [4 + 4]byte
	binary.LittleEndian.PutUint64(buf[:8], uint64(kv.Size()))
	if _, err = file.fp.WriteAt(buf[:8], 0); err != nil {
		return err
	}

	nkeys := 0
	offset := 8 + 8*kv.Size()
	iter, err := kv.Iter()
	for ; err == nil && iter.Valid(); err = iter.Next() {
		key, val := iter.Key(), iter.Val()

		binary.LittleEndian.PutUint64(buf[:8], uint64(offset))
		if _, err = file.fp.WriteAt(buf[:8], int64(8+8*nkeys)); err != nil {
			return err
		}

		binary.LittleEndian.PutUint32(buf[0:4], uint32(len(key)))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(len(val)))
		if _, err = file.fp.WriteAt(buf[:4+4], int64(offset)); err != nil {
			return err
		}
		offset += 4 + 4
		if _, err = file.fp.WriteAt(key, int64(offset)); err != nil {
			return err
		}
		offset += len(key)
		if _, err = file.fp.WriteAt(val, int64(offset)); err != nil {
			return err
		}
		offset += len(val)

		nkeys++
	}
	if err != nil {
		return err
	}

	check(nkeys == kv.Size())
	file.nkeys = nkeys
	return file.fp.Sync()
}

type SortedFileIter struct {
	file *SortedFile
	pos  int
	key  []byte
	val  []byte
}

func (iter *SortedFileIter) Valid() bool {
	return 0 <= iter.pos && iter.pos < iter.file.nkeys
}
func (iter *SortedFileIter) Key() []byte { return iter.key }
func (iter *SortedFileIter) Val() []byte { return iter.val }

func (iter *SortedFileIter) Next() error {
	if iter.pos < iter.file.nkeys {
		iter.pos++
	}
	return iter.loadCurrent()
}
func (iter *SortedFileIter) Prev() error {
	if iter.pos >= 0 {
		iter.pos--
	}
	return iter.loadCurrent()
}
func (iter *SortedFileIter) loadCurrent() (err error) {
	if iter.Valid() {
		iter.key, iter.val, err = iter.file.index(iter.pos)
	}
	return err
}

func (file *SortedFile) Size() int { return file.nkeys }
func (file *SortedFile) Iter() (SortedKVIter, error) {
	iter := &SortedFileIter{file: file, pos: 0}
	if err := iter.loadCurrent(); err != nil {
		return nil, err
	}
	return iter, nil
}

// 你来实现（读出 SSTable 里第 pos 对 KV，跟 CreateFromSorted 的写入格式反着读。
// 数据在硬盘上，用 fp.ReadAt(buf, offset) 从指定位置读，返回的 error 要透传上去）:
//  1. check(0 <= pos && pos < file.nkeys) 越界防呆
//  2. 读偏移量数组第 pos 项(位置 8+8*pos，8B):ReadAt 到 buf[:8]（出错 return nil,nil,err），
//     offset := int64(LittleEndian.Uint64(buf[:8]))——第 pos 个 KV 在文件里的起点
//  3. 合法性检查:offset 不该落进头部区(8+8*nkeys)里，否则文件损坏 → return errors.New("corrupted file")
//  4. 在 offset 处读 4+4 字节，拆出 klen、vlen(LittleEndian.Uint32(buf[0:4]) / buf[4:8])
//  5. data := make([]byte, klen+vlen)；在 offset+8 处 ReadAt(data) 一次读完 key+val
//  6. return data[:klen], data[klen:], nil(前段 key、后段 val)
func (file *SortedFile) index(pos int) (key []byte, val []byte, err error)

// 你来实现（二分查找第一个 >= key 的位置，返回定位好的迭代器。注意:index() 会返回 IO error，
// 所以不能用 slices.BinarySearch 这类不透传 error 的封装，要手写二分）:
//  1. 手写二分:lo, hi := 0, file.nkeys；for lo < hi:mid := lo+(hi-lo)/2，
//     k, _, err := file.index(mid)（出错 return nil, err），bytes.Compare(key, k):
//     大于 0 → lo=mid+1；小于 0 → hi=mid；等于 0 → 命中，pos 就是 mid
//     循环自然结束时 pos = lo(第一个 >= key 的位置，可能等于 nkeys)
//  2. iter := &SortedFileIter{file: file, pos: pos}
//  3. iter.loadCurrent() 把该位置的 KV 读进 iter(出错 return nil, err)
//  4. return iter, nil
func (file *SortedFile) Seek(key []byte) (SortedKVIter, error)

// UzBVUkNF https://systems-programming.org/
