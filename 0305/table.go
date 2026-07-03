package db0305

import (
	"encoding/json"
	"errors"
)

type DB struct {
	KV     KV
	tables map[string]Schema
}

func (db *DB) Open() error {
	db.tables = map[string]Schema{}
	return db.KV.Open()
}

func (db *DB) Close() error { return db.KV.Close() }

func (db *DB) Select(schema *Schema, row Row) (ok bool, err error) {
	key := row.EncodeKey(schema)
	val, ok, err := db.KV.Get(key)
	if err != nil || !ok {
		return ok, err
	}
	if err = row.DecodeVal(schema, val); err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) Insert(schema *Schema, row Row) (updated bool, err error) {
	key := row.EncodeKey(schema)
	val := row.EncodeVal(schema)
	return db.KV.SetEx(key, val, ModeInsert)
}

func (db *DB) Upsert(schema *Schema, row Row) (updated bool, err error) {
	key := row.EncodeKey(schema)
	val := row.EncodeVal(schema)
	return db.KV.SetEx(key, val, ModeUpsert)
}

func (db *DB) Update(schema *Schema, row Row) (updated bool, err error) {
	key := row.EncodeKey(schema)
	val := row.EncodeVal(schema)
	return db.KV.SetEx(key, val, ModeUpdate)
}

func (db *DB) Delete(schema *Schema, row Row) (deleted bool, err error) {
	key := row.EncodeKey(schema)
	return db.KV.Del(key)
}

type SQLResult struct {
	Updated int
	Header  []string
	Values  []Row
}

func (db *DB) ExecStmt(stmt interface{}) (r SQLResult, err error) {
	switch ptr := stmt.(type) {
	case *StmtCreatTable:
		err = db.execCreateTable(ptr)
	case *StmtSelect:
		r.Header = ptr.cols
		r.Values, err = db.execSelect(ptr)
	case *StmtInsert:
		r.Updated, err = db.execInsert(ptr)
	case *StmtUpdate:
		r.Updated, err = db.execUpdate(ptr)
	case *StmtDelete:
		r.Updated, err = db.execDelete(ptr)
	default:
		panic("unreachable")
	}
	return
}

// 你来实现（把解析出的 CREATE TABLE 语句变成一份 Schema 存进 KV）：
//  1. db.GetSchema(stmt.table) 若没报错，说明表已存在，返回 errors.New("duplicate table name")
//  2. schema := Schema{Table: stmt.table, Cols: stmt.cols}
//  3. schema.PKey, err = lookupColumns(stmt.cols, stmt.pkey)（lookupColumns 已实现好，把主键列名转成下标）；出错直接 return
//  4. json.Marshal(schema) 序列化，用 check(err == nil) 断言不应该出错
//  5. db.KV.Set([]byte("@schema_"+stmt.table), val) 写进 KV；出错直接 return
//  6. db.tables[schema.Table] = schema 顺手缓存，return nil
func (db *DB) execCreateTable(stmt *StmtCreatTable) (err error)

func (db *DB) GetSchema(table string) (Schema, error) {
	schema, ok := db.tables[table]
	if !ok {
		val, ok, err := db.KV.Get([]byte("@schema_" + table))
		if err == nil && ok {
			err = json.Unmarshal(val, &schema)
		}
		if err != nil {
			return Schema{}, err
		}
		if !ok {
			return Schema{}, errors.New("table is not found")
		}
		db.tables[table] = schema
	}
	return schema, nil
}

// 你来实现（执行 SELECT：按主键查一行，再挑出 SELECT 的列）：
//  1. schema, err := db.GetSchema(stmt.table)；出错直接 return nil, err
//  2. indices, err := lookupColumns(schema.Cols, stmt.cols)——把要选的列名转成下标；出错直接 return
//  3. row, err := makePKey(&schema, stmt.keys)——makePKey 已实现好，把 WHERE 里的等值条件拼成主键 Row；出错直接 return
//  4. ok, err := db.Select(&schema, row)；err!=nil 或 !ok 都直接 return nil, err（查不到就是 nil 行不是错误）
//  5. row = subsetRow(row, indices)（已实现好，按下标挑列），return []Row{row}, nil
func (db *DB) execSelect(stmt *StmtSelect) ([]Row, error)

// 你来实现（执行 INSERT：校验列数/列类型都对上 schema，再插入）：
//  1. schema, err := db.GetSchema(stmt.table)；出错直接 return 0, err
//  2. len(schema.Cols) != len(stmt.value) 说明给的值个数不对，报 "schema mismatch"
//  3. 逐列比较 schema.Cols[i].Type 和 stmt.value[i].Type，任一不匹配同样报 "schema mismatch"
//  4. updated, err := db.Insert(&schema, stmt.value)；出错直接 return
//  5. updated 为 true 说明真插入了一行，count++，最后 return count, nil
func (db *DB) execInsert(stmt *StmtInsert) (count int, err error)

// 你来实现（执行 UPDATE：按主键查到旧行，用 SET 的值覆盖非主键列，再写回）：
//  1. schema, err := db.GetSchema(stmt.table)；出错直接 return 0, err
//  2. row, err := makePKey(&schema, stmt.keys) 拼出主键行；出错直接 return
//  3. ok, err := db.Select(&schema, row)；err!=nil 或 !ok 直接 return 0, err（没查到就不更新）
//  4. fillNonPKey(&schema, stmt.value, row)（已实现好，把 SET 的值填进 row 对应列，不允许改主键列）；出错直接 return
//  5. updated, err := db.Update(&schema, row)；出错直接 return
//  6. updated 为 true 则 count++，return count, nil
func (db *DB) execUpdate(stmt *StmtUpdate) (count int, err error)

// 你来实现（执行 DELETE：按主键查到行就删）：
//  1. schema, err := db.GetSchema(stmt.table)；出错直接 return 0, err
//  2. row, err := makePKey(&schema, stmt.keys) 拼出主键行；出错直接 return
//  3. ok, err := db.Select(&schema, row)；err!=nil 或 !ok 直接 return 0, err
//  4. deleted, err := db.Delete(&schema, row)；出错直接 return
//  5. deleted 为 true 则 count++，return count, nil
func (db *DB) execDelete(stmt *StmtDelete) (count int, err error)

// UzBVUkNF https://systems-programming.org/
