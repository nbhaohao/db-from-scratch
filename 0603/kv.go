package db0603

import (
	"bytes"
	"slices"
)

type KV struct {
	log Log
	mem SortedArray
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
	kv.mem.Clear()
	for _, ent := range entries {
		n := kv.mem.Size()
		if n > 0 && bytes.Equal(kv.mem.Key(n-1), ent.key) {
			kv.mem.Pop()
		}
		if !ent.deleted {
			kv.mem.Push(ent.key, ent.val)
		}
	}
	return nil
}

func (kv *KV) Close() error { return kv.log.Close() }

func (kv *KV) Get(key []byte) (val []byte, ok bool, err error) {
	return kv.mem.Get(key)
}

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

// 你来实现（写入一对 KV:先按 mode 决定该不该写，再落 log(WAL)，再更新内存。逻辑跟前几章的
// SetEx 一样，只是内存结构换成了 kv.mem(SortedArray)——写内存改调 kv.mem.Set）:
//  1. oldVal, exist, err := kv.Get(key)(出错 return false, err)
//  2. 按 mode 定 updated:ModeUpsert = !exist || !bytes.Equal(oldVal,val)；ModeInsert = !exist；
//     ModeUpdate = exist && !bytes.Equal(oldVal,val)；default panic("unreachable")
//  3. 若 updated:先 kv.log.Write(&Entry{key:key, val:val})(先写日志再改内存，出错 return false,err)；
//     再 _, err = kv.mem.Set(key, val)；check(err == nil)
//  4. return updated, nil
func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error)

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

// 你来实现（删除一对 KV:先探是否存在，再从内存删，再落一条"删除"日志）:
//  1. _, exist, err := kv.Get(key)；err != nil || !exist → return false, err(不存在直接返回)
//  2. _, err = kv.mem.Del(key)；check(err == nil)
//  3. kv.log.Write(&Entry{key:key, deleted:true})——deleted 标记，重放 log 时据此抹掉该 key(出错 return false,err)
//  4. return true, nil
func (kv *KV) Del(key []byte) (deleted bool, err error)

func (kv *KV) Seek(key []byte) (SortedKVIter, error) {
	return kv.mem.Seek(key)
}

type RangedKVIter struct {
	iter SortedKVIter
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

func (kv *KV) Range(start, stop []byte, desc bool) (*RangedKVIter, error) {
	iter, err := kv.Seek(start)
	if err != nil {
		return nil, err
	}
	if desc && (!iter.Valid() || bytes.Compare(iter.Key(), start) > 0) {
		if err = iter.Prev(); err != nil {
			return nil, err
		}
	}
	return &RangedKVIter{iter: iter, stop: stop, desc: desc}, nil
}

// UzBVUkNF https://systems-programming.org/
