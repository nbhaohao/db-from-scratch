package db0302

import (
	"errors"
	"strings"
)

type Parser struct {
	buf string
	pos int
}

func NewParser(s string) Parser {
	return Parser{buf: s, pos: 0}
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

// 你来实现（从当前引号后开始扫描字符串字面量，处理转义，遇到配对引号结束）：
//  1. quote := p.buf[p.pos] 记住是单引号还是双引号；cur := p.pos+1 跳过开头引号
//  2. 逐字符扫描 cur：遇到反斜杠转义，看下一字符是不是引号字符，是就把它 append 进 out.Str（去掉反斜杠），cur+=2；不是就报 "bad escape"
//  3. 遇到和开头相同的 quote：字符串结束，out.Type = TypeStr，p.pos = cur+1（跳过收尾引号），return nil
//  4. 其他字符：append 进 out.Str，cur++
//  5. 扫到 buf 末尾还没遇到收尾引号：return errors.New("string is not terminated")
func (p *Parser) parseString(out *Cell) error

// 你来实现（解析一个有符号整数字面量）：
//  1. start, cur := p.pos, p.pos；开头若是 '-'/'+' 先 cur++ 跳过符号
//  2. 用 isDigit 继续往前扫数字
//  3. strconv.ParseInt(p.buf[start:cur], 10, 64) 解析出 out.I64，失败直接 return err
//  4. 成功：out.Type = TypeI64，p.pos = cur，return nil
func (p *Parser) parseInt(out *Cell) (err error)

func (p *Parser) isEnd() bool {
	p.skipSpaces()
	return p.pos >= len(p.buf)
}

// UzBVUkNF https://systems-programming.org/
