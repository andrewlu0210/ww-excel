package xlsxfile

import (
	"path"
	"strings"

	"ww-excel/internal/model"
)

// Materialize the appearance of cached pivot cells, without creating a table,
// refreshing the cache, or changing formulas and values. The original pivot
// definition stays in the source XLSX for in-place saves.
func (r *reader) readPivotAppearance(sh *model.Sheet, part string) {
	dir, base := path.Split(part)
	var rels struct {
		Items []struct {
			ID   string `xml:"Id,attr"`
			Type string `xml:"Type,attr"`
			Mode string `xml:"TargetMode,attr"`
		} `xml:"Relationship"`
	}
	if decodePart(r.parts, dir+"_rels/"+base+".rels", &rels) != nil {
		return
	}
	for _, rel := range rels.Items {
		if !strings.HasSuffix(rel.Type, "/pivotTable") || rel.Mode == "External" {
			continue
		}
		var pivot struct {
			ShowHeaders string `xml:"showHeaders,attr"`
			Location    struct {
				Ref          string `xml:"ref,attr"`
				FirstDataRow int    `xml:"firstDataRow,attr"`
				FirstDataCol int    `xml:"firstDataCol,attr"`
			} `xml:"location"`
			Style struct {
				Name string `xml:"name,attr"`
			} `xml:"pivotTableStyleInfo"`
			Rows []struct {
				Type string `xml:"t,attr"`
			} `xml:"rowItems>i"`
		}
		if decodePart(r.parts, relTarget(r.parts, part, rel.ID), &pivot) != nil {
			continue
		}
		ranges := parseSqref(pivot.Location.Ref)
		if len(ranges) != 1 {
			continue
		}
		rg := ranges[0]
		headerRows := min(max(pivot.Location.FirstDataRow, 0), rg.R1-rg.R0+1)
		rowHeaders := min(max(pivot.Location.FirstDataCol, 0), rg.C1-rg.C0+1)
		if headerRows > 0 && rowHeaders > 0 && pivot.ShowHeaders != "0" && pivot.ShowHeaders != "false" {
			sh.Layout.PivotHeaders = append(sh.Layout.PivotHeaders, model.Range{R0: rg.R0, C0: rg.C0, R1: rg.R0, C1: rg.C0 + rowHeaders - 1})
		}
		appearance := r.tableAppearance(pivot.Style.Name)
		if appearance == nil {
			continue
		}
		hasTotal := len(pivot.Rows) > 0 && pivot.Rows[len(pivot.Rows)-1].Type == "grand"
		// Only the header and grand-total rows have styles that we can locate
		// unambiguously without implementing pivot field layout and refresh.
		for row := rg.R0; row <= rg.R1; row++ {
			header, total := row < rg.R0+headerRows, hasTotal && row == rg.R1
			if !header && !total {
				continue
			}
			for col := rg.C0; col <= rg.C1; col++ {
				cell := sh.Get(row, col)
				styleID := cell.S
				if styleID == 0 {
					styleID = sh.Layout.StyleAt(row, col)
				}
				style := r.wb.StyleTable().Get(styleID)
				font, fill := appearance.Header, appearance.HeaderFill
				if total {
					font, fill = appearance.Totals, appearance.TotalsFill
				}
				style.Font = font.Resolve(style.Font)
				if fill != nil && style.Fill == "" {
					style.Fill = *fill
				}
				cell.S = r.wb.StyleTable().InternExplicit(style)
				sh.Set(row, col, cell)
			}
		}
	}
}
