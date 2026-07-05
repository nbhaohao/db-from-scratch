package db0703

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
)

type KVOptions struct {
	Dirpath string
}

type KV struct {
	Options KVOptions
	// metadata
	meta    KVMetaStore
	version uint64
	// data
	log  Log
	mem  SortedArray
	main []SortedFile
	MultiClosers
}

func (kv *KV) Open() (err error) {
	if err = kv.openAll(); err != nil {
		_ = kv.Close()
	}
	return err
}

func (kv *KV) openAll() error {
	err := os.Mkdir(kv.Options.Dirpath, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}

	if err := kv.openMeta(); err != nil {
		return err
	}
	if err := kv.openLog(); err != nil {
		return err
	}
	return kv.openSSTable()
}

func (kv *KV) openMeta() error {
	kv.meta.slots[0].FileName = path.Join(kv.Options.Dirpath, "meta0")
	kv.meta.slots[1].FileName = path.Join(kv.Options.Dirpath, "meta1")
	if err := kv.meta.Open(); err != nil {
		return err
	}
	kv.MultiClosers = append(kv.MultiClosers, &kv.meta)
	return nil
}

func (kv *KV) openLog() error {
	kv.log.FileName = path.Join(kv.Options.Dirpath, "kv_log")
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
	meta := kv.meta.Get()
	kv.version = meta.Version
	kv.main = kv.main[:0]
	for _, sstable := range meta.SSTables {
		sstable = path.Join(kv.Options.Dirpath, sstable)
		file := SortedFile{FileName: sstable}
		if err := file.Open(); err != nil {
			return err
		}
		kv.MultiClosers = append(kv.MultiClosers, &file)
		kv.main = append(kv.main, file)
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

// 你来实现（多层查询：main 从单个 SSTable 变成了 []SortedFile。查询要把 MemTable + 所有层
// 一起做多路归并，上层(下标小)优先；删除的 key(墓碑)在归并后要过滤掉，别露出下层的旧值）:
//  1. 建 MergedSortedKV，第一个放 &kv.mem（最上层，优先级最高）
//  2. for i := range kv.main 依次 append(&kv.main[i])（下标越小越新，紧跟在 mem 之后）
//  3. 对这个多层集合调 .Seek(key) 拿归并迭代器；透传 error
//  4. 用已就位的 filterDeleted(iter) 跳过墓碑后 return
func (kv *KV) Seek(key []byte) (SortedKVIter, error) {
	levels := MergedSortedKV{&kv.mem}
	for i := range kv.main {
		levels = append(levels, &kv.main[i])
	}
	iter, err := levels.Seek(key)
	if err != nil {
		return nil, err
	}
	return filterDeleted(iter)
}

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

// 你来实现（把 log/MemTable 转成一个新的顶层 SSTable，插到 main[0]。这次只落 MemTable 本身，
// 不再和下层合并——层与层的合并留到 0704。metadata 也从单个 SSTable 名换成了 SSTables 列表）:
//  1. kv.version++；同前拼 sstable_%d 全路径文件名
//  2. 只把 &kv.mem 用 CreateFromSorted 落成新文件（失败 os.Remove 半成品 + return）
//  3. meta.Get()，meta.Version=version，用 slices.Insert 把新文件名插到 meta.SSTables 第 0 位；meta.Set，失败关文件 return
//  4. 用 slices.Insert 把新 file 插到 kv.main 第 0 位（成为最上层 SSTable）
//  5. kv.mem.Clear()，return kv.log.Truncate()
func (kv *KV) Compact() error {
	kv.version++
	filename := fmt.Sprintf("sstable_%d", kv.version)
	filepath := path.Join(kv.Options.Dirpath, filename)

	newFile := SortedFile{FileName: filepath}
	if err := newFile.CreateFromSorted(&kv.mem); err != nil {
		os.Remove(filepath)
		return err
	}

	meta := kv.meta.Get()
	meta.Version = kv.version
	meta.SSTables = slices.Insert(meta.SSTables, 0, filename)
	if err := kv.meta.Set(meta); err != nil {
		newFile.Close()
		return err
	}

	kv.main = slices.Insert(kv.main, 0, newFile)

	kv.mem.Clear()
	return kv.log.Truncate()
}

// UzBVUkNF https://systems-programming.org/
