package db0605

import (
	"bytes"
	"slices"
)

type KV struct {
	log  Log
	mem  SortedArray
	main SortedFile
	MultiClosers
}

func (kv *KV) Open() (err error) {
	if err = kv.openAll(); err != nil {
		_ = kv.Close()
	}
	return err
}

func (kv *KV) openAll() error {
	if err := kv.openLog(); err != nil {
		return err
	}
	return kv.openSSTable()
}

func (kv *KV) openLog() error {
	if err := kv.log.Open(); err != nil {
		return err
	}
	kv.MultiClosers = append(kv.MultiClosers, &kv.log)

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
		kv.mem.Push(ent.key, ent.val, ent.deleted)
	}
	return nil
}

func (kv *KV) openSSTable() error {
	if kv.main.FileName != "" {
		if err := kv.main.Open(); err != nil {
			return err
		}
		kv.MultiClosers = append(kv.MultiClosers, &kv.main)
	}
	return nil
}

func (kv *KV) Get(key []byte) (val []byte, ok bool, err error) {
	iter, err := kv.Seek(key)
	ok = err == nil && iter.Valid() && bytes.Equal(iter.Key(), key)
	if ok {
		val = iter.Val()
	}
	return val, ok, err
}

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error) {
	oldVal, exist, err := kv.Get(key)
	if err != nil {
		return false, err
	}

	switch mode {
	case ModeUpsert:
		updated = !exist || !bytes.Equal(oldVal, val)
	case ModeInsert:
		updated = !exist
	case ModeUpdate:
		updated = exist && !bytes.Equal(oldVal, val)
	default:
		panic("unreachable")
	}
	if updated {
		if err = kv.log.Write(&Entry{key: key, val: val}); err != nil {
			return false, err
		}
		_, err = kv.mem.Set(key, val)
		check(err == nil)
	}
	return updated, nil
}

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

func (kv *KV) Del(key []byte) (deleted bool, err error) {
	if _, exist, err := kv.Get(key); err != nil || !exist {
		return false, err
	}
	_, err = kv.mem.Del(key)
	check(err == nil)
	if err = kv.log.Write(&Entry{key: key, deleted: true}); err != nil {
		return false, err
	}
	return true, nil
}

// 你来实现（合并 MemTable 和 SSTable 做查询:两层都要看，归并结果，再滤掉墓碑）:
//  1. m := MergedSortedKV{&kv.mem, &kv.main}(mem 在前 = 优先级高)
//  2. iter, err := m.Seek(key)(0605 给 MergedSortedKV 新增的 Seek，k 路归并定位；出错 return nil, err)
//  3. return filterDeleted(iter)——跳过开头的墓碑并包一层 NoDeletedIter(gen 已提供)
func (kv *KV) Seek(key []byte) (SortedKVIter, error)

func filterDeleted(iter SortedKVIter) (SortedKVIter, error) {
	for iter.Valid() && iter.Deleted() {
		if err := iter.Next(); err != nil {
			return nil, err
		}
	}
	return NoDeletedIter{iter}, nil
}

type NoDeletedIter struct {
	SortedKVIter
}

func (iter NoDeletedIter) Next() (err error) {
	err = iter.SortedKVIter.Next()
	for err == nil && iter.Valid() && iter.Deleted() {
		err = iter.SortedKVIter.Next()
	}
	return err
}

func (iter NoDeletedIter) Prev() (err error) {
	err = iter.SortedKVIter.Prev()
	for err == nil && iter.Valid() && iter.Deleted() {
		err = iter.SortedKVIter.Prev()
	}
	return err
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

// 你来实现（把 MemTable(log) 合并进 SSTable、产出新 SSTable、再清空 log。这就是 copy-on-write:
// 先写全新文件，再用 rename 原子替换旧文件，最后清空 log。分 3 步）:
//  1. 合并到临时文件:在 kv.main 同目录建临时文件(os.CreateTemp(path.Dir(...)))，file := SortedFile{FileName: tmp}，
//     m := MergedSortedKV{&kv.mem, &kv.main}，file.CreateFromSorted(m) 把两层合并写出(出错 return err)。
//     记得 defer os.Remove(tmp) 清掉失败残留
//  2. 原子替换:关掉旧 kv.main、关掉刚写好的 file，renameSync(file.FileName, kv.main.FileName)(rename + 对目录 fsync)。
//     替换失败时尽量把 kv.main 重新 Open 回来再 return err；成功后 kv.main.Open() 重新打开这个新文件
//  3. 清空:kv.mem.Clear() + return kv.log.Truncate()(数据已进 SSTable，log 可以丢了)
func (kv *KV) Compact() error

// UzBVUkNF https://systems-programming.org/
