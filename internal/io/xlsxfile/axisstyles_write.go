package xlsxfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"ww-excel/internal/model"
)

type axisStylePlan struct {
	columns map[int]int
	rows    map[int]*int
}

var axisAttributes = map[string]*regexp.Regexp{
	"min":          regexp.MustCompile(`\s+min\s*=\s*(?:"[^"]*"|'[^']*')`),
	"max":          regexp.MustCompile(`\s+max\s*=\s*(?:"[^"]*"|'[^']*')`),
	"style":        regexp.MustCompile(`\s+style\s*=\s*(?:"[^"]*"|'[^']*')`),
	"s":            regexp.MustCompile(`\s+s\s*=\s*(?:"[^"]*"|'[^']*')`),
	"customFormat": regexp.MustCompile(`\s+customFormat\s*=\s*(?:"[^"]*"|'[^']*')`),
}

func axisAttribute(raw, name string, value *int) string {
	end := strings.Index(raw, ">")
	if end < 0 {
		return raw
	}
	tag, rest := raw[:end+1], raw[end+1:]
	attr := ""
	if value != nil {
		attr = ` ` + name + `="` + strconv.Itoa(*value) + `"`
	}
	re := axisAttributes[name]
	if re.MatchString(tag) {
		return re.ReplaceAllString(tag, attr) + rest
	}
	if value == nil {
		return raw
	}
	pos := len(tag) - 1
	if pos > 0 && tag[pos-1] == '/' {
		pos--
	}
	return tag[:pos] + attr + tag[pos:] + rest
}
func axisInt(n int) *int { return &n }

// Preserve column widths and extension attributes while splitting only style
// boundaries. A column range never creates individual worksheet cells.
func patchAxisColumns(raw []byte, styles map[int]int) (string, error) {
	type col struct {
		raw     string
		style   int
		changed bool
	}
	cols := make([]col, model.MaxCols)
	if len(raw) > 0 {
		root, err := drawingNodes(raw)
		if err != nil {
			return "", err
		}
		for _, node := range root.children[0].children {
			if node.name != "col" {
				continue
			}
			first, last := 0, 0
			for _, a := range node.attrs {
				if a.Name.Local == "min" {
					first = atoi(a.Value) - 1
				}
				if a.Name.Local == "max" {
					last = atoi(a.Value) - 1
				}
			}
			for i := max(0, first); i <= min(last, model.MaxCols-1); i++ {
				cols[i].raw = node.raw(raw)
			}
		}
	}
	for i, style := range styles {
		if i >= 0 && i < len(cols) {
			cols[i].style = style
			cols[i].changed = true
		}
	}
	var result strings.Builder
	for i := 0; i < len(cols); {
		current := cols[i]
		last := i
		for last+1 < len(cols) && cols[last+1] == current {
			last++
		}
		if current.raw != "" || current.changed && current.style != 0 {
			tag := current.raw
			if tag == "" {
				tag = "<col/>"
			}
			tag = axisAttribute(tag, "min", axisInt(i+1))
			tag = axisAttribute(tag, "max", axisInt(last+1))
			if current.changed {
				tag = axisAttribute(tag, "style", &current.style)
			}
			result.WriteString(tag)
		}
		i = last + 1
	}
	if result.Len() == 0 {
		return "", nil
	}
	return "<cols>" + result.String() + "</cols>", nil
}
func emptyAxisRow(raw string, attrs []xml.Attr) bool {
	if attrs == nil {
		token, err := xml.NewDecoder(strings.NewReader(raw)).Token()
		if err != nil {
			return false
		}
		if start, ok := token.(xml.StartElement); ok {
			attrs = start.Attr
		} else {
			return false
		}
	}
	for _, a := range attrs {
		if a.Name.Local != "r" {
			return false
		}
	}
	open := strings.Index(raw, ">")
	if open < 0 {
		return false
	}
	if open > 0 && raw[open-1] == '/' {
		return true
	}
	close := strings.LastIndex(raw, "</")
	return close >= open && strings.TrimSpace(raw[open+1:close]) == ""
}

// Patch raw axis metadata after Excelize serializes the sheet. Its normal
// SetColStyle applies to every coordinate up to the last row and can expand a
// sparse whole-column edit into millions of blank cells. Also drop placeholder
// rows created by distant explicit cells, retaining all meaningful row metadata.
func patchAxisStyles(data []byte, plan axisStylePlan) ([]byte, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var out bytes.Buffer
	cursor := 0
	inData := false
	seenCols := false
	rowKeys := make([]int, 0, len(plan.rows))
	for row, style := range plan.rows {
		if style != nil {
			rowKeys = append(rowKeys, row)
		}
	}
	sort.Ints(rowKeys)
	rowIndex := 0
	insertRows := func(before int) {
		for rowIndex < len(rowKeys) && rowKeys[rowIndex] < before {
			row := rowKeys[rowIndex]
			out.WriteString(`<row r="` + strconv.Itoa(row+1) + `" s="` + strconv.Itoa(*plan.rows[row]) + `" customFormat="1"/>`)
			rowIndex++
		}
	}
	for {
		start := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "cols" {
				if err = d.Skip(); err != nil {
					return nil, err
				}
				end := int(d.InputOffset())
				seenCols = true
				if len(plan.columns) > 0 {
					patched, e := patchAxisColumns(data[start:end], plan.columns)
					if e != nil {
						return nil, e
					}
					out.Write(data[cursor:start])
					out.WriteString(patched)
					cursor = end
				}
			} else if t.Name.Local == "sheetData" {
				if !seenCols && len(plan.columns) > 0 {
					cols, e := patchAxisColumns(nil, plan.columns)
					if e != nil {
						return nil, e
					}
					out.Write(data[cursor:start])
					out.WriteString(cols)
					cursor = start
				}
				inData = true
				openEnd := int(d.InputOffset())
				raw := string(data[start:openEnd])
				if strings.HasSuffix(strings.TrimSpace(raw), "/>") && len(rowKeys) > 0 {
					out.Write(data[cursor:start])
					at := strings.LastIndex(raw, "/>")
					out.WriteString(raw[:at] + ">")
					insertRows(model.MaxRows)
					out.WriteString("</sheetData>")
					cursor = openEnd
					if err = d.Skip(); err != nil {
						return nil, err
					}
					inData = false
				}
			} else if inData && t.Name.Local == "row" {
				row := -1
				for _, a := range t.Attr {
					if a.Name.Local == "r" {
						row = atoi(a.Value) - 1
					}
				}
				if err = d.Skip(); err != nil {
					return nil, err
				}
				end := int(d.InputOffset())
				raw := string(data[start:end])
				out.Write(data[cursor:start])
				insertRows(row)
				if style, changed := plan.rows[row]; changed {
					raw = axisAttribute(raw, "s", style)
					if style != nil {
						raw = axisAttribute(raw, "customFormat", axisInt(1))
					} else {
						raw = axisAttribute(raw, "customFormat", nil)
					}
					// The edited tag now has meaningful format attributes unless removed.
					if style != nil || !emptyAxisRow(raw, nil) {
						out.WriteString(raw)
					}
				} else if !emptyAxisRow(raw, t.Attr) {
					out.WriteString(raw)
				}
				if rowIndex < len(rowKeys) && rowKeys[rowIndex] == row {
					rowIndex++
				}
				cursor = end
			}
		case xml.EndElement:
			if t.Name.Local == "sheetData" && inData {
				out.Write(data[cursor:start])
				insertRows(model.MaxRows)
				cursor = start
				inData = false
			}
		}
	}
	out.Write(data[cursor:])
	return out.Bytes(), nil
}

func (w *writer) writeAxisStyles(filename string) error {
	if len(w.axisStyles) == 0 {
		return nil
	}
	source, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer source.Close()
	parts := map[string]*zip.File{}
	for _, p := range source.File {
		parts[p.Name] = p
	}
	sheets, err := sheetParts(parts)
	if err != nil {
		return err
	}
	changed := map[string][]byte{}
	for sh, plan := range w.axisStyles {
		part := sheets[sh.Name]
		data, e := patchAxisStyles(readPart(parts, part), plan)
		if e != nil {
			return e
		}
		changed[part] = data
	}
	temp, err := os.CreateTemp(filepath.Dir(filename), ".axis-styles-*.xlsx")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	out := zip.NewWriter(temp)
	for _, entry := range source.File {
		if data, ok := changed[entry.Name]; ok {
			target, e := out.CreateHeader(&entry.FileHeader)
			if e != nil {
				return e
			}
			if _, e = target.Write(data); e != nil {
				return e
			}
		} else if e := out.Copy(entry); e != nil {
			return e
		}
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp.Name(), filename); err != nil {
		return err
	}
	for part, data := range changed {
		w.f.Pkg.Store(part, data)
		w.f.Sheet.Delete(part)
	}
	return nil
}
