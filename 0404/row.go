package db0404

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

func (row Row) EncodeKey(schema *Schema) (key []byte) {
	key = append(key, []byte(schema.Table)...)
	key = append(key, 0x00)
	check(len(row) == len(schema.Cols))
	for _, idx := range schema.PKey {
		value := row[idx]
		check(value.Type == schema.Cols[idx].Type)
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

var ErrOutOfRange = errors.New("out of range")

// 你来实现（EncodeKey 的逆过程；注意这里"表名前缀不对"要返回哨兵错误 ErrOutOfRange 而不是普通 error——
// 范围扫描时靠这个信号判断"扫到别的表的数据了，该停了"，和真正的解析失败要区分开）：
//  1. check(len(row) == len(schema.Cols))
//  2. key 长度不够 len(schema.Table)+1，或前缀不等于 schema.Table+"\x00"：return ErrOutOfRange（不是 errors.New）
//  3. key = key[len(schema.Table)+1:] 去掉前缀
//  4. 遍历 schema.PKey：row[idx] = Cell{Type: schema.Cols[idx].Type}，key, err = row[idx].DecodeKey(key)，出错直接 return
//  5. len(key) != 0 报 "trailing garbage"（这个用普通 errors.New），否则 return nil
func (row Row) DecodeKey(schema *Schema, key []byte) (err error) {
	check(len(row) == len(schema.Cols))

	if len(key) < len(schema.Table)+1 {
		return ErrOutOfRange
	}
	if string(key[:len(schema.Table)+1]) != schema.Table+"\x00" {
		return ErrOutOfRange
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
