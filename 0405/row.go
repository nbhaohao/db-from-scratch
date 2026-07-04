package db0405

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

// 你来实现（比上一章多了两处：每个主键 cell 前面多编码一个类型字节 byte(cell.Type)，
// 整个 key 结尾多补一个 0x00——这样"表名+完整主键"和"表名+主键前缀"在字节序上能被严格区分开，
// 是范围查询靠前缀定位边界的基础）：
//  1. check(len(row) == len(schema.Cols))
//  2. key := append([]byte(schema.Table), 0x00)
//  3. 遍历 schema.PKey：check(cell.Type == schema.Cols[idx].Type)；
//     key = append(key, byte(cell.Type))（先写类型字节）；key = cell.EncodeKey(key)（再写保序编码的值）
//  4. return append(key, 0x00)（结尾补 0x00，和 EncodeKeyPrefix 的 0xff 后缀区分开）
func (row Row) EncodeKey(schema *Schema) []byte {
	check(len(row) == len(schema.Cols))
	key := append([]byte(schema.Table), 0x00)
	for _, idx := range schema.PKey {
		cell := row[idx]
		check(cell.Type == schema.Cols[idx].Type)
		key = append(key, byte(cell.Type))
		key = cell.EncodeKey(key)
	}
	return append(key, 0x00)
}

// 你来实现（只编码主键的前 N 列（prefix 可能比完整主键短），用于构造范围查询的起止边界；
// positive 决定后缀是 0xff 还是不加——0xff 排在任何值后面，用来把"前缀匹配"变成"闭区间上界"或"开区间下界"）：
//  1. key := append([]byte(schema.Table), 0x00)
//  2. 遍历 prefix（i, cell）：check(cell.Type == schema.Cols[schema.PKey[i]].Type)；
//     key = append(key, byte(cell.Type))；key = cell.EncodeKey(key)
//  3. positive 为 true：key = append(key, 0xff)（补最大字节，让这个前缀排在所有"前缀+任何后续字节"之后）
//  4. return key
func EncodeKeyPrefix(schema *Schema, prefix []Cell, positive bool) []byte {
	key := append([]byte(schema.Table), 0x00)
	for i, cell := range prefix {
		check(cell.Type == schema.Cols[schema.PKey[i]].Type)
		key = append(key, byte(cell.Type))
		key = cell.EncodeKey(key)
	}
	if positive {
		key = append(key, 0xff)
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
		if !(len(key) > 0 && key[0] == byte(row[idx].Type)) {
			return errors.New("bad key")
		}
		key = key[1:]
		if key, err = row[idx].DecodeKey(key); err != nil {
			return err
		}
	}
	if !(len(key) == 1 && key[0] == 0x00) {
		return errors.New("bad key")
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
