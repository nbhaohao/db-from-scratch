package db0203

import "bytes"

type KV struct {
	log Log
	mem map[string][]byte
}

func (kv *KV) Open() error {
	if err := kv.log.Open(); err != nil {
		return err
	}

	kv.mem = map[string][]byte{}
	for {
		ent := Entry{}
		eof, err := kv.log.Read(&ent)
		if err != nil {
			return err
		} else if eof {
			break
		}

		if ent.deleted {
			delete(kv.mem, string(ent.key))
		} else {
			kv.mem[string(ent.key)] = ent.val
		}
	}
	return nil
}

func (kv *KV) Close() error { return kv.log.Close() }

func (kv *KV) Get(key []byte) (val []byte, ok bool, err error) {
	val, ok = kv.mem[string(key)]
	return
}

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

// 你来实现（updated 的语义 = 数据库状态是否真的变了）：
//  1. 先查内存 map：拿到旧值 prev 和是否存在 exist
//  2. 按 mode 判定 updated（switch）：
//     - ModeUpsert：不存在，或存在但值不同（bytes.Equal 比较）
//     - ModeInsert：只在不存在时
//     - ModeUpdate：只在存在且值不同时
//     注意：ModeUpdate 遇到不存在的 key 返回 false + nil error——"没更新到"是
//     正常业务结果，不是错误；同 key 同 value 也是 false（状态没变）
//  3. updated 才动手：先写 log（Entry{key, val}），成功后再改内存 map
//     ——顺序不能反：崩溃时 log 有 mem 无可以恢复，反过来数据就丢了
//  4. updated=false 时什么都不写——省掉无意义的 log 增长
func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error) {
	prev, exist := kv.mem[string(key)]
	switch mode {
	case ModeUpsert:
		updated = !exist || !bytes.Equal(prev, val)
	case ModeInsert:
		updated = !exist
	case ModeUpdate:
		updated = exist && !bytes.Equal(prev, val)
	}
	if updated {
		if err = kv.log.Write(&Entry{key: key, val: val}); err != nil {
			return false, err
		}
		kv.mem[string(key)] = val
	}
	return
}

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

func (kv *KV) Del(key []byte) (deleted bool, err error) {
	_, deleted = kv.mem[string(key)]
	if deleted {
		if err = kv.log.Write(&Entry{key: key, deleted: true}); err != nil {
			return false, err
		}
		delete(kv.mem, string(key))
	}
	return
}

// UzBVUkNF https://systems-programming.org/
