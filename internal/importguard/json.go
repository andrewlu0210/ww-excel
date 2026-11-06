package importguard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// stringReader rejects oversized JSON strings before encoding/json allocates
// them. Binary payloads use a separate bound; all strings share the byte budget.
type stringReader struct {
	r           io.Reader
	limits      Limits
	in, escaped bool
	size        int64
	scalar      int64
	key         []byte
	data        bool
	last        string
}

func (r *stringReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	for _, b := range p[:n] {
		if !r.in {
			switch b {
			case ' ', '\t', '\r', '\n', ',', ':', '{', '}', '[', ']', '"':
				r.scalar = 0
			default:
				r.scalar++
				if r.scalar > 256 {
					return 0, Exceeded("JSON scalar 大小", 256)
				}
			}
			if b == '"' {
				r.in = true
				r.size = 0
				r.key = r.key[:0]
				r.data = r.last == "Data" || r.last == "data" || r.last == "RawXML" || r.last == "rawXML" || r.last == "sourceConfig" || r.last == "SourceConfig" || r.last == "XML" || r.last == "xml"
				r.last = ""
			}
			if b == ',' || b == '{' || b == '[' {
				r.last = ""
			}
			continue
		}
		if !r.escaped && b == '"' {
			r.in = false
			r.last = string(r.key)
			continue
		}
		r.size++
		limit := r.limits.String
		if r.data {
			limit = r.limits.Data * 2
		} // Base64 and JSON escapes.
		if r.size > limit {
			return 0, Exceeded("JSON 字串大小", limit)
		}
		if len(r.key) < 256 {
			r.key = append(r.key, b)
		}
		if r.escaped {
			r.escaped = false
		} else if b == '\\' {
			r.escaped = true
		}
	}
	return n, err
}

// JSON validates structure and record budgets while spooling decompressed data
// to disk. The second decode can only allocate from this validated snapshot.
func JSON(ctx context.Context, r io.Reader) (*os.File, error) {
	l := Policy(ctx)
	f, err := os.CreateTemp("", "ww-import-json-*")
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	safe := &stringReader{r: Reader(ctx, r, l.Expanded, "解壓縮資料大小"), limits: l}
	dec := json.NewDecoder(io.TeeReader(safe, f))
	depth, records, tokens, roots := 0, 0, 0, 0
	type frame struct {
		kind     json.Delim
		key, tag string
		wantKey  bool
		count    int
	}
	var stack []frame
	cells := 0
	value := func() error {
		if len(stack) == 0 {
			return nil
		}
		top := &stack[len(stack)-1]
		if top.kind == '{' {
			top.wantKey = true
			return nil
		}
		top.count++
		limit := l.Records
		switch top.tag {
		case "cells":
			cells++
			if cells > l.Cells {
				return Exceeded("儲存格數", int64(l.Cells))
			}
		case "sheets":
			limit = l.Sheets
		case "styles":
			limit = 65536
		case "names":
			limit = 65536
		}
		if top.count > limit {
			return Exceeded(fmt.Sprintf("JSON 陣列記錄數（%s）", top.tag), int64(limit))
		}
		return nil
	}
	for {
		t, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("WWX JSON: %w", e)
		}
		tokens++
		if tokens > l.Tokens {
			return nil, Exceeded("JSON token 數", int64(l.Tokens))
		}
		if depth == 0 {
			roots++
		}

		if d, yes := t.(json.Delim); yes {
			if d == '{' || d == '[' {
				tag := ""
				if len(stack) > 0 {
					top := stack[len(stack)-1]
					if top.kind == '{' {
						tag = top.key
					}
				}
				if e = value(); e != nil {
					return nil, e
				}
				stack = append(stack, frame{kind: d, tag: tag, wantKey: d == '{'})
			} else {
				stack = stack[:len(stack)-1]
			}
		} else if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.kind == '{' && top.wantKey {
				top.key, _ = t.(string)
				top.wantKey = false
			} else if e = value(); e != nil {
				return nil, e
			}
		}
		if d, yes := t.(json.Delim); yes {
			switch d {
			case '{', '[':
				depth++
				records++
			case '}', ']':
				depth--
			}
			if depth > l.Depth {
				return nil, Exceeded("JSON 巢狀深度", int64(l.Depth))
			}
			if records > l.Records {
				return nil, Exceeded("JSON 記錄數", int64(l.Records))
			}
		}
	}
	if depth != 0 || roots != 1 {
		return nil, fmt.Errorf("WWX JSON 必須包含一個完整文件")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	ok = true
	return f, nil
}
