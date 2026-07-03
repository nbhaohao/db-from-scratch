package db0405

import (
	"bytes"
	"slices"
)

type KV struct {
	log  Log
	keys [][]byte
	vals [][]byte
}

func (kv *KV) Open() error {
	if err := kv.log.Open(); err != nil {
		return err
	}

	entries := []Entry{}
	for {
		ent := Entry{}
		eof, err := kv.log.Read(&ent)
		if err != nil {
			return err
		} else if eof {
			break
		}
		entries = append(entries, ent)
	}

	slices.SortStableFunc(entries, func(a, b Entry) int {
		return bytes.Compare(a.key, b.key)
	})
	kv.keys, kv.vals = kv.keys[:0], kv.vals[:0]
	for _, ent := range entries {
		n := len(kv.keys)
		if n > 0 && bytes.Equal(kv.keys[n-1], ent.key) {
			kv.keys, kv.vals = kv.keys[:n-1], kv.vals[:n-1]
		}
		if !ent.deleted {
			kv.keys = append(kv.keys, ent.key)
			kv.vals = append(kv.vals, ent.val)
		}
	}
	return nil
}

func (kv *KV) Close() error { return kv.log.Close() }

func (kv *KV) Get(key []byte) (val []byte, ok bool, err error) {
	if idx, ok := slices.BinarySearchFunc(kv.keys, key, bytes.Compare); ok {
		return kv.vals[idx], true, nil
	}
	return nil, false, nil
}

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error) {
	idx, exist := slices.BinarySearchFunc(kv.keys, key, bytes.Compare)
	switch mode {
	case ModeUpsert:
		updated = !exist || !bytes.Equal(kv.vals[idx], val)
	case ModeInsert:
		updated = !exist
	case ModeUpdate:
		updated = exist && !bytes.Equal(kv.vals[idx], val)
	default:
		panic("unreachable")
	}
	if updated {
		if err = kv.log.Write(&Entry{key: key, val: val}); err != nil {
			return false, err
		}
		if exist {
			kv.vals[idx] = val
		} else {
			kv.keys = slices.Insert(kv.keys, idx, key)
			kv.vals = slices.Insert(kv.vals, idx, val)
		}
	}
	return
}

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

func (kv *KV) Del(key []byte) (deleted bool, err error) {
	if idx, ok := slices.BinarySearchFunc(kv.keys, key, bytes.Compare); ok {
		if err = kv.log.Write(&Entry{key: key, deleted: true}); err != nil {
			return false, err
		}
		kv.keys = slices.Delete(kv.keys, idx, idx+1)
		kv.vals = slices.Delete(kv.vals, idx, idx+1)
		return true, nil
	}
	return false, nil
}

type KVIterator struct {
	keys [][]byte
	vals [][]byte
	pos  int
}

func (kv *KV) Seek(key []byte) (*KVIterator, error) {
	pos, _ := slices.BinarySearchFunc(kv.keys, key, bytes.Compare)
	return &KVIterator{keys: kv.keys, vals: kv.vals, pos: pos}, nil
}

func (iter *KVIterator) Valid() bool {
	return 0 <= iter.pos && iter.pos < len(iter.keys)
}

func (iter *KVIterator) Key() []byte {
	check(iter.Valid())
	return iter.keys[iter.pos]
}

func (iter *KVIterator) Val() []byte {
	check(iter.Valid())
	return iter.vals[iter.pos]
}

func (iter *KVIterator) Next() error {
	if iter.pos < len(iter.keys) {
		iter.pos++
	}
	return nil
}

func (iter *KVIterator) Prev() error {
	if iter.pos >= 0 {
		iter.pos--
	}
	return nil
}

type RangedKVIter struct {
	iter KVIterator
	stop []byte
	desc bool
}

func (iter *RangedKVIter) Valid() bool {
	if !iter.iter.Valid() {
		return false
	}
	r := bytes.Compare(iter.iter.Key(), iter.stop)
	if iter.desc && r < 0 {
		return false
	} else if !iter.desc && r > 0 {
		return false
	}
	return true
}

func (iter *RangedKVIter) Key() []byte {
	check(iter.Valid())
	return iter.iter.Key()
}

func (iter *RangedKVIter) Val() []byte {
	check(iter.Valid())
	return iter.iter.Val()
}

func (iter *RangedKVIter) Next() error {
	if !iter.Valid() {
		return nil
	}
	if iter.desc {
		return iter.iter.Prev()
	} else {
		return iter.iter.Next()
	}
}

// 你来实现（定位到范围扫描的起点，desc 决定是升序还是降序；start/stop 已经是编码好的边界字节串，
// 这层不用管业务语义，只管"从 start 开始，往 desc 方向走，用 stop 判断该不该停"）：
//  1. iter, err := kv.Seek(start)（复用二分查找，定位到第一个 >= start 的位置），出错 return nil, err
//  2. 降序扫描时 Seek 给的是">= start 的第一个位置"，但降序应该从 "<= start 的最后一个位置" 开始，
//     所以：desc 为 true 且 (!iter.Valid() 或 iter.Key() > start)，说明 Seek 给多走了一步，
//     调 iter.Prev() 退回一步；出错 return nil, err
//  3. return &RangedKVIter{iter: *iter, stop: stop, desc: desc}, nil
func (kv *KV) Range(start, stop []byte, desc bool) (*RangedKVIter, error)

// UzBVUkNF https://systems-programming.org/
