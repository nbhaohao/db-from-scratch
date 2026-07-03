package db0403

import (
	"encoding/binary"
	"errors"
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

func (cell *Cell) EncodeVal(toAppend []byte) []byte {
	switch cell.Type {
	case TypeI64:
		return binary.LittleEndian.AppendUint64(toAppend, uint64(cell.I64))
	case TypeStr:
		toAppend = binary.LittleEndian.AppendUint32(toAppend, uint32(len(cell.Str)))
		return append(toAppend, cell.Str...)
	default:
		panic("unreachable")
	}
}

func (cell *Cell) DecodeVal(data []byte) (rest []byte, err error) {
	switch cell.Type {
	case TypeI64:
		if len(data) < 8 {
			return data, errors.New("expect more data")
		}
		cell.I64 = int64(binary.LittleEndian.Uint64(data[0:8]))
		return data[8:], nil
	case TypeStr:
		if len(data) < 4 {
			return data, errors.New("expect more data")
		}
		size := int(binary.LittleEndian.Uint32(data[0:4]))
		if len(data) < 4+size {
			return data, errors.New("expect more data")
		}
		cell.Str = slices.Clone(data[4 : 4+size])
		return data[4+size:], nil
	default:
		panic("unreachable")
	}
}

// 你来实现（字符串的保序编码：用 0x00 当结尾符，所以数据里天然出现的 0x00/0x01 要转义掉，
// 否则会被误判成提前结束；转义规则：0x00→0x01 0x01，0x01→0x01 0x02，其他字节原样输出，最后补一个真正的 0x00 结尾）：
//  1. 遍历 input 的每个字节 ch：ch==0x00 或 ch==0x01 时，append 0x01 和 ch+1（转义对）；否则直接 append ch
//  2. 循环结束后 append 一个 0x00 作为真正的结尾符，return toAppend
func encodeStrKey(toAppend []byte, input []byte) []byte

// 你来实现（encodeStrKey 的逆过程：边扫边反转义，遇到未转义的 0x00 就是字符串结尾）：
//  1. 用 escape 标记"上一个字节是不是 0x01"；遍历 data 的每个字节 ch（下标 i）：
//     - escape 为 true：ch 必须是 0x01 或 0x02（否则报 "bad escape"），把 ch-1 append 进 out，escape 复位
//     - escape 为 false 且 ch==0x00：说明字符串结束，return out, data[i+1:], nil
//     - escape 为 false 且 ch==0x01：说明下一个字节是转义对的第二个字节，escape = true
//     - 否则：直接 append ch 进 out
//  2. 扫完 data 都没遇到结尾的 0x00：return nil, data, errors.New("string is not ended")
func decodeStrKey(data []byte) (out []byte, rest []byte, err error)

// 你来实现（key 的编码要求"字节序比较顺序 == 值的大小顺序"，这是范围查询的基础，和 EncodeVal 不是一回事）：
//  1. TypeI64：用 binary.BigEndian（大端才能保证字节序比较=数值比较）AppendUint64，
//     但 int64 是补码表示，负数的补码按无符号比较会排在正数后面，所以要把符号位翻转：uint64(cell.I64) ^ (1<<63)
//  2. TypeStr：调用上面的 encodeStrKey(toAppend, cell.Str)（转义处理 保证逐字节比较=字典序比较）
//  3. 其他 Type 不可能出现：panic("unreachable")
func (cell *Cell) EncodeKey(toAppend []byte) []byte

// 你来实现（EncodeKey 的逆过程）：
//  1. TypeI64：不足 8 字节报错；binary.BigEndian.Uint64(data[0:8]) 读出后再 ^(1<<63) 翻转回符号位，转 int64 存进 cell.I64
//  2. TypeStr：cell.Str, rest, err = decodeStrKey(data)，直接把三个返回值传回去
//  3. 其他 Type：panic("unreachable")
func (cell *Cell) DecodeKey(data []byte) (rest []byte, err error)

// UzBVUkNF https://systems-programming.org/
