package db0201

import (
	"fmt"
	"slices"
)

type CellType uint8

const (
	TypeI64 CellType = 1
	TypeStr CellType = 2
)

type Cell struct {
	Type CellType
	I64  int64
	Str  []byte
}

// 你来实现（把序列化结果 append 到 toAppend 后返回，别自己 new slice）：
//  1. 按 cell.Type 分两种情况（switch）
//  2. TypeI64：把 I64 以 8 字节小端 append（binary.LittleEndian 只有 Uint64 方法，
//     int64 直接转 uint64 —— 补码位模式不变，转换是零成本）
//  3. TypeStr：先 append 4 字节小端的长度 uint32(len)，再 append 数据本身
//  4. 其他 Type 不可能出现（panic("unreachable")）
func (cell *Cell) Encode(toAppend []byte) []byte {
	switch cell.Type {
	case TypeI64:
		toAppend = append(toAppend, byte(cell.I64&0xff), byte(cell.I64>>8&0xff), byte(cell.I64>>16&0xff), byte(cell.I64>>24&0xff), byte(cell.I64>>32&0xff), byte(cell.I64>>40&0xff), byte(cell.I64>>48&0xff), byte(cell.I64>>56&0xff))
	case TypeStr:
		toAppend = append(toAppend, byte(len(cell.Str)&0xff), byte(len(cell.Str)>>8&0xff), byte(len(cell.Str)>>16&0xff), byte(len(cell.Str)>>24&0xff))
		toAppend = append(toAppend, cell.Str...)
	default:
		panic("unreachable")
	}
	return toAppend
}

// 你来实现（从 data 头部解出一个 cell，返回没用完的 rest 给下一个 cell 用）：
//  1. 按 cell.Type 分两种情况（switch）
//  2. TypeI64：不足 8 字节先报错；读前 8 字节小端转 int64 存进 cell.I64，rest = 剩余部分
//  3. TypeStr：不足 4 字节先报错；读长度，再检查数据够不够长度，
//     拷贝出数据段（slices.Clone，别直接引用 data 的内存），rest = 剩余部分
//  4. 每一步"数据不够"都返回 error，不要 panic —— 输入可能来自损坏的文件
func (cell *Cell) Decode(data []byte) (rest []byte, err error) {
	switch cell.Type {
	case TypeI64:
		if len(data) < 8 {
			return nil, fmt.Errorf("data too short")
		}
		cell.I64 = int64(data[0]) | int64(data[1])<<8 | int64(data[2])<<16 | int64(data[3])<<24 | int64(data[4])<<32 | int64(data[5])<<40 | int64(data[6])<<48 | int64(data[7])<<56
		rest = data[8:]
	case TypeStr:
		if len(data) < 4 {
			return nil, fmt.Errorf("data too short")
		}
		length := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
		if len(data) < 4+length {
			return nil, fmt.Errorf("data too short")
		}
		cell.Str = slices.Clone(data[4 : 4+length])
		rest = data[4+length:]
	default:
		panic("unreachable")
	}
	return rest, nil
}

// UzBVUkNF https://systems-programming.org/
