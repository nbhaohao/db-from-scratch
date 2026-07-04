package db0403

import (
	"errors"
	"slices"
)

type Schema struct {
	Table string
	Cols  []Column
	PKey  []int // indexes of primary key columns
}

type Column struct {
	Name string
	Type CellType
}

type Row []Cell

func (schema *Schema) NewRow() Row {
	return make(Row, len(schema.Cols))
}

// 你来实现（把主键列拼成一个保序的 key：前缀表名+0x00 区分不同表，再依次拼各主键列的 EncodeKey）：
//  1. key = append(key, []byte(schema.Table)...) 再 append 一个 0x00 分隔符
//  2. check(len(row) == len(schema.Cols)) 断言行的列数和 schema 对得上
//  3. 遍历 schema.PKey（主键列下标列表）：check(row[idx].Type == schema.Cols[idx].Type)，
//     然后 key = row[idx].EncodeKey(key)（用 Cell 保序编码，不是 EncodeVal）
//  4. return key
func (row Row) EncodeKey(schema *Schema) (key []byte) {
	key = append(key, []byte(schema.Table)...)
	key = append(key, 0x00)
	check(len(row) == len(schema.Cols))
	for _, idx := range schema.PKey {
		check(row[idx].Type == schema.Cols[idx].Type)
		key = row[idx].EncodeKey(key)
	}
	return key
}

func (row Row) EncodeVal(schema *Schema) (val []byte) {
	check(len(row) == len(schema.Cols))
	for idx, value := range row {
		if !slices.Contains(schema.PKey, idx) {
			check(value.Type == schema.Cols[idx].Type)
			val = row[idx].EncodeVal(val)
		}
	}
	return val
}

// 你来实现（EncodeKey 的逆过程：先校验表名前缀，再依次解码各主键列）：
//  1. check(len(row) == len(schema.Cols))
//  2. key 长度不够 len(schema.Table)+1，或前缀不等于 schema.Table+"\x00"：报 "bad key"
//  3. key = key[len(schema.Table)+1:] 去掉前缀
//  4. 遍历 schema.PKey：row[idx] = Cell{Type: schema.Cols[idx].Type}，key, err = row[idx].DecodeKey(key)，出错直接 return
//  5. 解完 key 应该正好用完，len(key) != 0 报 "trailing garbage"，否则 return nil
func (row Row) DecodeKey(schema *Schema, key []byte) (err error) {
	check(len(row) == len(schema.Cols))

	if len(key) < len(schema.Table)+1 {
		return errors.New("bad key")
	}
	if string(key[:len(schema.Table)+1]) != schema.Table+"\x00" {
		return errors.New("bad key")
	}
	key = key[len(schema.Table)+1:]

	for _, idx := range schema.PKey {
		row[idx] = Cell{Type: schema.Cols[idx].Type}
		if key, err = row[idx].DecodeKey(key); err != nil {
			return err
		}
	}

	if len(key) != 0 {
		return errors.New("trailing garbage")
	}
	return nil
}

func (row Row) DecodeVal(schema *Schema, val []byte) (err error) {
	check(len(row) == len(schema.Cols))

	for idx, col := range schema.Cols {
		if slices.Contains(schema.PKey, idx) {
			continue
		}
		row[idx] = Cell{Type: col.Type}
		if val, err = row[idx].DecodeVal(val); err != nil {
			return err
		}
	}

	if len(val) != 0 {
		return errors.New("trailing garbage")
	}
	return nil
}

// UzBVUkNF https://systems-programming.org/
