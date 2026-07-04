package db0202

import (
	"errors"
	"slices"
)

var ErrInvalidKey = errors.New("invalid key")

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

// 你来实现（主键列 → K；表名做前缀区分不同表）：
//  1. key 先放表名前缀：append 表名字节 + 一个 0x00 分隔符
//     （0x00 防止 "ab" 和 "abc" 这类包含关系的表名造成 key 冲突）
//  2. 校验 row 和 schema 匹配：长度相等、每列 Type 一致（用 check()）
//  3. 按 schema 列顺序遍历，只把「在 PKey 里的列」用 Cell.Encode append 进 key
//     （判断"第 idx 列是不是主键"：slices.Contains(schema.PKey, idx)）
func (row Row) EncodeKey(schema *Schema) (key []byte) {
	key = append(key, schema.Table...)
	key = append(key, 0x00)
	for idx, cell := range row {
		if slices.Contains(schema.PKey, idx) {
			key = cell.Encode(key)
		}
	}
	return key
}

// 你来实现（非主键列 → V，和 EncodeKey 互补）：
//  1. 同样校验 row 和 schema 匹配
//  2. 按列顺序遍历，只把「不在 PKey 里的列」encode 进 val（无表名前缀）
func (row Row) EncodeVal(schema *Schema) (val []byte) {
	for idx, cell := range row {
		if !slices.Contains(schema.PKey, idx) {
			val = cell.Encode(val)
		}
	}
	return val
}

// 你来实现（EncodeKey 的逆过程，结果原地写进 row 的主键列）：
//  1. 校验并剥掉前缀：key 至少要有 len(表名)+1 字节，且前缀 == 表名+"\x00"，否则报错
//  2. 按列顺序遍历，只处理主键列：先设 row[idx] 的 Type（从 schema 取），
//     再 Decode，把返回的 rest 接着往下解
//  3. 全部解完 key 应该刚好用尽——有剩余字节说明 key 有垃圾数据，报错
func (row Row) DecodeKey(schema *Schema, key []byte) (err error) {
	if len(key) < len(schema.Table)+1 || key[len(schema.Table)] != 0x00 {
		return ErrInvalidKey
	}
	key = key[len(schema.Table)+1:]
	for idx := range row {
		if slices.Contains(schema.PKey, idx) {
			row[idx].Type = schema.Cols[idx].Type
			key, err = row[idx].Decode(key)
			if err != nil {
				return err
			}
		}
	}
	if len(key) > 0 {
		return ErrInvalidKey
	}
	return nil
}

// 你来实现（EncodeVal 的逆过程，补全 row 的非主键列）：
//  1. 校验匹配后按列顺序遍历，只处理非主键列，套路同 DecodeKey 第 2、3 步
func (row Row) DecodeVal(schema *Schema, val []byte) (err error) {
	for idx := range row {
		if !slices.Contains(schema.PKey, idx) {
			row[idx].Type = schema.Cols[idx].Type
			val, err = row[idx].Decode(val)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// UzBVUkNF https://systems-programming.org/
