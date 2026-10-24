// Package parser turns formula text into an AST with a Pratt parser
// using Excel's operator precedence:
//
//	:            range              (highest)
//	-  +         negation (prefix)
//	%            percent (postfix)
//	^            exponent (left associative, so 2^3^2 = 64; -2^2 = 4)
//	*  /
//	+  -
//	&            concatenation
//	= <> < > <= >=                  (lowest)
package parser

import (
	"fmt"
	"strings"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/lexer"
)

type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string { return e.Msg }

const (
	bpCompare = 10
	bpConcat  = 20
	bpAdd     = 30
	bpMul     = 40
	bpPow     = 50
	bpPrefix  = 60
	bpPercent = 70
	bpRange   = 80
)

type parser struct {
	toks []lexer.Token
	pos  int
}

// Parse parses formula text without the leading "=". Missing closing
// parentheses at the end are added, as Excel does.
func Parse(src string) (ast.Node, error) {
	toks, err := lexer.Tokenize(src)
	if err != nil {
		if le, ok := err.(*lexer.Error_); ok {
			return nil, &Error{Pos: le.Pos, Msg: le.Error()}
		}
		return nil, err
	}
	p := &parser{toks: toks}
	if p.peek().Kind == lexer.EOF {
		return nil, p.errf(p.peek(), "公式是空的")
	}
	n, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.Kind != lexer.EOF {
		if t.Kind == lexer.RParen {
			return nil, p.errf(t, "多了一個「)」")
		}
		return nil, p.errf(t, "這裡缺少運算子")
	}
	return n, nil
}

func (p *parser) peek() lexer.Token { return p.toks[p.pos] }

func (p *parser) advance() lexer.Token {
	t := p.toks[p.pos]
	if t.Kind != lexer.EOF {
		p.pos++
	}
	return t
}

func (p *parser) errf(t lexer.Token, format string, args ...any) *Error {
	return &Error{Pos: t.Pos, Msg: fmt.Sprintf(format, args...) + fmt.Sprintf("（位置 %d）", t.Pos+1)}
}

func infixBP(t lexer.Token) int {
	switch t.Kind {
	case lexer.Colon:
		return bpRange
	case lexer.Percent:
		return bpPercent
	case lexer.Op:
		switch t.Text {
		case "^":
			return bpPow
		case "*", "/":
			return bpMul
		case "+", "-":
			return bpAdd
		case "&":
			return bpConcat
		default:
			return bpCompare
		}
	}
	return 0
}

func (p *parser) expr(rbp int) (ast.Node, error) {
	left, err := p.prefix()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		bp := infixBP(t)
		if bp <= rbp {
			return left, nil
		}
		p.advance()
		switch t.Kind {
		case lexer.Percent:
			left = ast.Percent{X: left}
		case lexer.Colon:
			right, err := p.expr(bpRange)
			if err != nil {
				return nil, err
			}
			left, err = p.rangeOf(left, right, t)
			if err != nil {
				return nil, err
			}
		default:
			// All binary operators are left associative.
			right, err := p.expr(bp)
			if err != nil {
				return nil, err
			}
			left = ast.Binary{Op: t.Text, L: left, R: right}
		}
	}
}

func (p *parser) rangeOf(l, r ast.Node, at lexer.Token) (ast.Node, error) {
	lr, ok1 := l.(ast.Ref)
	rr, ok2 := r.(ast.Ref)
	if !ok1 || !ok2 {
		return nil, p.errf(at, "「:」兩邊必須是儲存格參照")
	}
	if rr.Sheet != "" && rr.Sheet != lr.Sheet {
		return nil, p.errf(at, "範圍不能跨工作表")
	}
	return ast.Ref{Sheet: lr.Sheet, Area: ast.Span(lr.Area, rr.Area)}, nil
}

func (p *parser) prefix() (ast.Node, error) {
	t := p.advance()
	switch t.Kind {
	case lexer.Number:
		return ast.Number{V: t.Num}, nil
	case lexer.String:
		return ast.String{V: t.Text}, nil
	case lexer.Bool:
		return ast.Bool{V: t.Text == "TRUE"}, nil
	case lexer.Error:
		return ast.Error{Code: t.Text}, nil
	case lexer.Ref:
		return ast.Ref{Area: t.Area}, nil
	case lexer.Sheet:
		ref := p.advance()
		if ref.Kind != lexer.Ref {
			return nil, p.errf(ref, "工作表名稱後面必須是儲存格參照")
		}
		return ast.Ref{Sheet: t.Text, Area: ref.Area}, nil
	case lexer.Name:
		return ast.Name{Name: t.Text}, nil
	case lexer.Func:
		return p.call(t)
	case lexer.LParen:
		inner, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		if err := p.closeParen(); err != nil {
			return nil, err
		}
		return ast.Paren{X: inner}, nil
	case lexer.LBrace:
		return p.array(t)
	case lexer.Op:
		if t.Text == "-" || t.Text == "+" {
			x, err := p.expr(bpPrefix)
			if err != nil {
				return nil, err
			}
			return ast.Unary{Op: t.Text[0], X: x}, nil
		}
	case lexer.EOF:
		return nil, p.errf(t, "公式不完整，結尾缺少運算元")
	}
	return nil, p.errf(t, "這裡不能出現「%s」", t.Text)
}

// closeParen consumes ")" or accepts end of input (auto-close).
func (p *parser) closeParen() error {
	t := p.peek()
	switch t.Kind {
	case lexer.RParen:
		p.advance()
		return nil
	case lexer.EOF:
		return nil
	}
	return p.errf(t, "這裡缺少「)」或「,」")
}

func (p *parser) call(name lexer.Token) (ast.Node, error) {
	p.advance() // "("
	// xlsx files prefix newer functions (_xlfn.XLOOKUP); drop it.
	fn := strings.TrimPrefix(strings.TrimPrefix(name.Text, "_XLFN."), "_XLWS.")
	call := ast.Call{Name: fn}
	if p.peek().Kind == lexer.RParen {
		p.advance()
		return call, nil
	}
	for {
		switch p.peek().Kind {
		case lexer.Comma, lexer.RParen:
			call.Args = append(call.Args, ast.Missing{})
		case lexer.EOF:
			if len(call.Args) > 0 {
				call.Args = append(call.Args, ast.Missing{})
			}
			return call, nil
		default:
			arg, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			call.Args = append(call.Args, arg)
		}
		if p.peek().Kind == lexer.Comma {
			p.advance()
			continue
		}
		if err := p.closeParen(); err != nil {
			return nil, err
		}
		return call, nil
	}
}

func (p *parser) array(open lexer.Token) (ast.Node, error) {
	arr := ast.Array{Rows: [][]ast.Node{{}}}
	for {
		el, err := p.arrayElement()
		if err != nil {
			return nil, err
		}
		row := len(arr.Rows) - 1
		arr.Rows[row] = append(arr.Rows[row], el)

		switch t := p.advance(); t.Kind {
		case lexer.Comma:
		case lexer.Semicolon:
			arr.Rows = append(arr.Rows, nil)
		case lexer.RBrace:
			width := len(arr.Rows[0])
			for _, r := range arr.Rows {
				if len(r) != width {
					return nil, p.errf(open, "陣列常數的每一列長度必須相同")
				}
			}
			return arr, nil
		default:
			return nil, p.errf(t, "陣列常數缺少「}」")
		}
	}
}

func (p *parser) arrayElement() (ast.Node, error) {
	t := p.advance()
	neg := false
	if t.Kind == lexer.Op && (t.Text == "-" || t.Text == "+") {
		neg = t.Text == "-"
		t = p.advance()
		if t.Kind != lexer.Number {
			return nil, p.errf(t, "陣列常數只能包含數字、文字、邏輯值或錯誤值")
		}
	}
	switch t.Kind {
	case lexer.Number:
		if neg {
			return ast.Number{V: -t.Num}, nil
		}
		return ast.Number{V: t.Num}, nil
	case lexer.String:
		return ast.String{V: t.Text}, nil
	case lexer.Bool:
		return ast.Bool{V: t.Text == "TRUE"}, nil
	case lexer.Error:
		return ast.Error{Code: t.Text}, nil
	}
	return nil, p.errf(t, "陣列常數只能包含數字、文字、邏輯值或錯誤值")
}
