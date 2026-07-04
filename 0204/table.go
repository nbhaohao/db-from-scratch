package db0204

type DB struct {
	KV KV
}

func (db *DB) Open() error  { return db.KV.Open() }
func (db *DB) Close() error { return db.KV.Close() }

// 你来实现（入参 row 只需填好主键列；查到后把非主键列原地补全）：
//  1. row.EncodeKey 得到 K → KV.Get 查询
//  2. 出错或没查到：原样返回（!ok 不是错误）
//  3. 查到了：用 row.DecodeVal 把 V 解码回 row 的非主键列（就地修改）
func (db *DB) Select(schema *Schema, row Row) (ok bool, err error) {
	val, ok, err := db.KV.Get(row.EncodeKey(schema))
	if err != nil || !ok {
		return false, err
	}
	return true, row.DecodeVal(schema, val)
}

// 你来实现（下面三个写操作套路一样：EncodeKey + EncodeVal → SetEx，只差 mode）：
//  1. insert 语句语义 = 只新增，key 已存在则不动 → 用哪个 UpdateMode？
func (db *DB) Insert(schema *Schema, row Row) (updated bool, err error) {
	return db.KV.SetEx(row.EncodeKey(schema), row.EncodeVal(schema), ModeInsert)
}

// 2. upsert = 有则覆盖、无则新增（KV 的 set 天然就是这个语义）
func (db *DB) Upsert(schema *Schema, row Row) (updated bool, err error) {
	return db.KV.SetEx(row.EncodeKey(schema), row.EncodeVal(schema), ModeUpsert)
}

// 3. update 语句语义 = 只更新已有的 key
func (db *DB) Update(schema *Schema, row Row) (updated bool, err error) {
	return db.KV.SetEx(row.EncodeKey(schema), row.EncodeVal(schema), ModeUpdate)
}

// 你来实现（删除定位只靠 K）：
//  1. row.EncodeKey → KV.Del；不需要 EncodeVal——V 是跟着 K 一起删的
func (db *DB) Delete(schema *Schema, row Row) (deleted bool, err error) {
	return db.KV.Del(row.EncodeKey(schema))
}

// UzBVUkNF https://systems-programming.org/
