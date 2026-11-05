package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"ww-excel/internal/model"
)

func (w *writer) addShapeObject(sh *model.Sheet, obj model.Object, cell string) error {
	before := map[string]string{}
	w.f.Drawings.Range(func(k, v any) bool { data, _ := xml.Marshal(v); before[k.(string)] = string(data); return true })
	if err := w.addObject(sh, obj, cell); err != nil {
		return err
	}
	target := 0
	w.f.Drawings.Range(func(k, v any) bool {
		data, _ := xml.Marshal(v)
		if before[k.(string)] == string(data) {
			return true
		}
		root, err := drawingNodes(data)
		if err != nil {
			return true
		}
		for _, node := range root.all("sp") {
			target = max(target, node.id())
		}
		return true
	})
	if target == 0 {
		return fmt.Errorf("無法寫入文字方塊")
	}
	if w.shapeAdded == nil {
		w.shapeAdded = map[*model.Sheet]map[int]int{}
	}
	if w.shapeAdded[sh] == nil {
		w.shapeAdded[sh] = map[int]int{}
	}
	w.shapeAdded[sh][obj.ID] = target
	return nil
}

func shapeTransform(l *model.Layout, obj model.Object) string {
	x, y := obj.Anchor.DX, obj.Anchor.DY
	for col := 0; col < obj.Anchor.Col; col++ {
		x += l.ColWidth(col)
	}
	for row := 0; row < obj.Anchor.Row; row++ {
		y += l.RowHeight(row)
	}
	width, height := obj.W, obj.H
	shape := obj.Shape
	if shape.W > 0 && shape.H > 0 {
		width, height = shape.W, shape.H
		x += (obj.W - width) / 2
		y += (obj.H - height) / 2
	}
	return fmt.Sprintf(`<a:xfrm xmlns:a="%s" rot="%d" flipH="%d" flipV="%d"><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, drawingNS, int64(shape.Rot*60000), boolInt(shape.FlipH), boolInt(shape.FlipV), int64(mathRound(x*emuPerPx)), int64(mathRound(y*emuPerPx)), int64(mathRound(width*emuPerPx)), int64(mathRound(height*emuPerPx)))
}
func markerXML(name string, a model.Anchor) string {
	return fmt.Sprintf(`<xdr:%s xmlns:xdr="%s"><xdr:col>%d</xdr:col><xdr:colOff>%d</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>%d</xdr:rowOff></xdr:%s>`, name, spreadsheetDrawingNS, a.Col, int64(mathRound(a.DX*emuPerPx)), a.Row, int64(mathRound(a.DY*emuPerPx)), name)
}
func positionShape(data []byte, node *drawingNode, obj model.Object, l *model.Layout, anchor bool) []byte {
	var edits []drawingEdit
	if properties := node.child("spPr"); properties != nil {
		transform := shapeTransform(l, obj)
		if current := properties.child("xfrm"); current != nil {
			edits = append(edits, drawingEdit{current.start, current.end, transform})
		} else {
			edits = append(edits, drawingEdit{properties.open, properties.open, transform})
		}
	}
	if anchor {
		frame := node.parent
		if frame != nil && (frame.name == "oneCellAnchor" || frame.name == "twoCellAnchor") {
			if from := frame.child("from"); from != nil {
				edits = append(edits, drawingEdit{from.start, from.end, markerXML("from", obj.Anchor)})
			}
			if to := frame.child("to"); to != nil {
				end := shiftAnchor(l, obj.Anchor, obj.W, obj.H)
				edits = append(edits, drawingEdit{to.start, to.end, markerXML("to", end)})
			}
			if ext := frame.child("ext"); ext != nil {
				value := fmt.Sprintf(`<xdr:ext xmlns:xdr="%s" cx="%d" cy="%d"/>`, spreadsheetDrawingNS, int64(mathRound(obj.W*emuPerPx)), int64(mathRound(obj.H*emuPerPx)))
				edits = append(edits, drawingEdit{ext.start, ext.end, value})
			}
		}
	}
	return applyDrawingEdits(data, edits)
}

// A high stable ID lets a WWX-origin shape be found again on the next save
// without mutating its immutable provenance or undo history.
func exportedShapeID(obj model.Object) int { return 1000000 + obj.ID }
func shapeNode(root *drawingNode, obj model.Object) *drawingNode {
	if obj.Shape.SourceID != 0 {
		if node := findShape(root, obj.Shape.SourceID); node != nil {
			return node
		}
	}
	return findShape(root, exportedShapeID(obj))
}
func patchShapeDrawing(data []byte, sh *model.Sheet, old []model.Object, added map[int]int) ([]byte, error) {
	previous := map[int]model.Object{}
	current := map[int]bool{}
	for _, obj := range old {
		if obj.Shape != nil {
			previous[obj.ID] = obj
		}
	}
	for _, obj := range sh.Layout.Objects {
		if obj.Shape == nil {
			continue
		}
		current[obj.ID] = true
		prev, exists := previous[obj.ID]
		if exists && reflect.DeepEqual(prev, obj) {
			continue
		}
		root, err := drawingNodes(data)
		if err != nil {
			return nil, err
		}
		node := shapeNode(root, obj)
		if target, ok := added[obj.ID]; ok {
			node = findShape(root, target)
			if node == nil {
				return nil, fmt.Errorf("無法寫入文字方塊")
			}
			if obj.Shape.RawXML != "" {
				raw := []byte(obj.Shape.RawXML)
				saved, e := drawingNodes(raw)
				if e != nil {
					return nil, e
				}
				part := findShape(saved, obj.Shape.SourceID)
				if part != nil {
					raw = positionShape(raw, part, obj, &sh.Layout, false)
					saved, _ = drawingNodes(raw)
					part = findShape(saved, obj.Shape.SourceID)
					// New workbook relationships and theme colors are resolved values.
					// Remove source hyperlinks whose relationship IDs cannot be transferred.
					var strips []drawingEdit
					for _, name := range []string{"hlinkClick", "hlinkMouseOver"} {
						for _, link := range saved.all(name) {
							strips = append(strips, drawingEdit{link.start, link.end, ""})
						}
					}
					raw = applyDrawingEdits(raw, strips)
					saved, _ = drawingNodes(raw)
					part = findShape(saved, obj.Shape.SourceID)
					raw = applyDrawingEdits(raw, []drawingEdit{drawingAttribute(raw, part.child("nvSpPr").child("cNvPr"), "id", strconv.Itoa(exportedShapeID(obj)))})
					data = applyDrawingEdits(data, []drawingEdit{{node.start, node.end, string(raw)}})
				}
			} else {
				data = applyDrawingEdits(data, []drawingEdit{drawingAttribute(data, node.child("nvSpPr").child("cNvPr"), "id", strconv.Itoa(exportedShapeID(obj)))})
			}
			root, _ = drawingNodes(data)
			node = findShape(root, exportedShapeID(obj))
		}
		if node == nil {
			return nil, fmt.Errorf("文字方塊不存在")
		}
		if !exists || !reflect.DeepEqual(prev.Shape.Text, obj.Shape.Text) || prev.Shape.Font != obj.Shape.Font {
			data = replaceShapeText(data, node, obj.Shape)
			root, _ = drawingNodes(data)
			node = shapeNode(root, obj)
			if _, ok := added[obj.ID]; ok {
				node = findShape(root, exportedShapeID(obj))
			}
		}
		if exists && !obj.ReadOnly && (prev.Anchor != obj.Anchor || prev.W != obj.W || prev.H != obj.H) {
			data = positionShape(data, node, obj, &sh.Layout, true)
		}
	}
	for id, obj := range previous {
		if current[id] {
			continue
		}
		root, err := drawingNodes(data)
		if err != nil {
			return nil, err
		}
		node := shapeNode(root, obj)
		if node == nil {
			continue
		}
		remove := node
		if node.parent != nil && (node.parent.name == "oneCellAnchor" || node.parent.name == "twoCellAnchor") {
			remove = node.parent
		}
		data = applyDrawingEdits(data, []drawingEdit{{remove.start, remove.end, ""}})
	}
	return data, nil
}

// Patch the saved ZIP rather than reserializing DrawingML through Excelize.
// This retains unknown shape properties, group containers and sibling objects.
func (w *writer) preserveShapes(filename string, old map[*model.Sheet]model.Layout) error {
	needed := false
	for _, sh := range w.wb.Sheets {
		if hasChartObjects(sh.Layout.Objects) {
			needed = true
			break
		}
		prev, tracked := old[sh]
		if !tracked && len(w.shapeAdded[sh]) == 0 {
			continue
		}
		if len(w.shapeAdded[sh]) > 0 || !reflect.DeepEqual(shapeObjects(prev.Objects), shapeObjects(sh.Layout.Objects)) {
			needed = true
			break
		}
	}
	if !needed {
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
	changed := map[string][]byte{}
	for _, sh := range w.wb.Sheets {
		prev, tracked := old[sh]
		shapeChanged := len(w.shapeAdded[sh]) > 0 || tracked && !reflect.DeepEqual(shapeObjects(prev.Objects), shapeObjects(sh.Layout.Objects))
		if !shapeChanged && !hasChartObjects(sh.Layout.Objects) {
			continue
		}
		sheetPart := sheets[sh.Name]
		root, e := drawingNodes(readPart(parts, sheetPart))
		if e != nil {
			return e
		}
		drawings := root.all("drawing")
		if len(drawings) == 0 {
			return fmt.Errorf("無法寫入文字方塊")
		}
		part := relTarget(parts, sheetPart, drawings[0].attr("id"))
		if part == "" {
			return fmt.Errorf("無法寫入文字方塊")
		}
		data := readPart(parts, part)
		if shapeChanged {
			data, e = patchShapeDrawing(data, sh, prev.Objects, w.shapeAdded[sh])
			if e != nil {
				return e
			}
		}
		data = normalizeChartDrawingAnchors(data, &sh.Layout)
		changed[part] = data
	}
	temp, err := os.CreateTemp(filepath.Dir(filename), ".shape-details-*.xlsx")
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
	for name, data := range changed {
		w.f.Pkg.Store(name, data)
		w.f.Drawings.Delete(name)
	}
	return nil
}
func shapeObjects(objects []model.Object) []model.Object {
	var out []model.Object
	for _, obj := range objects {
		if obj.Shape != nil {
			out = append(out, obj)
		}
	}
	return out
}
