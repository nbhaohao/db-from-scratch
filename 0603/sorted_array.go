package db0603

import (
	"bytes"
	"slices"
)

type SortedArray struct {
	keys [][]byte
	vals [][]byte
}

func (arr *SortedArray) Size() int {
	return len(arr.keys)
}

func (arr *SortedArray) Iter() (SortedKVIter, error) {
	return &SortedArrayIter{arr.keys, arr.vals, 0}, nil
}

func (arr *SortedArray) Seek(key []byte) (SortedKVIter, error) {
	pos, _ := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
	return &SortedArrayIter{keys: arr.keys, vals: arr.vals, pos: pos}, nil
}

type SortedArrayIter struct {
	keys [][]byte
	vals [][]byte
	pos  int
}

func (iter *SortedArrayIter) Valid() bool {
	return 0 <= iter.pos && iter.pos < len(iter.keys)
}

func (iter *SortedArrayIter) Key() []byte {
	check(iter.Valid())
	return iter.keys[iter.pos]
}

func (iter *SortedArrayIter) Val() []byte {
	check(iter.Valid())
	return iter.vals[iter.pos]
}

func (iter *SortedArrayIter) Next() error {
	if iter.pos < len(iter.keys) {
		iter.pos++
	}
	return nil
}

func (iter *SortedArrayIter) Prev() error {
	if iter.pos >= 0 {
		iter.pos--
	}
	return nil
}

func (arr *SortedArray) Clear() {
	arr.keys, arr.vals = arr.keys[:0], arr.vals[:0]
}

func (arr *SortedArray) Push(key []byte, val []byte) {
	arr.keys = append(arr.keys, key)
	arr.vals = append(arr.vals, val)
}

func (arr *SortedArray) Pop() {
	n := arr.Size()
	arr.keys, arr.vals = arr.keys[:n-1], arr.vals[:n-1]
}

func (arr *SortedArray) Key(i int) []byte {
	return arr.keys[i]
}

func (arr *SortedArray) Get(key []byte) (val []byte, ok bool, err error) {
	if idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare); ok {
		return arr.vals[idx], true, nil
	}
	return nil, false, nil
}

// 你来实现（在排序数组里插入或更新一对 KV，保持 keys 有序。这段就是从原来 KV 里搬过来的逻辑）:
//  1. idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare) 找位置
//  2. updated = !ok || !bytes.Equal(val, arr.vals[idx])(不存在、或值真的变了才算更新)
//  3. 若 updated:ok(已存在)→ arr.vals[idx] = val 原地改；否则用 slices.Insert 在 idx 处把 key、val 各插进去
//  4. return updated, nil
func (arr *SortedArray) Set(key []byte, val []byte) (updated bool, err error) {
	idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
	updated = !ok || !bytes.Equal(val, arr.vals[idx])
	if updated {
		if ok {
			arr.vals[idx] = val
		} else {
			arr.keys = slices.Insert(arr.keys, idx, key)
			arr.vals = slices.Insert(arr.vals, idx, val)
		}
	}
	return updated, nil
}

// 你来实现（从排序数组里删除 key。0603 只有内存一层，还没 SSTable，直接物理删除即可）:
//  1. idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
//  2. 存在(ok)→ slices.Delete 从 arr.keys、arr.vals 各删掉 [idx, idx+1)，return true, nil
//  3. 不存在 → return false, nil
func (arr *SortedArray) Del(key []byte) (deleted bool, err error) {
	idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
	if ok {
		arr.keys = slices.Delete(arr.keys, idx, idx+1)
		arr.vals = slices.Delete(arr.vals, idx, idx+1)
		return true, nil
	}
	return false, nil
}

// UzBVUkNF https://systems-programming.org/
