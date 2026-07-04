package db0303

import (
	"errors"
	"strconv"
	"strings"
)

type Parser struct {
	buf string
	pos int
}

func NewParser(s string) Parser {
	return Parser{buf: s, pos: 0}
}

type StmtSelect struct {
	table string
	cols  []string
	keys  []NamedCell
}

type NamedCell struct {
	column string
	value  Cell
}

func isSpace(ch byte) bool {
	switch ch {
	case '\t', '\n', '\v', '\f', '\r', ' ':
		return true
	}
	return false
}
func isAlpha(ch byte) bool {
	return 'a' <= (ch|32) && (ch|32) <= 'z'
}
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
func isNameStart(ch byte) bool {
	return isAlpha(ch) || ch == '_'
}
func isNameContinue(ch byte) bool {
	return isAlpha(ch) || isDigit(ch) || ch == '_'
}
func isSeparator(ch byte) bool {
	return ch < 128 && !isNameContinue(ch)
}

func (p *Parser) skipSpaces() {
	for p.pos < len(p.buf) && isSpace(p.buf[p.pos]) {
		p.pos += 1
	}
}

func (p *Parser) tryKeyword(kw string) bool {
	p.skipSpaces()
	if !(p.pos+len(kw) <= len(p.buf) && strings.EqualFold(p.buf[p.pos:p.pos+len(kw)], kw)) {
		return false
	}
	if p.pos+len(kw) < len(p.buf) && !isSeparator(p.buf[p.pos+len(kw)]) {
		return false
	}
	p.pos += len(kw)
	return true
}

func (p *Parser) tryPunctuation(tok string) bool {
	p.skipSpaces()
	if !(p.pos+len(tok) <= len(p.buf) && p.buf[p.pos:p.pos+len(tok)] == tok) {
		return false
	}
	p.pos += len(tok)
	return true
}

func (p *Parser) tryName() (string, bool) {
	p.skipSpaces()
	start, cur := p.pos, p.pos
	if !(cur < len(p.buf) && isNameStart(p.buf[cur])) {
		return "", false
	}
	cur++
	for cur < len(p.buf) && isNameContinue(p.buf[cur]) {
		cur++
	}
	p.pos = cur
	return p.buf[start:cur], true
}

func (p *Parser) parseValue(out *Cell) error {
	p.skipSpaces()
	if p.pos >= len(p.buf) {
		return errors.New("expect value")
	}
	ch := p.buf[p.pos]
	if ch == '"' || ch == '\'' {
		return p.parseString(out)
	} else if isDigit(ch) || ch == '-' || ch == '+' {
		return p.parseInt(out)
	} else {
		return errors.New("expect value")
	}
}

func (p *Parser) parseString(out *Cell) error {
	quote := p.buf[p.pos]
	cur := p.pos + 1
	for cur < len(p.buf) {
		ch := p.buf[cur]
		if ch == '\\' {
			cur++
			if cur < len(p.buf) && (p.buf[cur] == '"' || p.buf[cur] == '\'') {
				out.Str = append(out.Str, p.buf[cur])
				cur++
			} else {
				return errors.New("bad escape")
			}
		} else if ch == quote {
			out.Type = TypeStr
			p.pos = cur + 1
			return nil
		} else {
			out.Str = append(out.Str, p.buf[cur])
			cur++
		}
	}
	return errors.New("string is not terminated")
}

func (p *Parser) parseInt(out *Cell) (err error) {
	start, cur := p.pos, p.pos
	if p.buf[cur] == '-' || p.buf[cur] == '+' {
		cur++
	}
	for cur < len(p.buf) && isDigit(p.buf[cur]) {
		cur++
	}

	if out.I64, err = strconv.ParseInt(p.buf[start:cur], 10, 64); err != nil {
		return err
	}
	out.Type = TypeI64
	p.pos = cur
	return nil
}

// 你来实现（解析一个 "列名 = 值" 的等值条件）：
//  1. out.column, ok := p.tryName()；拿不到列名报 "expect column"
//  2. p.tryPunctuation("=") 匹配等号，匹配不到报 "expect ="
//  3. 剩下交给 p.parseValue(&out.value) 解析等号右边的值，直接 return 它的结果
func (p *Parser) parseEqual(out *NamedCell) error {
	name, ok := p.tryName()
	if !ok {
		return errors.New("expect column")
	}
	out.column = name
	if !p.tryPunctuation("=") {
		return errors.New("expect =")
	}
	return p.parseValue(&out.value)
}

// 你来实现（解析 SELECT col1, col2 FROM table WHERE ...）：
//  1. p.tryKeyword("SELECT") 打头，失败报 "expect keyword"
//  2. for !p.tryKeyword("FROM")：不断读列名 append 进 out.cols；除第一个列名外，读之前先要求 tryPunctuation(",")
//  3. 循环结束若 out.cols 为空，报 "expect column list"
//  4. FROM 后 p.tryName() 拿表名存 out.table，拿不到报 "expect table name"
//  5. 剩下交给 p.parseWhere(&out.keys)，直接 return 它的结果
func (p *Parser) parseSelect(out *StmtSelect) error {
	if !p.tryKeyword("SELECT") {
		return errors.New("expect keyword")
	}
	for !p.tryKeyword("FROM") {
		if len(out.cols) > 0 && !p.tryPunctuation(",") {
			break
		}
		name, ok := p.tryName()
		if !ok {
			break
		}
		out.cols = append(out.cols, name)
	}
	if len(out.cols) == 0 {
		return errors.New("expect column list")
	}
	table, ok := p.tryName()
	if !ok {
		return errors.New("expect table name")
	}
	out.table = table
	return p.parseWhere(&out.keys)
}

// 你来实现（解析 WHERE col=val AND col=val ... ;）：
//  1. p.tryKeyword("WHERE") 打头，失败报 "expect keyword"
//  2. for !p.tryPunctuation(";")：每轮 new 一个 NamedCell，除第一轮外先要求 tryKeyword("AND")；
//     调 p.parseEqual(&expr) 解析这一个等值条件，append 进 *out
//  3. 循环结束若 *out 为空（一个条件都没有），报 "expect where clause"
//  4. 都通过 return nil
func (p *Parser) parseWhere(out *[]NamedCell) error {
	if !p.tryKeyword("WHERE") {
		return errors.New("expect keyword")
	}
	for !p.tryPunctuation(";") {
		if len(*out) > 0 && !p.tryKeyword("AND") {
			break
		}
		var expr NamedCell
		if err := p.parseEqual(&expr); err != nil {
			return err
		}
		*out = append(*out, expr)
	}
	if len(*out) == 0 {
		return errors.New("expect where clause")
	}
	return nil
}

func (p *Parser) isEnd() bool {
	p.skipSpaces()
	return p.pos >= len(p.buf)
}

// UzBVUkNF https://systems-programming.org/
