package db0301

import "strings"

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

// 你来实现（大小写不敏感匹配一个关键字，匹配成功才前进 pos，失败原地不动）：
//  1. 先 skipSpaces() 跳过前导空白
//  2. 剩余长度不够 kw 长，或 strings.EqualFold(p.buf[p.pos:p.pos+len(kw)], kw) 不匹配，直接 return false
//  3. 关键字后面紧跟非分隔符（比如匹配到 "SELECT" 但源码其实是 "SELECTx"）也算失败，return false
//  4. 都通过：p.pos += len(kw)，return true
func (p *Parser) tryKeyword(kw string) bool {
	p.skipSpaces()
	if p.pos+len(kw) > len(p.buf) || !strings.EqualFold(p.buf[p.pos:p.pos+len(kw)], kw) {
		return false
	}
	end := p.pos + len(kw)
	if end < len(p.buf) && !isSeparator(p.buf[end]) {
		return false
	}
	p.pos = end
	return true
}

// 你来实现（贪婪匹配一个标识符：字母/下划线开头，后随字母数字下划线）：
//  1. 先 skipSpaces()
//  2. start, cur := p.pos, p.pos；cur 处若不满足 isNameStart，直接 return "", false
//  3. cur++ 后用 isNameContinue 继续往前扫，扫到不满足为止
//  4. p.pos = cur，return p.buf[start:cur], true
func (p *Parser) tryName() (string, bool) {
	p.skipSpaces()
	start, cur := p.pos, p.pos
	if cur >= len(p.buf) || !isNameStart(p.buf[cur]) {
		return "", false
	}
	cur++
	for cur < len(p.buf) && isNameContinue(p.buf[cur]) {
		cur++
	}
	p.pos = cur
	return p.buf[start:cur], true
}

func (p *Parser) isEnd() bool {
	p.skipSpaces()
	return p.pos >= len(p.buf)
}

// UzBVUkNF https://systems-programming.org/
