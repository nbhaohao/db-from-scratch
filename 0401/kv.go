package db0401

type KV struct {
	log  Log
	keys [][]byte
	vals [][]byte
}

// 你来实现（把 log 里的全部 entry 重放成一个按 key 排好序的内存数组，去重只留最后写入的版本）：
//  1. kv.log.Open()，出错直接 return
//  2. for 循环 kv.log.Read(&ent) 读出全部 entry 存进一个局部 entries 切片，遇到 eof 跳出，出错直接 return
//  3. slices.SortStableFunc(entries, ...) 按 key 用 bytes.Compare 稳定排序（stable 保证同 key 时后写的排后面）
//  4. kv.keys, kv.vals 先清空（[:0] 复用底层数组）；遍历排好序的 entries：
//     若上一个已放入的 key 和当前相同，先把上一个弹出（同 key 只保留最后一次的版本）；
//     若当前 entry 不是删除标记（!ent.deleted），append 进 kv.keys/kv.vals
//  5. return nil
func (kv *KV) Open() error

func (kv *KV) Close() error { return kv.log.Close() }

// 你来实现（在有序数组里二分查找 key）：
//  1. slices.BinarySearchFunc(kv.keys, key, bytes.Compare) 返回 idx, ok
//  2. ok 为 true：return kv.vals[idx], true, nil
//  3. 否则：return nil, false, nil
func (kv *KV) Get(key []byte) (val []byte, ok bool, err error)

type UpdateMode int

const (
	ModeUpsert UpdateMode = 0 // insert or update
	ModeInsert UpdateMode = 1 // insert new
	ModeUpdate UpdateMode = 2 // update existing
)

// 你来实现（按 mode 决定要不要写，写完把内存有序数组也同步更新）：
//  1. idx, exist := slices.BinarySearchFunc(kv.keys, key, bytes.Compare) 先查一次
//  2. 按 mode 算出 updated：ModeUpsert 是"不存在或值变了"；ModeInsert 是"不存在"；ModeUpdate 是"存在且值变了"；
//     其他 mode panic("unreachable")
//  3. updated 为 true 才真正写：kv.log.Write(&Entry{key: key, val: val})，出错 return false, err
//  4. exist 为 true：直接 kv.vals[idx] = val 覆盖；exist 为 false：
//     slices.Insert(kv.keys, idx, key) 和 slices.Insert(kv.vals, idx, val) 把新 key/val 插到排序应在的位置
//  5. return（命名返回值 updated, err 已经在前面赋值过了）
func (kv *KV) SetEx(key []byte, val []byte, mode UpdateMode) (updated bool, err error)

func (kv *KV) Set(key []byte, val []byte) (updated bool, err error) {
	return kv.SetEx(key, val, ModeUpsert)
}

// 你来实现（二分查到 key 才删：写一条 deleted 标记的 entry 落盘，再把内存数组里对应位置摘掉）：
//  1. idx, ok := slices.BinarySearchFunc(kv.keys, key, bytes.Compare)；ok 为 false 直接 return false, nil
//  2. kv.log.Write(&Entry{key: key, deleted: true})，出错 return false, err
//  3. slices.Delete(kv.keys, idx, idx+1) 和 slices.Delete(kv.vals, idx, idx+1) 摘掉这一项
//  4. return true, nil
func (kv *KV) Del(key []byte) (deleted bool, err error)

// UzBVUkNF https://systems-programming.org/
