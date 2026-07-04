package db0404

import (
	"encoding/json"
	"errors"
	"slices"
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

type RowIterator struct {
	schema *Schema
	iter   *KVIterator
	valid  bool
	row    Row
}

// 你来实现（把底层 KVIterator 当前指向的一对 key/val 解码成一行 Row；返回 false 表示"这一行不属于这张表了，扫描该停"）：
//  1. !iter.Valid()：底层游标已经越界，return false, nil
//  2. row.DecodeKey(schema, iter.Key())：err == ErrOutOfRange 说明扫到别的表的数据了（表名前缀不对），return false, nil；
//     err != nil 是真正的解析错误，return false, err
//  3. row.DecodeVal(schema, iter.Val())，出错 return false, err
//  4. 都通过：return true, nil
func decodeKVIter(schema *Schema, iter *KVIterator, row Row) (bool, error) {
	if !iter.Valid() {
		return false, nil
	}
	if err := row.DecodeKey(schema, iter.Key()); err != nil {
		if err == ErrOutOfRange {
			return false, nil
		}
		return false, err
	}
	if err := row.DecodeVal(schema, iter.Val()); err != nil {
		return false, err
	}
	return true, nil
}

func (iter *RowIterator) Valid() bool {
	return iter.valid
}

func (iter *RowIterator) Row() Row {
	check(iter.valid)
	return iter.row
}

func (iter *RowIterator) Next() (err error) {
	if err = iter.iter.Next(); err != nil {
		return err
	}
	iter.valid, err = decodeKVIter(iter.schema, iter.iter, iter.row)
	return err
}

// 你来实现（定位到一张表里某个位置，返回一个可以 Next() 逐行往后扫的 RowIterator；
// 传进来的 row 通常是只填了部分主键列的"前缀"，靠 EncodeKey 编码复用底层 KV.Seek 的二分查找）：
//  1. iter, err := db.KV.Seek(row.EncodeKey(schema))，出错 return nil, err
//  2. valid, err := decodeKVIter(schema, iter, row) 立即尝试解出第一行；出错 return nil, err
//  3. return &RowIterator{schema, iter, valid, row}, nil
func (db *DB) Seek(schema *Schema, row Row) (*RowIterator, error) {
	iter, err := db.KV.Seek(row.EncodeKey(schema))
	if err != nil {
		return nil, err
	}
	valid, err := decodeKVIter(schema, iter, row)
	if err != nil {
		return nil, err
	}
	return &RowIterator{schema, iter, valid, row}, nil
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

func (db *DB) execCreateTable(stmt *StmtCreatTable) (err error) {
	if _, err := db.GetSchema(stmt.table); err == nil {
		return errors.New("duplicate table name")
	}

	schema := Schema{
		Table: stmt.table,
		Cols:  stmt.cols,
	}
	if schema.PKey, err = lookupColumns(stmt.cols, stmt.pkey); err != nil {
		return err
	}

	val, err := json.Marshal(schema)
	check(err == nil)
	if _, err = db.KV.Set([]byte("@schema_"+stmt.table), val); err != nil {
		return err
	}

	db.tables[schema.Table] = schema
	return nil
}

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

func lookupColumns(cols []Column, names []string) (indices []int, err error) {
	for _, name := range names {
		idx := slices.IndexFunc(cols, func(col Column) bool {
			return col.Name == name
		})
		if idx < 0 {
			return nil, errors.New("column is not found")
		}
		indices = append(indices, idx)
	}
	return
}

func makePKey(schema *Schema, pkey []NamedCell) (Row, error) {
	if len(schema.PKey) != len(pkey) {
		return nil, errors.New("not primary key")
	}
	row := schema.NewRow()
	for _, idx1 := range schema.PKey {
		col := schema.Cols[idx1]
		idx2 := slices.IndexFunc(pkey, func(expr NamedCell) bool {
			return expr.column == col.Name && expr.value.Type == col.Type
		})
		if idx2 < 0 {
			return nil, errors.New("not primary key")
		}
		row[idx1] = pkey[idx2].value
	}
	return row, nil
}

func subsetRow(row Row, indices []int) (out Row) {
	for _, idx := range indices {
		out = append(out, row[idx])
	}
	return
}

func (db *DB) execSelect(stmt *StmtSelect) ([]Row, error) {
	schema, err := db.GetSchema(stmt.table)
	if err != nil {
		return nil, err
	}
	indices, err := lookupColumns(schema.Cols, stmt.cols)
	if err != nil {
		return nil, err
	}

	row, err := makePKey(&schema, stmt.keys)
	if err != nil {
		return nil, err
	}
	if ok, err := db.Select(&schema, row); err != nil || !ok {
		return nil, err
	}

	row = subsetRow(row, indices)
	return []Row{row}, nil
}

func (db *DB) execInsert(stmt *StmtInsert) (count int, err error) {
	schema, err := db.GetSchema(stmt.table)
	if err != nil {
		return 0, err
	}
	if len(schema.Cols) != len(stmt.value) {
		return 0, errors.New("schema mismatch")
	}
	for i := range schema.Cols {
		if schema.Cols[i].Type != stmt.value[i].Type {
			return 0, errors.New("schema mismatch")
		}
	}

	updated, err := db.Insert(&schema, stmt.value)
	if err != nil {
		return 0, err
	}
	if updated {
		count++
	}
	return count, nil
}

func fillNonPKey(schema *Schema, updates []NamedCell, out Row) error {
	for _, expr := range updates {
		idx := slices.IndexFunc(schema.Cols, func(col Column) bool {
			return col.Name == expr.column && col.Type == expr.value.Type
		})
		if idx < 0 || slices.Contains(schema.PKey, idx) {
			return errors.New("cannot update column")
		}
		out[idx] = expr.value
	}
	return nil
}

func (db *DB) execUpdate(stmt *StmtUpdate) (count int, err error) {
	schema, err := db.GetSchema(stmt.table)
	if err != nil {
		return 0, err
	}

	row, err := makePKey(&schema, stmt.keys)
	if err != nil {
		return 0, err
	}
	if ok, err := db.Select(&schema, row); err != nil || !ok {
		return 0, err
	}

	if err = fillNonPKey(&schema, stmt.value, row); err != nil {
		return 0, err
	}
	updated, err := db.Update(&schema, row)
	if err != nil {
		return 0, err
	}
	if updated {
		count++
	}
	return count, nil
}

func (db *DB) execDelete(stmt *StmtDelete) (count int, err error) {
	schema, err := db.GetSchema(stmt.table)
	if err != nil {
		return 0, err
	}

	row, err := makePKey(&schema, stmt.keys)
	if err != nil {
		return 0, err
	}

	updated, err := db.Delete(&schema, row)
	if err != nil {
		return 0, err
	}
	if updated {
		count++
	}
	return count, nil
}

// UzBVUkNF https://systems-programming.org/
