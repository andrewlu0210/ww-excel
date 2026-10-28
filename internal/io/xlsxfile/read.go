package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"ww-excel/internal/formula/ast"
	"ww-excel/internal/formula/parser"
	"ww-excel/internal/model"
	"ww-excel/internal/numfmt"
)

// Read loads an xlsx file. Worksheets are read with one streaming pass
// over their XML; excelize is only used for workbook-level parts
// (styles, defined names, comments). The returned excelize file is kept
// as the base for in-place saves: it parses a worksheet only when a
// save writes to it, so opening a large file stays fast and small.
func Read(path string) (*model.Workbook, *excelize.File, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, nil, err
	}
	wb, err := load(f, path)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return wb, f, nil
}

type reader struct {
	f       *excelize.File
	parts   map[string]*zip.File
	wb      *model.Workbook
	styles  map[int]uint32
	sst     []string
	richSST map[int]*model.RichText
}

func load(f *excelize.File, file string) (*model.Workbook, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	parts := map[string]*zip.File{}
	for _, p := range z.File {
		parts[p.Name] = p
	}
	def, _ := f.GetStyle(0)
	defStyle := fromExcelize(f, def)
	if defStyle.Font.Name == "" {
		defStyle.Font = model.DefaultFont
	}
	r := &reader{f: f, parts: parts, wb: &model.Workbook{Styles: model.NewStyleTable(defStyle)}, styles: map[int]uint32{0: 0}}
	if r.sst, r.richSST, err = readSharedStrings(parts, f); err != nil {
		return nil, err
	}
	sheets, err := sheetList(parts)
	if err != nil {
		return nil, err
	}
	names := f.GetDefinedName()
	for _, s := range sheets {
		sh, err := r.sheet(s.name, s.part, names)
		if err != nil {
			return nil, fmt.Errorf("工作表「%s」：%w", s.name, err)
		}
		r.wb.Sheets = append(r.wb.Sheets, sh)
	}
	if len(r.wb.Sheets) == 0 {
		return nil, fmt.Errorf("活頁簿沒有工作表")
	}
	for _, dn := range names {
		if strings.HasPrefix(dn.Name, "_xlnm.") || !model.ValidName(dn.Name) {
			continue
		}
		n := &model.DefinedName{Name: dn.Name, Formula: strings.TrimPrefix(dn.RefersTo, "="), Comment: dn.Comment}
		if dn.Scope != "" && dn.Scope != "Workbook" {
			i := r.wb.SheetIndex(dn.Scope)
			if i < 0 {
				continue
			}
			n.Scope = r.wb.Sheets[i]
		}
		r.wb.Names = append(r.wb.Names, n)
	}
	return r.wb, nil
}

func (r *reader) style(id int) uint32 {
	if s, ok := r.styles[id]; ok {
		return s
	}
	xs, err := r.f.GetStyle(id)
	var st model.Style
	if err == nil {
		st = fromExcelize(r.f, xs)
	}
	def := r.wb.Styles.Default()
	if st.Font.Name == "" {
		st.Font.Name = def.Font.Name
	}
	if st.Font.Size == 0 {
		st.Font.Size = def.Font.Size
	}
	s := r.wb.Styles.InternExplicit(st)
	r.styles[id] = s
	return s
}

type sheetRef struct{ name, part string }

// sheetList returns the sheets in workbook order with their parts.
func sheetList(parts map[string]*zip.File) ([]sheetRef, error) {
	var wbDoc struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := decodePart(parts, "xl/workbook.xml", &wbDoc); err != nil {
		return nil, err
	}
	var out []sheetRef
	for _, s := range wbDoc.Sheets {
		if part := relTarget(parts, "xl/workbook.xml", s.RID); part != "" {
			out = append(out, sheetRef{s.Name, part})
		}
	}
	return out, nil
}

// sheetParts maps sheet names to their worksheet parts.
func sheetParts(parts map[string]*zip.File) (map[string]string, error) {
	list, err := sheetList(parts)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, s := range list {
		out[s.name] = s.part
	}
	return out, nil
}

func decodePart(parts map[string]*zip.File, name string, v any) error {
	f, ok := parts[name]
	if !ok {
		return nil
	}
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	return xml.NewDecoder(r).Decode(v)
}

// readSharedStrings keeps rich runs separately so plain strings stay compact.
func readSharedStrings(parts map[string]*zip.File, f *excelize.File) ([]string, map[int]*model.RichText, error) {
	name := "xl/sharedStrings.xml"
	if _, ok := parts[name]; !ok {
		if t := relOfType(parts, "xl/workbook.xml", "/sharedStrings"); t != "" {
			name = t
		}
	}
	p, ok := parts[name]
	if !ok {
		return nil, nil, nil
	}
	rc, err := p.Open()
	if err != nil {
		return nil, nil, err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var out []string
	rich := map[int]*model.RichText{}
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			return out, rich, nil
		}
		if err != nil {
			return nil, nil, err
		}
		if t, ok := tok.(xml.StartElement); ok && t.Name.Local == "si" {
			value, runs, err := readTextElement(dec, "si", f)
			if err != nil {
				return nil, nil, err
			}
			if runs != nil {
				rich[len(out)] = runs
			}
			out = append(out, value)
		}
	}
}

// relOfType finds a relationship target of part by type suffix.
func relOfType(parts map[string]*zip.File, part, typeSuffix string) string {
	dir, base := path.Split(part)
	var rels struct {
		Rels []struct {
			Type   string `xml:"Type,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if decodePart(parts, dir+"_rels/"+base+".rels", &rels) != nil {
		return ""
	}
	for _, r := range rels.Rels {
		if strings.HasSuffix(r.Type, typeSuffix) {
			if strings.HasPrefix(r.Target, "/") {
				return strings.TrimPrefix(r.Target, "/")
			}
			return path.Join(dir, r.Target)
		}
	}
	return ""
}

// sheetState collects what one pass over a worksheet finds besides cells.
type sheetState struct {
	merges     []model.Range
	drawingRID string
	tableRIDs  []string
	validation *model.Validation // being read
	dvs        []model.Validation
	cfRanges   []model.Range // current conditionalFormatting block
	cf         *rawRule      // rule being read
	cfs        []rawRule
	ps         model.PageSetup
	psFound    bool
	fitToPage  bool
	frozen     bool
	xSplit     int
	ySplit     int
	header     string
	footer     string
}

type rawRule struct {
	rule     model.CFRule
	priority int
	dxf      int
	formulas []string
	colors   []string
}

// shared is a shared formula's master: its position and parsed text.
type shared struct {
	row, col int
	node     ast.Node
}

func (r *reader) sheet(name, part string, names []excelize.DefinedName) (*model.Sheet, error) {
	sh := model.NewSheet(name)
	sh.SourceName = name
	l := &sh.Layout
	p, ok := r.parts[part]
	if !ok {
		return sh, nil
	}
	rc, err := p.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)

	st := &sheetState{ps: model.DefaultPageSetup()}
	sharedF := map[string]shared{}
	var text strings.Builder
	var inlineRich *model.RichText // character data of the current leaf element
	collect := false
	row, col := 0, -1
	// The cell being read.
	var cType, fType, fRef, fSI string
	var cStyle int
	var hasV, hasF, inIS bool
	var v, ftext string
	inExt := 0 // depth inside extLst (x14 data validations)
	inHeaderFooter := false

	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "sheetFormatPr":
				if v := attr(t, "defaultRowHeight"); v != "" {
					if h, err := strconv.ParseFloat(v, 64); err == nil && h > 0 {
						l.DefaultRow = ptToPx(h)
					}
				}
				if v := attr(t, "defaultColWidth"); v != "" {
					if w, err := strconv.ParseFloat(v, 64); err == nil && w > 0 {
						l.DefaultCol = widthToPx(w)
					}
				} else if v := attr(t, "baseColWidth"); v != "" {
					// Excel's default width is the base width plus padding.
					if w, err := strconv.ParseFloat(v, 64); err == nil && w > 0 {
						l.DefaultCol = widthToPx(w + 0.71)
					}
				}
			case "pane":
				st.frozen = strings.HasPrefix(attr(t, "state"), "frozen")
				st.xSplit = atoiFloat(attr(t, "xSplit"))
				st.ySplit = atoiFloat(attr(t, "ySplit"))
			case "col":
				lo, hi := atoi(attr(t, "min"))-1, atoi(attr(t, "max"))-1
				if lo < 0 || hi < lo {
					continue
				}
				hi = min(hi, model.MaxCols-1)
				width, _ := strconv.ParseFloat(attr(t, "width"), 64)
				hidden := attr(t, "hidden") == "1" || attr(t, "hidden") == "true"
				styleID := atoi(attr(t, "style"))
				custom := width > 0 && widthToPx(width) != l.DefaultCol
				for c := lo; c <= hi; c++ {
					if custom || hidden {
						px := l.DefaultCol
						if width > 0 {
							px = widthToPx(width)
						}
						l.Cols[c] = model.Size{Px: px, Hidden: hidden}
					}
					if styleID > 0 {
						l.ColStyles[c] = r.style(styleID)
					}
				}
			case "row":
				if v := attr(t, "r"); v != "" {
					row = atoi(v) - 1
				} else {
					row++
				}
				col = -1
				if row < 0 || row >= model.MaxRows {
					continue
				}
				ht, _ := strconv.ParseFloat(attr(t, "ht"), 64)
				hidden := attr(t, "hidden") == "1" || attr(t, "hidden") == "true"
				// Auto-sized rows may store a height without customHeight.
				if hidden || ht > 0 && ptToPx(ht) != l.DefaultRow {
					px := l.DefaultRow
					if ht > 0 {
						px = ptToPx(ht)
					}
					l.Rows[row] = model.Size{Px: px, Hidden: hidden}
				}
				if s := atoi(attr(t, "s")); s > 0 && (attr(t, "customFormat") == "1" || attr(t, "customFormat") == "true") {
					l.RowStyles[row] = r.style(s)
				}
			case "c":
				if ref := attr(t, "r"); ref != "" {
					if c, rr, err := excelize.CellNameToCoordinates(ref); err == nil {
						col, row = c-1, rr-1
					}
				} else {
					col++
				}
				cType, cStyle = attr(t, "t"), atoi(attr(t, "s"))
				inlineRich = nil
				hasV, hasF, inIS = false, false, false
				v, ftext, fType, fRef, fSI = "", "", "", "", ""
			case "v":
				text.Reset()
				collect = true
			case "f":
				if inExt > 0 {
					text.Reset()
					collect = true
					continue
				}
				hasF = true
				fType, fRef, fSI = attr(t, "t"), attr(t, "ref"), attr(t, "si")
				text.Reset()
				collect = true
			case "is":
				var textErr error
				v, inlineRich, textErr = readTextElement(dec, "is", r.f)
				if textErr != nil {
					return nil, textErr
				}
				hasV = true
			case "t":
				if inIS {
					collect = true
				}
			case "rPh":
				collect = false
			case "mergeCell":
				if rs := parseSqref(attr(t, "ref")); len(rs) == 1 {
					st.merges = append(st.merges, rs[0])
				}
			case "dataValidation":
				st.validation = &model.Validation{
					Ranges:      parseSqref(attr(t, "sqref")),
					Type:        attr(t, "type"),
					Operator:    attr(t, "operator"),
					AllowBlank:  attr(t, "allowBlank") == "1",
					NoDropdown:  attr(t, "showDropDown") == "1", // OOXML: means hide it
					ErrorStyle:  attr(t, "errorStyle"),
					ErrorTitle:  attr(t, "errorTitle"),
					Error:       attr(t, "error"),
					PromptTitle: attr(t, "promptTitle"),
					Prompt:      attr(t, "prompt"),
				}
			case "formula1", "formula2", "formula", "sqref":
				text.Reset()
				collect = true
			case "conditionalFormatting":
				st.cfRanges = parseSqref(attr(t, "sqref"))
			case "cfRule":
				if inExt > 0 {
					continue
				}
				st.cf = &rawRule{priority: atoi(attr(t, "priority")), dxf: -1}
				if d := attr(t, "dxfId"); d != "" {
					st.cf.dxf = atoi(d)
				}
				st.cf.rule = cfRuleFromAttrs(t)
				st.cf.rule.Ranges = st.cfRanges
			case "color":
				if st.cf != nil {
					st.cf.colors = append(st.cf.colors, rgb(attr(t, "rgb")))
				}
			case "pageMargins":
				st.psFound = true
				set := func(dst *float64, name string) {
					if f, err := strconv.ParseFloat(attr(t, name), 64); err == nil {
						*dst = f
					}
				}
				set(&st.ps.Left, "left")
				set(&st.ps.Right, "right")
				set(&st.ps.Top, "top")
				set(&st.ps.Bottom, "bottom")
				set(&st.ps.Header, "header")
				set(&st.ps.Footer, "footer")
			case "pageSetup":
				if size := atoi(attr(t, "paperSize")); size > 0 {
					for name, code := range paperCodes {
						if code == size {
							st.ps.Paper, st.psFound = name, true
						}
					}
				}
				if attr(t, "orientation") == "landscape" {
					st.ps.Landscape, st.psFound = true, true
				}
				if s := atoi(attr(t, "scale")); s >= 10 && s != 100 {
					st.ps.Scale, st.psFound = s, true
				}
				st.ps.FitWide = atoiDefault(attr(t, "fitToWidth"), 1)
				st.ps.FitTall = atoiDefault(attr(t, "fitToHeight"), 1)
			case "pageSetUpPr":
				st.fitToPage = attr(t, "fitToPage") == "1" || attr(t, "fitToPage") == "true"
			case "printOptions":
				st.ps.Gridlines = attr(t, "gridLines") == "1" || attr(t, "gridLines") == "true"
				st.ps.Headings = attr(t, "headings") == "1" || attr(t, "headings") == "true"
				st.ps.CenterH = attr(t, "horizontalCentered") == "1" || attr(t, "horizontalCentered") == "true"
				st.ps.CenterV = attr(t, "verticalCentered") == "1" || attr(t, "verticalCentered") == "true"
				st.psFound = st.psFound || st.ps.Gridlines || st.ps.Headings
			case "headerFooter":
				inHeaderFooter = true
			case "oddHeader", "oddFooter":
				if inHeaderFooter {
					text.Reset()
					collect = true
				}
			case "drawing":
				st.drawingRID = relID(t)
			case "tablePart":
				st.tableRIDs = append(st.tableRIDs, relID(t))
			case "extLst":
				inExt++
			}
		case xml.CharData:
			if collect {
				text.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				v, hasV, collect = text.String(), true, false
			case "f":
				collect = false
				if inExt == 0 {
					ftext = text.String()
				} else if st.validation != nil && st.validation.Formula1 == "" {
					st.validation.Formula1 = text.String()
				}
			case "t":
				if inIS {
					collect = false
				}
			case "is":
				v, hasV, inIS = text.String(), true, false
			case "c":
				if row < 0 || row >= model.MaxRows || col < 0 || col >= model.MaxCols {
					continue
				}
				formula := ""
				if hasF {
					formula = r.formulaText(ftext, fType, fRef, fSI, row, col, sharedF)
				}
				cell := r.cellValue(cType, v, hasV)
				if inlineRich != nil {
					cell.Rich = inlineRich
				}
				if formula != "" {
					cell.F = &model.Formula{Text: formula}
				}
				if cStyle > 0 {
					cell.S = r.style(cStyle)
				}
				if cell.HasContent() || cell.S != 0 {
					sh.Set(row, col, cell)
				}
			case "formula1":
				collect = false
				if st.validation != nil {
					st.validation.Formula1 = text.String()
				}
			case "formula2":
				collect = false
				if st.validation != nil {
					st.validation.Formula2 = text.String()
				}
			case "sqref":
				collect = false
				if st.validation != nil && len(st.validation.Ranges) == 0 {
					st.validation.Ranges = parseSqref(text.String())
				}
			case "formula":
				collect = false
				if st.cf != nil {
					st.cf.formulas = append(st.cf.formulas, text.String())
				}
			case "dataValidation":
				if v := st.validation; v != nil && v.Type != "" && v.Type != "none" && len(v.Ranges) > 0 {
					v.Formula1 = strings.TrimPrefix(v.Formula1, "=")
					v.Formula2 = strings.TrimPrefix(v.Formula2, "=")
					st.dvs = append(st.dvs, *v)
				}
				st.validation = nil
			case "cfRule":
				if st.cf != nil {
					st.cfs = append(st.cfs, *st.cf)
					st.cf = nil
				}
			case "oddHeader":
				collect = false
				st.header = text.String()
			case "oddFooter":
				collect = false
				st.footer = text.String()
			case "headerFooter":
				inHeaderFooter = false
			case "extLst":
				inExt--
			}
		}
	}

	l.Merges = st.merges
	l.SortMerges()
	if st.frozen {
		l.FreezeRows, l.FreezeCols = max(st.ySplit, 0), max(st.xSplit, 0)
	}
	l.Validations = st.dvs
	l.CondFormats = r.condFormats(st.cfs)
	r.readNotes(sh, name)
	for _, dn := range names {
		if dn.Scope != name {
			continue
		}
		switch dn.Name {
		case "_xlnm._FilterDatabase":
			if rs := parseSqref(refPart(dn.RefersTo)); len(rs) == 1 {
				l.Filter = &model.AutoFilter{Range: rs[0], Cols: map[int][]string{}}
			}
		case "_xlnm.Print_Area":
			if rs := parseSqref(strings.Split(refPart(dn.RefersTo), ",")[0]); len(rs) == 1 {
				st.ps.PrintArea, st.psFound = &rs[0], true
			}
		}
	}
	if st.header != "" || st.footer != "" {
		st.ps.HeaderText, st.ps.FooterText, st.psFound = centerSection(st.header), centerSection(st.footer), true
	}
	if st.fitToPage {
		st.psFound = true
	} else {
		st.ps.FitWide, st.ps.FitTall = 0, 0
	}
	if st.psFound {
		ps := st.ps
		l.PageSetup = &ps
	}
	for _, rid := range st.tableRIDs {
		if t, ok := readTable(r.parts, relTarget(r.parts, part, rid)); ok {
			l.Tables = append(l.Tables, t)
		}
	}
	if st.drawingRID != "" {
		readDrawing(r.parts, relTarget(r.parts, part, st.drawingRID), sh)
	}
	return sh, nil
}

func relID(el xml.StartElement) string {
	for _, a := range el.Attr {
		if a.Name.Local == "id" {
			return a.Value
		}
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}

// atoiFloat reads "3" or "3.0" (pane splits are written as doubles).
func atoiFloat(s string) int {
	f, _ := strconv.ParseFloat(s, 64)
	return int(f)
}

func refPart(refersTo string) string {
	ref := strings.TrimPrefix(refersTo, "=")
	if i := strings.LastIndex(ref, "!"); i >= 0 {
		ref = ref[i+1:]
	}
	return ref
}

// formulaText returns a cell's formula, expanding shared formulas.
// Text with file-only prefixes (_xlfn.) is rewritten in canonical form.
func (r *reader) formulaText(text, typ, ref, si string, row, col int, sharedF map[string]shared) string {
	if typ == "shared" && si != "" {
		if text != "" {
			node, err := parser.Parse(text)
			if err == nil {
				sharedF[si] = shared{row: row, col: col, node: node}
			}
		} else if m, ok := sharedF[si]; ok {
			return ast.Format(ast.Translate(m.node, row-m.row, col-m.col))
		}
	}
	if strings.Contains(text, "_xl") {
		if node, err := parser.Parse(text); err == nil {
			return ast.Format(node)
		}
	}
	return text
}

func (r *reader) cellValue(typ, v string, hasV bool) model.Cell {
	if !hasV {
		return model.Cell{}
	}
	switch typ {
	case "s":
		i, err := strconv.Atoi(v)
		if err != nil || i < 0 || i >= len(r.sst) {
			return model.Cell{}
		}
		cell := model.StringCell(r.sst[i])
		cell.Rich = r.richSST[i]
		return cell
	case "str", "inlineStr":
		return model.StringCell(v)
	case "b":
		return model.BoolCell(v == "1" || strings.EqualFold(v, "TRUE"))
	case "e":
		if code := model.ErrorCode(v); code != "" {
			return model.ErrorCell(code)
		}
		return model.StringCell(v)
	case "d":
		if t, err := time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(v, "Z")); err == nil {
			return model.NumberCell(numfmt.TimeSerial(t))
		}
		return model.StringCell(v)
	}
	if v == "" {
		return model.Cell{}
	}
	if n, err := strconv.ParseFloat(v, 64); err == nil {
		return model.NumberCell(n)
	}
	return model.StringCell(v)
}

// readNotes loads cell notes (excelize reads the comments part without
// parsing the worksheet).
func (r *reader) readNotes(sh *model.Sheet, name string) {
	comments, err := r.f.GetComments(name)
	if err != nil {
		return
	}
	for _, c := range comments {
		col, row, err := excelize.CellNameToCoordinates(c.Cell)
		if err != nil {
			continue
		}
		if sh.Layout.Notes == nil {
			sh.Layout.Notes = map[[2]int]model.Note{}
		}
		sh.Layout.Notes[[2]int{row - 1, col - 1}] = model.Note{Author: c.Author, Text: noteText(c)}
	}
}

// noteText joins a comment's text, dropping the "Author:" line Excel
// puts in front of a note.
func noteText(c excelize.Comment) string {
	var b strings.Builder
	b.WriteString(c.Text)
	for _, run := range c.Paragraph {
		b.WriteString(run.Text)
	}
	text := b.String()
	if c.Author != "" {
		text = strings.TrimPrefix(text, c.Author+":\n")
		text = strings.TrimPrefix(text, c.Author+":\r\n")
	}
	return text
}

// readTable reads a table part (xl/tables/tableN.xml).
func readTable(parts map[string]*zip.File, name string) (model.Table, bool) {
	var t struct {
		Name   string `xml:"name,attr"`
		Ref    string `xml:"ref,attr"`
		Totals int    `xml:"totalsRowCount,attr"`
		Info   struct {
			Name    string `xml:"name,attr"`
			Stripes string `xml:"showRowStripes,attr"`
		} `xml:"tableStyleInfo"`
	}
	if name == "" || decodePart(parts, name, &t) != nil || t.Name == "" {
		return model.Table{}, false
	}
	rs := parseSqref(t.Ref)
	if len(rs) != 1 {
		return model.Table{}, false
	}
	return model.Table{Name: t.Name, Range: rs[0], Style: t.Info.Name, TotalsRow: t.Totals > 0, NoBands: t.Info.Stripes == "0"}, true
}

// cfRuleFromAttrs maps an OOXML cfRule to our rule (without formulas).
func cfRuleFromAttrs(t xml.StartElement) model.CFRule {
	r := model.CFRule{StopIfTrue: attr(t, "stopIfTrue") == "1"}
	switch attr(t, "type") {
	case "cellIs":
		r.Type, r.Op = "cell", attr(t, "operator")
	case "expression":
		r.Type = "formula"
	case "top10":
		r.Type, r.Value1, r.Percent = "top", attr(t, "rank"), attr(t, "percent") == "1"
		if attr(t, "bottom") == "1" {
			r.Type = "bottom"
		}
	case "aboveAverage":
		r.Type, r.Below = "average", attr(t, "aboveAverage") == "0"
	case "duplicateValues":
		r.Type = "duplicate"
	case "uniqueValues":
		r.Type = "unique"
	case "containsText", "notContainsText", "beginsWith", "endsWith":
		r.Type, r.Value1 = "text", attr(t, "text")
		r.Op = map[string]string{"containsText": "containsText", "notContainsText": "notContains", "beginsWith": "beginsWith", "endsWith": "endsWith"}[attr(t, "type")]
	case "containsBlanks":
		r.Type = "blanks"
	case "notContainsBlanks":
		r.Type = "no_blanks"
	case "containsErrors":
		r.Type = "errors"
	case "notContainsErrors":
		r.Type = "no_errors"
	case "colorScale":
		r.Type = "color_scale"
	case "dataBar":
		r.Type = "data_bar"
	}
	return r
}

// condFormats finishes the raw rules: formulas, colors, formats and
// priority order.
func (r *reader) condFormats(raw []rawRule) []model.CFRule {
	sort.SliceStable(raw, func(i, j int) bool { return raw[i].priority < raw[j].priority })
	var out []model.CFRule
	for _, rr := range raw {
		rule := rr.rule
		switch rule.Type {
		case "":
			continue // icon sets, time periods: not supported
		case "cell":
			if len(rr.formulas) > 0 {
				rule.Value1 = rr.formulas[0]
			}
			if len(rr.formulas) > 1 {
				rule.Value2 = rr.formulas[1]
			}
		case "formula":
			if len(rr.formulas) == 0 {
				continue
			}
			rule.Value1 = rr.formulas[0]
		case "color_scale":
			if len(rr.colors) >= 3 {
				rule.Type = "3_color_scale"
			} else {
				rule.Type = "2_color_scale"
			}
			rule.Colors = rr.colors
		case "data_bar":
			rule.Colors = rr.colors
		}
		rule.Value1 = strings.TrimPrefix(rule.Value1, "=")
		rule.Value2 = strings.TrimPrefix(rule.Value2, "=")
		if rr.dxf >= 0 {
			if st, err := r.f.GetConditionalStyle(rr.dxf); err == nil && st != nil {
				rule.Format = cfFormatOf(st)
			}
		}
		out = append(out, rule)
	}
	return out
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
