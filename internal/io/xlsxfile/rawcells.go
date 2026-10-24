package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// rawCell is what the package XML says about a <c> element.
type rawCell struct {
	style   int
	formula bool
}

// rawCells reads the style index and formula presence of every <c>
// element of every sheet straight from the package XML. excelize's
// GetCellStyle pads the sheet with empty cells as a side effect, which
// would bloat in-place saves, and its row iterator drops cells that only
// carry a style or have an empty formula result.
func rawCells(file string) (map[string]map[[2]int]rawCell, error) {
	z, err := zip.OpenReader(file)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	parts := map[string]*zip.File{}
	for _, f := range z.File {
		parts[f.Name] = f
	}
	sheets, err := sheetParts(parts)
	if err != nil {
		return nil, err
	}
	out := map[string]map[[2]int]rawCell{}
	for name, part := range sheets {
		f, ok := parts[part]
		if !ok {
			continue
		}
		cells, err := scanSheet(f)
		if err != nil {
			return nil, err
		}
		out[name] = cells
	}
	return out, nil
}

// sheetParts maps sheet names to their worksheet part paths.
func sheetParts(parts map[string]*zip.File) (map[string]string, error) {
	var wbDoc struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := decodePart(parts, "xl/workbook.xml", &wbDoc); err != nil {
		return nil, err
	}
	if err := decodePart(parts, "xl/_rels/workbook.xml.rels", &rels); err != nil {
		return nil, err
	}
	targets := map[string]string{}
	for _, r := range rels.Rels {
		t := r.Target
		if strings.HasPrefix(t, "/") {
			t = strings.TrimPrefix(t, "/")
		} else {
			t = path.Join("xl", t)
		}
		targets[r.ID] = t
	}
	out := map[string]string{}
	for _, s := range wbDoc.Sheets {
		out[s.Name] = targets[s.RID]
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

func scanSheet(f *zip.File) (map[[2]int]rawCell, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := map[[2]int]rawCell{}
	dec := xml.NewDecoder(r)
	row, col := 0, 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch el.Name.Local {
		case "row":
			if v := attr(el, "r"); v != "" {
				row, _ = strconv.Atoi(v)
			} else {
				row++
			}
			col = 0
		case "c":
			if ref := attr(el, "r"); ref != "" {
				if c, rr, err := excelize.CellNameToCoordinates(ref); err == nil {
					col, row = c, rr
				}
			} else {
				col++
			}
			if s, err := strconv.Atoi(attr(el, "s")); err == nil && s > 0 {
				out[[2]int{row - 1, col - 1}] = rawCell{style: s}
			}
		case "f":
			pos := [2]int{row - 1, col - 1}
			c := out[pos]
			c.formula = true
			out[pos] = c
		}
	}
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
