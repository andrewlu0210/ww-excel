package xlsxfile

import (
	"fmt"
	"path"
	"strings"
)

// Check the actual XML element counts before Excelize can discard invalid
// field indices. Reject malformed definitions instead of importing partial pivots.
func (r *reader) validatePivotFields(sheetPart string) error {
	dir, base := path.Split(sheetPart)
	var rels struct {
		Items []struct {
			ID   string `xml:"Id,attr"`
			Type string `xml:"Type,attr"`
		} `xml:"Relationship"`
	}
	if err := decodePart(r.parts, dir+"_rels/"+base+".rels", &rels); err != nil {
		return fmt.Errorf("樞紐分析表資料無法讀取：%w", err)
	}
	for _, rel := range rels.Items {
		if !strings.HasSuffix(rel.Type, "/pivotTable") {
			continue
		}
		pivotPart := relTarget(r.parts, sheetPart, rel.ID)
		var pivot struct {
			Name   string     `xml:"name,attr"`
			Fields []struct{} `xml:"pivotFields>pivotField"`
			Data   []struct {
				Index int `xml:"fld,attr"`
			} `xml:"dataFields>dataField"`
		}
		if r.parts[pivotPart] == nil {
			return fmt.Errorf("樞紐分析表資料無法讀取：%w", fmt.Errorf("missing pivot table part %q", pivotPart))
		}
		if err := decodePart(r.parts, pivotPart, &pivot); err != nil {
			return fmt.Errorf("樞紐分析表資料無法讀取：%w", err)
		}
		cachePart := relOfType(r.parts, pivotPart, "/pivotCacheDefinition")
		var cache struct {
			Fields []struct{} `xml:"cacheFields>cacheField"`
		}
		if r.parts[cachePart] == nil {
			return fmt.Errorf("樞紐分析表資料無法讀取：%w", fmt.Errorf("missing pivot cache part %q", cachePart))
		}
		if err := decodePart(r.parts, cachePart, &cache); err != nil {
			return fmt.Errorf("樞紐分析表資料無法讀取：%w", err)
		}
		if len(pivot.Fields) == 0 || len(pivot.Fields) != len(cache.Fields) {
			return fmt.Errorf("樞紐分析表「%s」的欄位數與快取不一致", pivot.Name)
		}
		for _, field := range pivot.Data {
			if field.Index < 0 || field.Index >= len(cache.Fields) {
				return fmt.Errorf("樞紐分析表「%s」的資料欄位索引超出快取範圍", pivot.Name)
			}
		}
	}
	return nil
}
