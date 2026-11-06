// Package importguard bounds untrusted workbook input before model allocation.
package importguard

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Limits are per import. Tests can use smaller budgets without global state.
type Limits struct {
	Compressed, Expanded, Part, String, Data     int64
	Records, Tokens, Cells, Sheets, Parts, Depth int
}

func DefaultLimits() Limits {
	return Limits{Compressed: 256 << 20, Expanded: 512 << 20, Part: 128 << 20, String: 512 << 10, Data: 64 << 20,
		Records: 4_000_000, Tokens: 64_000_000, Cells: 2_000_000, Sheets: 256, Parts: 10_000, Depth: 128}
}

type limitsKey struct{}

func WithLimits(ctx context.Context, l Limits) context.Context {
	return context.WithValue(ctx, limitsKey{}, l)
}
func Policy(ctx context.Context) Limits {
	if l, ok := ctx.Value(limitsKey{}).(Limits); ok {
		return l
	}
	return DefaultLimits()
}

type LimitError struct {
	Resource string
	Limit    int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("匯入超過安全限制：%s（上限 %d）", e.Resource, e.Limit)
}
func Exceeded(resource string, limit int64) error { return &LimitError{resource, limit} }

type reader struct {
	ctx         context.Context
	r           io.Reader
	left, limit int64
	resource    string
	err         error
}

// Reader checks cancellation at every read and verifies the byte limit at EOF.
func Reader(ctx context.Context, r io.Reader, limit int64, resource string) io.Reader {
	return &reader{ctx: ctx, r: r, left: limit, limit: limit, resource: resource}
}
func (r *reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > r.left+1 {
		p = p[:r.left+1]
	}
	n, err := r.r.Read(p)
	r.left -= int64(n)
	if r.left < 0 {
		r.err = Exceeded(r.resource, r.limit)
		return 0, r.err
	}
	return n, err
}

// TempCopy creates a private snapshot; callers must remove it after use.
func TempCopy(ctx context.Context, r io.Reader, limit int64, resource string) (*os.File, error) {
	f, err := os.CreateTemp("", "ww-import-*")
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(f, Reader(ctx, r, limit, resource)); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
