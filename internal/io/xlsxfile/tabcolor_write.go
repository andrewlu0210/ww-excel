package xlsxfile

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
)

const worksheetNamespace = ` xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`

// Replace the entire color node: setting only RGB leaves an old theme/tint active.
func patchTabColor(data []byte, color string) ([]byte, error) {
	root, err := drawingNodes(data)
	if err != nil {
		return nil, err
	}
	if len(root.children) != 1 {
		return nil, errors.New("無法更新分頁色彩")
	}
	sheet := root.children[0]
	properties := sheet.child("sheetPr")
	raw := []byte(`<sheetPr` + worksheetNamespace + `/>`)
	if properties != nil {
		raw = []byte(properties.raw(data))
	}
	parsed, err := drawingNodes(raw)
	if err != nil {
		return nil, err
	}
	value := ""
	if color != "" {
		value = `<tabColor` + worksheetNamespace + ` rgb="FF` + escapeDrawing(color) + `"/>`
	}
	patched := replaceChartChildren(raw, parsed.children[0], []chartReplacement{{"tabColor", value}}, []string{"tabColor", "outlinePr", "pageSetUpPr", "extLst"})
	result := replaceChartChildren(data, sheet, []chartReplacement{{"sheetPr", patched}}, []string{"sheetPr", "dimension", "sheetViews", "sheetFormatPr", "cols", "sheetData"})
	return []byte(result), nil
}
func (w *writer) writeTabColors(filename string) error {
	if len(w.tabColors) == 0 {
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
	for name, color := range w.tabColors {
		part := sheets[name]
		data, err := patchTabColor(readPart(parts, part), color)
		if err != nil {
			return err
		}
		changed[part] = data
	}
	temp, err := os.CreateTemp(filepath.Dir(filename), ".tab-colors-*.xlsx")
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
	if err := out.Close(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), filename); err != nil {
		return err
	}
	for part, data := range changed {
		w.f.Pkg.Store(part, data)
		w.f.Sheet.Delete(part)
	}
	return nil
}
