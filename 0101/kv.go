package db0101

type KV struct {
	mem map[string][]byte
}

func (kv *KV) Open() error {
	kv.mem = map[string][]byte{} // empty
	return nil
}

func (kv *KV) Close() error { return nil }

// 你来实现（纯内存 map，还没碰硬盘）：
//  1. 从 kv.mem 里取 key（map 的 key 用 string(key)，因为 []byte 不能做 map key）
//  2. map 读取的双返回值 val, ok 正好对上签名——直接 naked return
func (kv *KV) Get(key []byte) (val []byte, ok bool, err error)

// 你来实现（updated = 数据库状态是否真的变了，这个语义要一路带到后面的 log 关）：
//  1. 先读旧值 prev 和是否存在 exist
//  2. 写入 map：kv.mem[string(key)] = val
//  3. updated = 原本不存在，或存在但值变了（bytes.Equal 比较 prev 和 val）
func (kv *KV) Set(key []byte, val []byte) (updated bool, err error)

// 你来实现（deleted = 这次删除有没有真的删掉东西）：
//  1. 先看 key 在不在（map 取值的第二返回值 = deleted）
//  2. 再 delete(kv.mem, string(key))——删不存在的 key 是空操作，不会报错
func (kv *KV) Del(key []byte) (deleted bool, err error)

// UzBVUkNF https://systems-programming.org/
