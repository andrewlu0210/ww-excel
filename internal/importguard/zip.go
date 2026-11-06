package importguard

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"ww-excel/internal/formula/parser"
)

// CheckDirectory counts real central-directory records before archive/zip can
// allocate one object per entry. Header counts alone are untrusted.
func CheckDirectory(f *os.File, l Limits) error {
	st, err := f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size < 22 {
		return zip.ErrFormat
	}
	n := min(size, int64(65557))
	tail := make([]byte, n)
	if _, err = f.ReadAt(tail, size-n); err != nil {
		return err
	}
	i := -1
	for at := len(tail) - 22; at >= 0; at-- {
		if bytes.Equal(tail[at:at+4], []byte{'P', 'K', 5, 6}) && at+22+int(binary.LittleEndian.Uint16(tail[at+20:])) == len(tail) {
			i = at
			break
		}
	}
	if i < 0 {
		return zip.ErrFormat
	}
	e := tail[i:]
	if i+22+int(binary.LittleEndian.Uint16(e[20:])) != len(tail) {
		return zip.ErrFormat
	}
	count := uint64(binary.LittleEndian.Uint16(e[10:]))
	off := uint64(binary.LittleEndian.Uint32(e[16:]))
	length := uint64(binary.LittleEndian.Uint32(e[12:]))
	if count == 65535 || off == 0xffffffff || length == 0xffffffff {
		// ZIP64 locator immediately precedes EOCD.
		pos := size - n + int64(i)
		var loc [20]byte
		if pos < 20 {
			return zip.ErrFormat
		}
		if _, err = f.ReadAt(loc[:], pos-20); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(loc[:]) != 0x07064b50 {
			return zip.ErrFormat
		}
		at := binary.LittleEndian.Uint64(loc[8:])
		if at > uint64(size-56) {
			return zip.ErrFormat
		}
		var z [56]byte
		if _, err = f.ReadAt(z[:], int64(at)); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(z[:]) != 0x06064b50 {
			return zip.ErrFormat
		}
		count = binary.LittleEndian.Uint64(z[32:])
		length = binary.LittleEndian.Uint64(z[40:])
		off = binary.LittleEndian.Uint64(z[48:])
	}
	if count > uint64(l.Parts) {
		return Exceeded("ZIP part 數", int64(l.Parts))
	}
	if off > uint64(size) || length > uint64(size)-off {
		return zip.ErrFormat
	}
	end := off + length
	actual := uint64(0)
	for off < end {
		var h [46]byte
		if end-off < 46 {
			return zip.ErrFormat
		}
		if _, err = f.ReadAt(h[:], int64(off)); err != nil {
			return err
		}
		if binary.LittleEndian.Uint32(h[:]) != 0x02014b50 {
			return zip.ErrFormat
		}
		name := uint64(binary.LittleEndian.Uint16(h[28:]))
		extra := uint64(binary.LittleEndian.Uint16(h[30:]))
		comment := uint64(binary.LittleEndian.Uint16(h[32:]))
		if name > 1024 {
			return Exceeded("ZIP part 名稱長度", 1024)
		}
		step := 46 + name + extra + comment
		if step > end-off {
			return zip.ErrFormat
		}
		off += step
		actual++
		if actual > uint64(l.Parts) {
			return Exceeded("ZIP part 數", int64(l.Parts))
		}
	}
	if actual != count {
		return zip.ErrFormat
	}
	return nil
}

// ZIP validates every actual decompressed byte, including custom XML/drawings.
// It operates on a private snapshot so later parsers see precisely these bytes.
func ZIP(ctx context.Context, f *os.File) error {
	l := Policy(ctx)
	if err := CheckDirectory(f, l); err != nil {
		return err
	}
	st, _ := f.Stat()
	z, err := zip.NewReader(f, st.Size())
	if err != nil {
		return err
	}
	var total int64
	counts := xmlCounts{}
	records := 0
	names := map[string]bool{}
	for _, p := range z.File {
		if err = ctx.Err(); err != nil {
			return err
		}
		if names[p.Name] {
			return fmt.Errorf("ZIP part 名稱重複：%s", p.Name)
		}
		names[p.Name] = true
		if p.UncompressedSize64 > uint64(l.Part) {
			return Exceeded("ZIP part 大小", l.Part)
		}
		if p.UncompressedSize64 > uint64(l.Expanded-total) {
			return Exceeded("ZIP 解壓縮總大小", l.Expanded)
		}
		total += int64(p.UncompressedSize64)
		rc, e := p.Open()
		if e != nil {
			return e
		}
		r := Reader(ctx, rc, l.Part, "ZIP part 大小")
		if strings.HasSuffix(strings.ToLower(p.Name), ".xml") || strings.HasSuffix(strings.ToLower(p.Name), ".rels") {
			e = checkXML(r, l, &counts, &records)
		} else {
			_, e = io.Copy(io.Discard, r)
		}
		rc.Close()
		if e != nil {
			return fmt.Errorf("%s: %w", p.Name, e)
		}
	}
	return ctx.Err()
}

// xmlReader bounds a single markup or text token before xml.Decoder allocation.
const (
	xmlText byte = iota
	xmlPrefix
	xmlMarkup
	xmlComment
	xmlCDATA
	xmlPI
)

type xmlReader struct {
	r            io.Reader
	limit, n     int64
	state, quote byte
	prefix       [9]byte
	prefixN      int
	prev, prev2  byte
}

func (r *xmlReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	for _, b := range p[:n] {
		if r.state == xmlText && b == '<' {
			r.state = xmlPrefix
			r.n = 0
			r.prefixN = 0
		}
		r.n++
		if r.n > r.limit {
			return 0, Exceeded("XML token 大小", r.limit)
		}
		switch r.state {
		case xmlPrefix:
			r.prefix[r.prefixN] = b
			r.prefixN++
			if r.prefixN == 2 {
				switch b {
				case '?':
					r.state = xmlPI
				case '!':
				default:
					r.state = xmlMarkup
				}
			} else if r.prefixN > 2 {
				prefix := string(r.prefix[:r.prefixN])
				if prefix == "<!--" {
					r.state = xmlComment
				} else if prefix == "<![CDATA[" {
					r.state = xmlCDATA
				} else if !strings.HasPrefix("<!--", prefix) && !strings.HasPrefix("<![CDATA[", prefix) {
					return 0, fmt.Errorf("unsupported XML directive")
				}
			}
		case xmlMarkup:
			if r.quote != 0 {
				if b == r.quote {
					r.quote = 0
				}
			} else if b == '\'' || b == '"' {
				r.quote = b
			} else if b == '>' {
				r.state = xmlText
				r.n = 0
			}
		case xmlComment:
			if r.prev2 == '-' && r.prev == '-' && b == '>' {
				r.state = xmlText
				r.n = 0
			}
		case xmlCDATA:
			if r.prev2 == ']' && r.prev == ']' && b == '>' {
				r.state = xmlText
				r.n = 0
			}
		case xmlPI:
			if r.prev == '?' && b == '>' {
				r.state = xmlText
				r.n = 0
			}
		}
		r.prev2, r.prev = r.prev, b
	}
	return n, err
}

type xmlCounts struct{ tokens, cells, sheets, shared, objects, styles int }

func checkXML(r io.Reader, l Limits, counts *xmlCounts, records *int) error {
	d := xml.NewDecoder(&xmlReader{r: r, limit: l.String})
	depth := 0
	roots := 0
	cellDepth := 0
	chars := 0
	formulaDepth := 0
	var formula strings.Builder
	for {
		t, err := d.Token()
		if err == io.EOF {
			if depth != 0 || roots != 1 {
				return io.ErrUnexpectedEOF
			}
			return nil
		}
		if err != nil {
			return err
		}
		counts.tokens++
		if counts.tokens > l.Tokens {
			return Exceeded("XML token 數", int64(l.Tokens))
		}
		switch t := t.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
			}
			*records++
			if *records > l.Records {
				return Exceeded("XML 記錄數", int64(l.Records))
			}
			depth++
			if strings.Contains(t.Name.Space, "drawingml") {
				switch t.Name.Local {
				case "sp", "pic", "graphicFrame", "grpSp", "cxnSp":
					counts.objects++
					if counts.objects > 2000 {
						return Exceeded("圖形數", 2000)
					}
				}
			}
			if strings.Contains(t.Name.Space, "spreadsheetml") {
				if t.Name.Local == "si" {
					counts.shared++
					if counts.shared > l.Cells {
						return Exceeded("共用字串數", int64(l.Cells))
					}
				}
				if t.Name.Local == "xf" {
					counts.styles++
					if counts.styles > 65536 {
						return Exceeded("樣式數", 65536)
					}
				}
				if t.Name.Local == "c" || t.Name.Local == "si" {
					cellDepth = depth
					chars = 0
				}
				if t.Name.Local == "f" || t.Name.Local == "definedName" || t.Name.Local == "formula" || t.Name.Local == "formula1" || t.Name.Local == "formula2" {
					formulaDepth = depth
					formula.Reset()
				}
			}
			if depth > l.Depth {
				return Exceeded("XML 巢狀深度", int64(l.Depth))
			}
			if t.Name.Local == "c" && strings.Contains(t.Name.Space, "spreadsheetml") {
				counts.cells++
				if counts.cells > l.Cells {
					return Exceeded("儲存格數", int64(l.Cells))
				}
			}
			if t.Name.Local == "sheet" && strings.Contains(t.Name.Space, "spreadsheetml") {
				counts.sheets++
				if counts.sheets > l.Sheets {
					return Exceeded("工作表數", int64(l.Sheets))
				}
			}
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) > 0 {
				return fmt.Errorf("invalid XML text outside root")
			}
			if cellDepth > 0 {
				chars += utf8.RuneCount(t)
				if chars > 65536 {
					return Exceeded("儲存格 XML 文字大小", 65536)
				}
			}
			if formulaDepth > 0 {
				if formula.Len()+len(t) > 64<<10 {
					return Exceeded("公式長度", 64<<10)
				}
				formula.Write(t)
			}
		case xml.EndElement:
			if formulaDepth == depth {
				if err := parser.CheckResources(formula.String()); err != nil {
					return err
				}
				formulaDepth = 0
			}
			if cellDepth == depth {
				cellDepth = 0
			}
			depth--
		}
	}
}
