package xlsxfile

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"ww-excel/internal/model"
)

type cfXMLEdit struct {
	start, end int
	text       []byte
}

func applyCFEdits(data []byte, edits []cfXMLEdit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, edit := range edits {
		data = append(append(append([]byte{}, data[:edit.start]...), edit.text...), data[edit.end:]...)
	}
	return data
}

func escapeCF(s string) string { var b bytes.Buffer; xml.EscapeText(&b, []byte(s)); return b.String() }

// Custom per-slot icons cannot be expressed by Excelize's preset-only API.
func iconRuleXML(rule model.CFRule, priority int) []byte {
	set := rule.IconSet
	if rule.Priority > 0 {
		priority = rule.Priority
	}
	id := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", priority, formatSqref(rule.Ranges))))
	var b bytes.Buffer
	fmt.Fprintf(&b, `<x14:conditionalFormatting xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main"><x14:cfRule type="iconSet" priority="%d" id="{%x-%x-%x-%x-%x}"><x14:iconSet iconSet="%s" reverse="%t" showValue="%t" custom="%t">`, priority, id[:4], id[4:6], id[6:8], id[8:10], id[10:16], escapeCF(set.Set), set.Reverse, !set.HideValue, len(set.Icons) > 0)
	for _, threshold := range set.Thresholds {
		fmt.Fprintf(&b, `<x14:cfvo type="%s" gte="%t">`, escapeCF(threshold.Type), !threshold.Strict)
		if threshold.Value != "" {
			fmt.Fprintf(&b, `<xm:f>%s</xm:f>`, escapeCF(threshold.Value))
		}
		b.WriteString(`</x14:cfvo>`)
	}
	for _, icon := range set.Icons {
		fmt.Fprintf(&b, `<x14:cfIcon iconSet="%s" iconId="%d"/>`, escapeCF(icon.Set), icon.Index)
	}
	fmt.Fprintf(&b, `</x14:iconSet></x14:cfRule><xm:sqref>%s</xm:sqref></x14:conditionalFormatting>`, escapeCF(formatSqref(rule.Ranges)))
	return b.Bytes()
}

// Keep worksheet bytes intact outside icon rules, including unrelated x14
// extensions and namespace declarations. Every save repeats this replacement,
// so the source workbook cached by Excelize cannot restore preset icons later.
func replaceWorksheetIcons(data []byte, rules []model.CFRule) ([]byte, error) {
	var icons bytes.Buffer
	for i, rule := range rules {
		if rule.Type == "icon_set" && rule.IconSet != nil {
			icons.Write(iconRuleXML(rule, i+1))
		}
	}
	if icons.Len() == 0 {
		return data, nil
	}
	var edits []cfXMLEdit
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth, groupStart, groupDepth, extensionEnd, containerEnd, worksheetEnd := 0, -1, 0, -1, -1, -1
	for {
		start := int(dec.InputOffset())
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "conditionalFormatting" {
				groupStart, groupDepth = start, depth
			}
		case xml.EndElement:
			if t.Name.Local == "conditionalFormatting" && depth == groupDepth {
				end := int(dec.InputOffset())
				source := data[groupStart:end]
				var group rawConditionalGroup
				if err := xml.Unmarshal(source, &group); err != nil {
					return nil, err
				}
				count := 0
				for _, rule := range group.Rules {
					if rule.Icons != nil {
						count++
					}
				}
				if count > 0 {
					if count == len(group.Rules) {
						edits = append(edits, cfXMLEdit{groupStart, end, nil})
					} else {
						// Leave other rule types in a shared conditionalFormatting group.
						sub := xml.NewDecoder(bytes.NewReader(source))
						ruleStart := -1
						for {
							offset := int(sub.InputOffset())
							item, e := sub.Token()
							if e == io.EOF {
								break
							}
							if e != nil {
								return nil, e
							}
							switch x := item.(type) {
							case xml.StartElement:
								if x.Name.Local == "cfRule" && attr(x, "type") == "iconSet" {
									ruleStart = offset
								}
							case xml.EndElement:
								if x.Name.Local == "cfRule" && ruleStart >= 0 {
									edits = append(edits, cfXMLEdit{groupStart + ruleStart, groupStart + int(sub.InputOffset()), nil})
									ruleStart = -1
								}
							}
						}
					}
				}
				groupStart = -1
			}
			if t.Name.Local == "conditionalFormattings" {
				containerEnd = start
			}
			if t.Name.Local == "extLst" && depth == 2 {
				extensionEnd = start
			}
			if t.Name.Local == "worksheet" && depth == 1 {
				worksheetEnd = start
			}
			depth--
		}
	}
	insertAt := containerEnd
	insert := icons.Bytes()
	if insertAt < 0 {
		insert = []byte(`<ext uri="{78C0D931-6437-407d-A8EE-F0AAD7539E65}"><x14:conditionalFormattings xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` + string(insert) + `</x14:conditionalFormattings></ext>`)
		insertAt = extensionEnd
		if insertAt < 0 {
			insertAt = worksheetEnd
			insert = []byte(`<extLst>` + string(insert) + `</extLst>`)
		}
	}
	if insertAt < 0 {
		return nil, fmt.Errorf("worksheet closing element missing")
	}
	edits = append(edits, cfXMLEdit{insertAt, insertAt, insert})
	return applyCFEdits(data, edits), nil
}

// Restore conditional-format details after every save, including saves from
// Excelize's cached workbook, without rewriting unrelated archive entries.
func preserveConditionalDetails(filename string, wb *model.Workbook) error {
	wanted := map[string][]model.CFRule{}
	for _, sheet := range wb.Sheets {
		for _, rule := range sheet.Layout.CondFormats {
			if rule.Type == "icon_set" && rule.IconSet != nil || rule.Type == "data_bar" && rule.DataBar != nil {
				wanted[sheet.Name] = sheet.Layout.CondFormats
				break
			}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	source, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer source.Close()
	parts := map[string]*zip.File{}
	for _, entry := range source.File {
		parts[entry.Name] = entry
	}
	sheets, err := sheetParts(parts)
	if err != nil {
		return err
	}
	byPart := map[string][]model.CFRule{}
	for name, rules := range wanted {
		if part := sheets[name]; part != "" {
			byPart[part] = rules
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(filename), ".conditional-details-*.xlsx")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	out := zip.NewWriter(tmp)
	for _, entry := range source.File {
		if rules, ok := byPart[entry.Name]; ok {
			reader, e := entry.Open()
			if e != nil {
				return e
			}
			data, e := io.ReadAll(reader)
			reader.Close()
			if e != nil {
				return e
			}
			data, e = replaceWorksheetBarBounds(data, rules)
			if e != nil {
				return e
			}
			data, e = replaceWorksheetIcons(data, rules)
			if e != nil {
				return e
			}
			writer, e := out.CreateHeader(&entry.FileHeader)
			if e != nil {
				return e
			}
			if _, e = writer.Write(data); e != nil {
				return e
			}
		} else if e := out.Copy(entry); e != nil {
			return e
		}
	}
	if err = out.Close(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filename)
}
