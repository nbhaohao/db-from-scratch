package db0605

import (
	"bytes"
	"slices"
)

type SortedArray struct {
	keys    [][]byte
	vals    [][]byte
	deleted []bool
}

func (arr *SortedArray) Size() int { return len(arr.keys) }

func (arr *SortedArray) EstimatedSize() int {
	return len(arr.keys)
}

func (arr *SortedArray) Iter() (SortedKVIter, error) {
	return &SortedArrayIter{arr.keys, arr.vals, arr.deleted, 0}, nil
}

func (arr *SortedArray) Seek(key []byte) (SortedKVIter, error) {
	pos, _ := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
	return &SortedArrayIter{keys: arr.keys, vals: arr.vals, deleted: arr.deleted, pos: pos}, nil
}

type SortedArrayIter struct {
	keys    [][]byte
	vals    [][]byte
	deleted []bool
	pos     int
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

func (iter *SortedArrayIter) Deleted() bool {
	check(iter.Valid())
	return iter.deleted[iter.pos]
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
	arr.keys, arr.vals, arr.deleted = arr.keys[:0], arr.vals[:0], arr.deleted[:0]
}

func (arr *SortedArray) Push(key []byte, val []byte, deleted bool) {
	arr.keys = append(arr.keys, key)
	arr.vals = append(arr.vals, val)
	arr.deleted = append(arr.deleted, deleted)
}

func (arr *SortedArray) Pop() {
	n := arr.Size()
	arr.keys, arr.vals, arr.deleted = arr.keys[:n-1], arr.vals[:n-1], arr.deleted[:n-1]
}

func (arr *SortedArray) Key(i int) []byte {
	return arr.keys[i]
}

// 你来实现（插入/更新一对 KV。0605 给 SortedArray 加了 deleted 墓碑标记，Set 要把该位置的墓碑清掉）:
//  1. idx, ok := slices.BinarySearchFunc(arr.keys, key, bytes.Compare)
//  2. updated = !ok || arr.deleted[idx] || !bytes.Equal(val, arr.vals[idx])
//     (不存在、或原来是墓碑、或值变了都算更新——"复活一个被删的 key"也是更新)
//  3. 若 updated:ok(已存在)→ arr.vals[idx]=val 且 arr.deleted[idx]=false(清墓碑)；
//     否则用 slices.Insert 在 idx 处把 key、val 各插进去，同时给 arr.deleted 插入 false
//  4. return updated, nil
func (arr *SortedArray) Set(key []byte, val []byte) (updated bool, err error)

// 你来实现（删除一对 KV。两层结构下上层不能物理删——否则会把下层 SSTable 里的旧值查出来，
// 所以要留一个 deleted=true 的墓碑盖住下层）:
//  1. idx, ok := slices.BinarySearchFunc(...)；exist := ok && !arr.deleted[idx](真存在 = 有 key 且不是墓碑)
//  2. 若 exist:arr.vals[idx]=nil、arr.deleted[idx]=true(就地变墓碑)，return true, nil
//  3. 否则:在 idx 处 slices.Insert 一个 val=nil、deleted=true 的墓碑，return false, nil
//     (即使本层没有这个 key 也要插墓碑，因为它可能藏在下层 SSTable 里)
func (arr *SortedArray) Del(key []byte) (deleted bool, err error)

// UzBVUkNF https://systems-programming.org/
