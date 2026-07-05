package db0804

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
	// LSM-Tree
	LogShreshold int
	GrowthFactor float32
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

type KVTX struct {
	target  *KV
	updates SortedArray
	levels  MergedSortedKV
}

func (kv *KV) NewTX() *KVTX {
	tx := &KVTX{target: kv}
	tx.levels = MergedSortedKV{&tx.updates, &kv.mem}
	for i := range kv.main {
		tx.levels = append(tx.levels, &kv.main[i])
	}
	return tx
}

func (tx *KVTX) Abort() {}

func (tx *KVTX) Commit() error { return tx.target.applyTX(tx) }

func (kv *KV) applyTX(tx *KVTX) error {
	if err := kv.updateLog(tx); err != nil {
		return err
	}
	kv.updateMem(tx)
	return nil
}

// 你来实现（Commit 第一步:把事务缓冲 tx.updates 里每条改动写进 log。0804 先做最朴素的逐条写;
// 多个 key 真正的原子提交(共用一条 commit 标记)留到 0805）:
//  1. tx.updates.Iter() 遍历事务里的每条 KV
//  2. 每条 kv.log.Write(&Entry{iter.Key(), iter.Val(), iter.Deleted()});写失败 return err
//  3. 遍历自身出错用 check 断言(内存迭代器不该出错),return nil
func (kv *KV) updateLog(tx *KVTX) error {
	iter, err := tx.updates.Iter()
	check(err == nil)
	for ; iter.Valid(); err = iter.Next() {
		check(err == nil)
		if err := kv.log.Write(&Entry{key: iter.Key(), val: iter.Val(), deleted: iter.Deleted()}); err != nil {
			return err
		}
	}
	return nil
}

// 你来实现（Commit 第二步:把事务缓冲 tx.updates 应用到真正的 kv.mem(和上一步 log 落盘对应)）:
//  1. tx.updates.Iter() 遍历每条改动
//  2. iter.Deleted() → kv.mem.Del(key);否则 kv.mem.Set(key, val);内存操作出错用 check
func (kv *KV) updateMem(tx *KVTX) {
	iter, err := tx.updates.Iter()
	check(err == nil)
	for ; iter.Valid(); err = iter.Next() {
		check(err == nil)
		if iter.Deleted() {
			_, err = kv.mem.Del(iter.Key())
		} else {
			_, err = kv.mem.Set(iter.Key(), iter.Val())
		}
		check(err == nil)
	}
}

func (kv *KV) Open() (err error) {
	if kv.Options.LogShreshold <= 0 {
		kv.Options.LogShreshold = 1000
	}
	if kv.Options.GrowthFactor < 2.0 {
		kv.Options.GrowthFactor = 2.0
	}
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

func (tx *KVTX) Get(key []byte) (val []byte, ok bool, err error) {
	iter, err := tx.Seek(key)
	ok = err == nil && iter.Valid() && bytes.Equal(iter.Key(), key)
	if ok {
		val = iter.Val()
	}
	return val, ok, err
}

func (kv *KV) Get(key []byte) (val []byte, ok bool, err error) {
	tx := kv.NewTX()
	defer tx.Abort()
	return tx.Get(key)
}

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

// 你来实现（事务内的写:和原来 KV.SetEx 几乎一样,但改动只记进 tx.updates(事务私有的 MemTable),
// 不再直接写 log、也不碰 kv.mem——真正落 log 要等 Commit 一并处理,这样多个 key 才能一起原子提交）:
//  1. tx.Get(key) 拿 oldVal/exist(事务内查询,能看到本事务已做的改动)
//  2. 按 mode(Upsert/Insert/Update)算 updated(逻辑同旧 KV.SetEx)
//  3. updated 时:tx.updates.Set(key, val)(只进事务缓冲,不写 log)
//  4. return updated,nil
func (tx *KVTX) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error) {
	oldVal, exist, err := tx.Get(key)
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
	}
	if updated {
		_, err = tx.updates.Set(key, val)
		check(err == nil)
	}
	return updated, nil
}

func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error) {
	tx := kv.NewTX()
	updated, err = tx.SetEx(key, val, mode)
	return abortOrCommit(tx, updated, err)
}

type TXLike interface {
	Abort()
	Commit() error
}

func abortOrCommit(tx TXLike, updated bool, err error) (bool, error) {
	if err != nil {
		tx.Abort()
	} else {
		err = tx.Commit()
	}
	return err == nil && updated, err
}

func (tx *KVTX) Set(key []byte, val []byte) (updated bool, err error) {
	return tx.SetEx(key, val, ModeUpsert)
}

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

// 你来实现（事务内的删除:同样只写进 tx.updates 的墓碑,不碰 log/kv.mem）:
//  1. tx.Get(key):不存在或出错 → return false,err
//  2. tx.updates.Del(key)(在事务缓冲里留墓碑)
//  3. return true,nil
func (tx *KVTX) Del(key []byte) (deleted bool, err error) {
	_, exist, err := tx.Get(key)
	if err != nil || !exist {
		return false, err
	}
	_, err = tx.updates.Del(key)
	check(err == nil)
	return true, nil
}

func (kv *KV) Del(key []byte) (deleted bool, err error) {
	tx := kv.NewTX()
	deleted, err = tx.Del(key)
	return abortOrCommit(tx, deleted, err)
}

// 你来实现（事务内查询。对 LSM-Tree 来说只是多叠一层:NewTX 里已把 tx.levels 拼成
// [tx.updates(事务改动,最新) + kv.mem + 各层 SSTable],这里照常归并 + 过滤墓碑即可）:
//  1. tx.levels.Seek(key) 拿多层归并迭代器;透传 error
//  2. filterDeleted(iter) 跳墓碑后 return
func (tx *KVTX) Seek(key []byte) (SortedKVIter, error) {
	iter, err := tx.levels.Seek(key)
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

func (tx *KVTX) Range(start, stop []byte, desc bool) (*RangedKVIter, error) {
	iter, err := tx.Seek(start)
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

func (kv *KV) Compact() error {
	if kv.mem.Size() >= kv.Options.LogShreshold {
		if err := kv.compactLog(); err != nil {
			return err
		}
	}
	for i := 0; i < len(kv.main)-1; i++ {
		if kv.shouldMerge(i) {
			if err := kv.compactSSTable(i); err != nil {
				return err
			}
			i--
			continue
		}
	}
	return nil
}

func (kv *KV) shouldMerge(idx int) bool {
	cur, next := kv.main[idx].EstimatedSize(), kv.main[idx+1].EstimatedSize()
	return float32(cur)*kv.Options.GrowthFactor >= float32(cur+next)
}

func (kv *KV) compactLog() error {
	kv.version++
	sstable := fmt.Sprintf("sstable_%d", kv.version)
	filename := path.Join(kv.Options.Dirpath, sstable)

	file := SortedFile{FileName: filename}
	m := SortedKV(&kv.mem)
	if len(kv.main) == 0 {
		m = NoDeletedSortedKV{m}
	}
	if err := file.CreateFromSorted(m); err != nil {
		_ = os.Remove(filename)
		return err
	}

	meta := kv.meta.Get()
	meta.Version = kv.version
	meta.SSTables = slices.Insert(meta.SSTables, 0, sstable)
	if err := kv.meta.Set(meta); err != nil {
		_ = file.Close()
		return err
	}

	kv.main = slices.Insert(kv.main, 0, file)
	kv.mem.Clear()
	return kv.log.Truncate()
}

func (kv *KV) compactSSTable(level int) error {
	kv.version++
	sstable := fmt.Sprintf("sstable_%d", kv.version)
	filename := path.Join(kv.Options.Dirpath, sstable)

	file := SortedFile{FileName: filename}
	m := SortedKV(MergedSortedKV{&kv.main[level], &kv.main[level+1]})
	if len(kv.main) == level+2 {
		m = NoDeletedSortedKV{m}
	}
	if err := file.CreateFromSorted(m); err != nil {
		_ = os.Remove(filename)
		return err
	}

	meta := kv.meta.Get()
	meta.Version = kv.version
	meta.SSTables = slices.Replace(meta.SSTables, level, level+2, sstable)
	if err := kv.meta.Set(meta); err != nil {
		_ = file.Close()
		return err
	}

	old1, old2 := kv.main[level].FileName, kv.main[level+1].FileName
	kv.main = slices.Replace(kv.main, level, level+2, file)
	_ = os.Remove(old1)
	_ = os.Remove(old2)
	return nil
}

type NoDeletedSortedKV struct {
	SortedKV
}

func (kv NoDeletedSortedKV) Iter() (iter SortedKVIter, err error) {
	if iter, err = kv.SortedKV.Iter(); err != nil {
		return nil, err
	}
	return NoDeletedIter{iter}, nil
}

// UzBVUkNF https://systems-programming.org/
