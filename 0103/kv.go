package db0103

import (
	"bytes"
	"io"
)

type KV struct {
	log Log
	mem map[string][]byte
}

// 你来实现（启动时重放 log 重建内存状态——log 是唯一真相源，mem 是它的缓存视图）：
//  1. 先 kv.log.Open()，出错直接返回
//  2. kv.mem = 空 map
//  3. 循环 kv.log.Read(&ent) 直到 eof：
//     - deleted 的条目 → delete(mem, key)
//     - 普通条目 → mem[key] = val
//     后面的条目自然覆盖前面的（同 key 再 set 就是重新赋值），得到最终状态
func (kv *KV) Open() error {
	if err := kv.log.Open(); err != nil {
		return err
	}
	kv.mem = make(map[string][]byte)
	for {
		var ent Entry
		if err := ent.Decode(kv.log.fp); err != nil {
			if err == io.EOF {
				break
			}
			return err
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

// 你来实现（0101 的内存版 Set 之上，加一步持久化）：
//  1. 先算 updated（同 0101：不存在或值变了）
//  2. 只有 updated 才动手——先写 log：kv.log.Write(Entry{key, val})，出错返回
//  3. 写 log 成功后再改内存 map
//     顺序不能反：先 log 后内存，崩溃时靠 log 能重放恢复；反过来内存有 log 无就丢数据
func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	prev, exist := kv.mem[string(key)]
	updated = !exist || !bytes.Equal(prev, val)
	if updated {
		if err := kv.log.Write(&Entry{key: key, val: val}); err != nil {
			return false, err
		}
	}
	kv.mem[string(key)] = val
	return updated, nil
}

// 你来实现（删除也要落一条 log，否则重启重放时这个 key 又回来了）：
//  1. 先看 key 在不在（= deleted）
//  2. 存在才处理：写一条 Entry{key, deleted: true} 到 log，出错返回；再 delete 内存
func (kv *KV) Del(key []byte) (deleted bool, err error) {
	deleted = true
	if _, ok := kv.mem[string(key)]; !ok {
		deleted = false
	}
	if deleted {
		if err := kv.log.Write(&Entry{key: key, deleted: true}); err != nil {
			return false, err
		}
		delete(kv.mem, string(key))
	}
	return deleted, nil
}

// UzBVUkNF https://systems-programming.org/
