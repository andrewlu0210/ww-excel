// Package lexer tokenizes formula text (without the leading "=").
package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/model"
)

type Kind int

const (
	EOF Kind = iota
	Number
	String
	Bool
	Error
	Ref   // cell, column range or row range; Area is set
	Sheet // sheet prefix including "!"; Text is the sheet name
	Func  // identifier followed by "("
	Name  // other identifier
	Op    // + - * / ^ & = <> < > <= >=
	Percent
	Colon
	LParen
	RParen
	LBrace
	RBrace
	Comma
	Semicolon
)

type Token struct {
	Kind Kind
	Text string
	Pos  int // byte offset in the source
	End  int
	Num  float64
	Area ast.Area
}

type Error_ struct {
	Pos int
	Msg string
}

func (e *Error_) Error() string { return fmt.Sprintf("%s（位置 %d）", e.Msg, e.Pos+1) }

var errorLiterals = []string{
	model.ErrDiv0, model.ErrValue, model.ErrNull, model.ErrName, model.ErrNum,
	model.ErrRef, model.ErrSpill, model.ErrCalc, model.ErrNA,
}

// Tokenize splits src into tokens, ending with EOF.
func Tokenize(src string) ([]Token, error) {
	var toks []Token
	i := 0
	for {
		for i < len(src) && isSpace(src[i]) {
			i++
		}
		if i >= len(src) {
			toks = append(toks, Token{Kind: EOF, Pos: i, End: i})
			return toks, nil
		}
		tok, err := next(src, i)
		if err != nil {
			return nil, err
		}
		toks = append(toks, tok)
		i = tok.End
	}
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func next(src string, i int) (Token, error) {
	c := src[i]
	single := func(k Kind) (Token, error) {
		return Token{Kind: k, Text: src[i : i+1], Pos: i, End: i + 1}, nil
	}
	switch c {
	case '(':
		return single(LParen)
	case ')':
		return single(RParen)
	case '{':
		return single(LBrace)
	case '}':
		return single(RBrace)
	case ',':
		return single(Comma)
	case ';':
		return single(Semicolon)
	case ':':
		return single(Colon)
	case '%':
		return single(Percent)
	case '+', '-', '*', '/', '^', '&', '=':
		return single(Op)
	case '<':
		if i+1 < len(src) && (src[i+1] == '=' || src[i+1] == '>') {
			return Token{Kind: Op, Text: src[i : i+2], Pos: i, End: i + 2}, nil
		}
		return single(Op)
	case '>':
		if i+1 < len(src) && src[i+1] == '=' {
			return Token{Kind: Op, Text: ">=", Pos: i, End: i + 2}, nil
		}
		return single(Op)
	case '"':
		return lexString(src, i)
	case '\'':
		return lexQuotedSheet(src, i)
	case '#':
		for _, lit := range errorLiterals {
			if len(src)-i >= len(lit) && strings.EqualFold(src[i:i+len(lit)], lit) {
				return Token{Kind: Error, Text: lit, Pos: i, End: i + len(lit)}, nil
			}
		}
		return Token{}, &Error_{Pos: i, Msg: "無法辨識的錯誤值"}
	}

	if tok, ok := lexRowRange(src, i); ok {
		return tok, nil
	}
	if c >= '0' && c <= '9' || c == '.' {
		return lexNumber(src, i)
	}
	if c == '$' || isIdentStart(src, i) {
		return lexWord(src, i)
	}
	r, _ := utf8.DecodeRuneInString(src[i:])
	return Token{}, &Error_{Pos: i, Msg: fmt.Sprintf("無法辨識的字元「%c」", r)}
}

func lexString(src string, i int) (Token, error) {
	var b strings.Builder
	j := i + 1
	for j < len(src) {
		if src[j] == '"' {
			if j+1 < len(src) && src[j+1] == '"' {
				b.WriteByte('"')
				j += 2
				continue
			}
			return Token{Kind: String, Text: b.String(), Pos: i, End: j + 1}, nil
		}
		b.WriteByte(src[j])
		j++
	}
	return Token{}, &Error_{Pos: i, Msg: "字串缺少結尾的引號"}
}

func lexQuotedSheet(src string, i int) (Token, error) {
	var b strings.Builder
	j := i + 1
	for j < len(src) {
		if src[j] == '\'' {
			if j+1 < len(src) && src[j+1] == '\'' {
				b.WriteByte('\'')
				j += 2
				continue
			}
			if j+1 < len(src) && src[j+1] == '!' {
				return Token{Kind: Sheet, Text: b.String(), Pos: i, End: j + 2}, nil
			}
			return Token{}, &Error_{Pos: i, Msg: "工作表名稱後面缺少「!」"}
		}
		b.WriteByte(src[j])
		j++
	}
	return Token{}, &Error_{Pos: i, Msg: "工作表名稱缺少結尾的單引號"}
}

func lexNumber(src string, i int) (Token, error) {
	j := i
	for j < len(src) && isDigit(src[j]) {
		j++
	}
	if j < len(src) && src[j] == '.' {
		j++
		for j < len(src) && isDigit(src[j]) {
			j++
		}
	}
	if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
		k := j + 1
		if k < len(src) && (src[k] == '+' || src[k] == '-') {
			k++
		}
		if k < len(src) && isDigit(src[k]) {
			for k < len(src) && isDigit(src[k]) {
				k++
			}
			j = k
		}
	}
	n, err := strconv.ParseFloat(src[i:j], 64)
	if err != nil {
		return Token{}, &Error_{Pos: i, Msg: "無效的數字"}
	}
	return Token{Kind: Number, Text: src[i:j], Pos: i, End: j, Num: n}, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentStart(src string, i int) bool {
	r, _ := utf8.DecodeRuneInString(src[i:])
	return r == '_' || r == '\\' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || r == '.' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// lexRowRange matches "1:3" or "$1:$3".
func lexRowRange(src string, i int) (Token, bool) {
	r0, abs0, j, ok := rowPart(src, i)
	if !ok || j >= len(src) || src[j] != ':' {
		return Token{}, false
	}
	r1, abs1, k, ok := rowPart(src, j+1)
	if !ok || (k < len(src) && isWordByte(src[k])) {
		return Token{}, false
	}
	a := ast.Area{Kind: ast.RowArea, R0: r0, R1: r1, C1: model.MaxCols - 1, AbsR0: abs0, AbsR1: abs1}
	if a.R0 > a.R1 {
		a.R0, a.R1, a.AbsR0, a.AbsR1 = a.R1, a.R0, a.AbsR1, a.AbsR0
	}
	return Token{Kind: Ref, Text: src[i:k], Pos: i, End: k, Area: a}, true
}

func rowPart(src string, i int) (row int, abs bool, end int, ok bool) {
	if i < len(src) && src[i] == '$' {
		abs = true
		i++
	}
	j := i
	for j < len(src) && isDigit(src[j]) {
		j++
	}
	if j == i {
		return 0, false, 0, false
	}
	n, err := strconv.Atoi(src[i:j])
	if err != nil || n < 1 || n > model.MaxRows {
		return 0, false, 0, false
	}
	return n - 1, abs, j, true
}

func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '$' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= 0x80
}

func lexWord(src string, i int) (Token, error) {
	j := i
	for j < len(src) {
		r, size := utf8.DecodeRuneInString(src[j:])
		if !isIdentPart(r) && !(j == i && r == '\\') {
			break
		}
		j += size
	}
	word := src[i:j]
	k := j
	for k < len(src) && isSpace(src[k]) {
		k++
	}
	nextByte := byte(0)
	if j < len(src) {
		nextByte = src[j]
	}

	switch {
	case nextByte == '!':
		return Token{Kind: Sheet, Text: word, Pos: i, End: j + 1}, nil
	case k < len(src) && src[k] == '(' && !strings.Contains(word, "$"):
		return Token{Kind: Func, Text: strings.ToUpper(word), Pos: i, End: j}, nil
	}
	if strings.EqualFold(word, "TRUE") || strings.EqualFold(word, "FALSE") {
		return Token{Kind: Bool, Text: strings.ToUpper(word), Pos: i, End: j}, nil
	}
	if a, ok := parseCell(word); ok {
		return Token{Kind: Ref, Text: word, Pos: i, End: j, Area: a}, nil
	}
	// Column range A:C
	if c0, abs0, ok := parseCol(word); ok && nextByte == ':' {
		w2 := j + 1
		e := w2
		for e < len(src) && (src[e] == '$' || src[e] >= 'A' && src[e] <= 'Z' || src[e] >= 'a' && src[e] <= 'z') {
			e++
		}
		if c1, abs1, ok := parseCol(src[w2:e]); ok && (e >= len(src) || !isWordByte(src[e])) {
			a := ast.Area{Kind: ast.ColArea, C0: c0, C1: c1, R1: model.MaxRows - 1, AbsC0: abs0, AbsC1: abs1}
			if a.C0 > a.C1 {
				a.C0, a.C1, a.AbsC0, a.AbsC1 = a.C1, a.C0, a.AbsC1, a.AbsC0
			}
			return Token{Kind: Ref, Text: src[i:e], Pos: i, End: e, Area: a}, nil
		}
	}
	if strings.Contains(word, "$") {
		return Token{}, &Error_{Pos: i, Msg: fmt.Sprintf("無效的參照「%s」", word)}
	}
	return Token{Kind: Name, Text: word, Pos: i, End: j}, nil
}

// parseCell parses $A$1 style references.
func parseCell(word string) (ast.Area, bool) {
	i := 0
	absCol := false
	if i < len(word) && word[i] == '$' {
		absCol = true
		i++
	}
	j := i
	for j < len(word) && (word[j] >= 'A' && word[j] <= 'Z' || word[j] >= 'a' && word[j] <= 'z') {
		j++
	}
	col, ok := ast.ColIndex(word[i:j])
	if !ok {
		return ast.Area{}, false
	}
	row, absRow, end, ok := rowPart(word, j)
	if !ok || end != len(word) {
		return ast.Area{}, false
	}
	return ast.CellAt(row, col, absRow, absCol), true
}

func parseCol(word string) (int, bool, bool) {
	abs := strings.HasPrefix(word, "$")
	col, ok := ast.ColIndex(strings.TrimPrefix(word, "$"))
	return col, abs, ok
}
