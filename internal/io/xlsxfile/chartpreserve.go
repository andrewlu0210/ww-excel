package xlsxfile

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"ww-excel/internal/model"
)

func attachChartSource(parts map[string]*zip.File, name string, c *model.Chart) {
	data := readPart(parts, name)
	root, err := drawingNodes(data)
	if err != nil {
		return
	}
	for i := range c.Series {
		labels := c.Series[i].Labels
		if labels == nil {
			continue
		}
		for _, node := range root.all("dLbls") {
			if string(data[node.open:node.close]) == labels.RawXML {
				labels.RawXML = standaloneChartNode(data, node)
				break
			}
		}
		labels.SourceConfig = labelConfig(labels)
	}
	var ct struct {
		Overrides []struct {
			Name string `xml:"PartName,attr"`
			Type string `xml:"ContentType,attr"`
		} `xml:"Override"`
		Defaults []struct {
			Ext  string `xml:"Extension,attr"`
			Type string `xml:"ContentType,attr"`
		} `xml:"Default"`
	}
	decodePart(parts, "[Content_Types].xml", &ct)
	types := map[string]string{}
	defaults := map[string]string{}
	for _, v := range ct.Overrides {
		types[strings.TrimPrefix(v.Name, "/")] = v.Type
	}
	for _, v := range ct.Defaults {
		defaults[v.Ext] = v.Type
	}
	source := &model.ChartSource{XML: string(data), Part: name, Baseline: c.Clone(), Resources: map[string]model.ChartResource{}}
	visited := map[string]bool{name: true}
	var visit func(string)
	visit = func(part string) {
		rel := chartRelsPart(part)
		raw := readPart(parts, rel)
		if len(raw) == 0 {
			return
		}
		source.Resources[rel] = model.ChartResource{Data: raw, ContentType: "application/vnd.openxmlformats-package.relationships+xml"}
		var rels struct {
			Items []struct {
				Target string `xml:"Target,attr"`
				Mode   string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if xml.Unmarshal(raw, &rels) != nil {
			return
		}
		for _, r := range rels.Items {
			if r.Mode == "External" {
				continue
			}
			target := chartRelTarget(part, r.Target)
			if visited[target] {
				continue
			}
			visited[target] = true
			raw := readPart(parts, target)
			if len(raw) == 0 {
				continue
			}
			typ := types[target]
			if typ == "" {
				typ = defaults[strings.TrimPrefix(path.Ext(target), ".")]
			}
			source.Resources[target] = model.ChartResource{Data: raw, ContentType: typ}
			visit(target)
		}
	}
	visit(name)
	c.Source = source
}
func standaloneChartNode(data []byte, node *drawingNode) string {
	raw := node.raw(data)
	at := strings.Index(raw, ">")
	if at < 0 {
		return raw
	}
	namespaces := map[string]string{}
	for p := node; p != nil; p = p.parent {
		for _, a := range p.attrs {
			key := ""
			if a.Name.Space == "xmlns" {
				key = "xmlns:" + a.Name.Local
			} else if a.Name.Local == "xmlns" {
				key = "xmlns"
			}
			if key != "" {
				if _, ok := namespaces[key]; !ok {
					namespaces[key] = a.Value
				}
			}
		}
	}
	attrs := ""
	for key, value := range namespaces {
		if !strings.Contains(raw[:at], key+"=") {
			attrs += " " + key + `="` + escapeDrawing(value) + `"`
		}
	}
	if at > 0 && raw[at-1] == '/' {
		at--
	}
	return raw[:at] + attrs + raw[at:]
}
func chartRelsPart(part string) string {
	return path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
}
func chartRelTarget(part, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(target, "/")
	}
	return path.Clean(path.Join(path.Dir(part), target))
}

// Preserve only edits whose exact XML fields we can update. Other changes keep
// using the existing rebuild path and its compatibility warning.
func CanPreserveChart(c *model.Chart) bool {
	if c == nil || c.Source == nil || c.Source.Baseline == nil || c.Source.XML == "" {
		return false
	}
	a, b := c.Clone(), c.Source.Baseline.Clone()
	a.Source = nil
	b.Source = nil
	normalize := func(c *model.Chart) {
		c.GapWidth = nil
		c.HoleSize = 0
		c.Title = ""
		c.AutoTitle = false
		c.Legend = ""
		c.XTitle = ""
		c.YTitle = ""
		c.ValueAxis = nil
		c.CategoryAxis = nil
		c.SecondaryAxis = nil
		c.SecondaryCategoryAxis = nil
		for i := range c.Series {
			c.Series[i].Color = ""
			c.Series[i].LineWidth = 0
			c.Series[i].Marker = ""
			c.Series[i].Name = ""
			c.Series[i].Categories = ""
			c.Series[i].Values = ""
			c.Series[i].Labels = nil
			if len(c.Series[i].PointColors) == 0 {
				c.Series[i].PointColors = nil
			}
			if len(c.Series[i].PointStyles) == 0 {
				c.Series[i].PointStyles = nil
			}
			if len(c.Series[i].CategoryLabels) == 0 {
				c.Series[i].CategoryLabels = nil
			}
		}
	}
	normalize(a)
	normalize(b)
	if !reflect.DeepEqual(a, b) {
		return false
	}
	root, err := drawingNodes([]byte(c.Source.XML))
	return err == nil && len(root.all("ser")) == len(c.Series)
}

var chartChildOrder = []string{"title", "autoTitleDeleted", "pivotFmts", "view3D", "floor", "sideWall", "backWall", "plotArea", "legend", "plotVisOnly", "dispBlanksAs", "showDLblsOverMax", "extLst"}
var seriesChildOrder = []string{"idx", "order", "tx", "spPr", "invertIfNegative", "pictureOptions", "marker", "dPt", "dLbls", "trendline", "errBars", "cat", "val", "xVal", "yVal", "smooth", "shape", "extLst"}

func chartTitleText(old *drawingNode, data []byte, text string) string {
	if old == nil {
		return chartElement("title", labelTextXML(text)+chartBoolXML("overlay", false))
	}
	// Reuse the original title's paragraph/run formatting while replacing text.
	tx := labelTextXML(text)
	parsed, _ := drawingNodes([]byte(tx))
	if parsed != nil {
		for _, name := range []string{"pPr", "rPr"} {
			nodes := old.all(name)
			if len(nodes) > 0 {
				tag := "<a:p>"
				if name == "rPr" {
					tag = "<a:r>"
				}
				tx = strings.ReplaceAll(tx, tag, tag+standaloneChartNode(data, nodes[0]))
			}
		}
	}
	return replaceChartChildren(data, old, []chartReplacement{{"tx", tx}}, []string{"tx", "layout", "overlay", "spPr", "txPr", "extLst"})
}
func patchChartReferences(source []byte, old, new model.ChartSeries, generated *drawingNode, gen []byte, ser *drawingNode, kind string) string {
	changes := []chartReplacement{}
	ref := func(name string) {
		value := ""
		if node := generated.child(name); node != nil {
			value = standaloneChartNode(gen, node)
		}
		changes = append(changes, chartReplacement{name, value})
	}
	if old.Name != new.Name {
		ref("tx")
	}
	if old.Categories != new.Categories {
		if ser.child("xVal") != nil {
			ref("xVal")
		} else {
			ref("cat")
		}
	}
	if old.Values != new.Values {
		if ser.child("yVal") != nil {
			ref("yVal")
		} else {
			ref("val")
		}
	}
	if labelConfig(old.Labels) != labelConfig(new.Labels) || !reflect.DeepEqual(old.Labels, new.Labels) {
		changes = append(changes, chartReplacement{"dLbls", chartLabelsXML(new.Labels)})
	}
	changes = append(changes, seriesAppearanceChanges(source, ser, old, new, kind)...)
	return replaceChartChildren(source, ser, changes, seriesChildOrder)
}
func matchChartSeries(nodes []*drawingNode, data []byte, series model.ChartSeries, used map[*drawingNode]bool) *drawingNode {
	reference := func(node *drawingNode, kinds ...string) string {
		for _, kind := range kinds {
			if part := node.child(kind); part != nil {
				for _, f := range part.all("f") {
					var value string
					if xml.Unmarshal(data[f.start:f.end], &value) == nil {
						return strings.TrimPrefix(value, "=")
					}
				}
			}
		}
		return ""
	}
	var best *drawingNode
	bestScore := -1
	for _, node := range nodes {
		if used[node] {
			continue
		}
		if series.Type != "" {
			plot := node.parent.name
			if series.Type == "line" && plot != "lineChart" && plot != "line3DChart" {
				continue
			}
			if series.Type == "column" && plot != "barChart" && plot != "bar3DChart" {
				continue
			}
		}
		score := 0
		if reference(node, "val", "yVal") == series.Values {
			score += 8
		}
		if reference(node, "tx") == series.Name {
			score += 4
		}
		if reference(node, "cat", "xVal") == series.Categories {
			score += 2
		}
		if score > bestScore {
			best, bestScore = node, score
		}
	}
	if best != nil {
		used[best] = true
	}
	return best
}
func preserveChartXML(c *model.Chart, generated []byte) ([]byte, error) {
	if !CanPreserveChart(c) {
		return restoreChartAxisSettings(restoreChartAxisTitles(restoreGeneratedSeriesStyles(generated, c), c), c, &model.Chart{}), nil
	}
	source := []byte(c.Source.XML)
	old := c.Source.Baseline
	root, err := drawingNodes(source)
	if err != nil {
		return nil, err
	}
	genRoot, err := drawingNodes(generated)
	if err != nil {
		return nil, err
	}
	charts := root.all("chart")
	if len(charts) != 1 {
		return nil, fmt.Errorf("無法更新圖表格式")
	}
	chart := charts[0]
	changes := []chartReplacement{}
	if c.Title != old.Title || c.AutoTitle != old.AutoTitle {
		title := ""
		if c.Title != "" {
			title = chartTitleText(chart.child("title"), source, c.Title)
		}
		changes = append(changes, chartReplacement{"title", title}, chartReplacement{"autoTitleDeleted", chartBoolXML("autoTitleDeleted", c.Title == "" && !c.AutoTitle)})
	}
	if c.Legend != old.Legend {
		legend := ""
		node := chart.child("legend")
		if c.Legend != "none" {
			pos := map[string]string{"right": "r", "top": "t", "bottom": "b"}[c.Legend]
			if node != nil {
				legend = replaceChartChildren(source, node, []chartReplacement{{"legendPos", chartVal("legendPos", pos)}}, []string{"legendPos", "legendEntry", "layout", "overlay", "spPr", "txPr", "extLst"})
			} else {
				legend = chartElement("legend", chartVal("legendPos", pos))
			}
		}
		changes = append(changes, chartReplacement{"legend", legend})
	}
	// Apply edits to independent subtrees first, then replace chart-level children.
	edits := []drawingEdit{}
	sers := root.all("ser")
	sort.SliceStable(sers, func(i, j int) bool {
		a, _ := strconv.Atoi(sers[i].child("idx").attr("val"))
		b, _ := strconv.Atoi(sers[j].child("idx").attr("val"))
		return a < b
	})
	genSers := genRoot.all("ser")
	used, genUsed := map[*drawingNode]bool{}, map[*drawingNode]bool{}
	for i, s := range c.Series {
		ser := matchChartSeries(sers, source, old.Series[i], used)
		genSer := matchChartSeries(genSers, generated, s, genUsed)
		if ser == nil || genSer == nil {
			return nil, fmt.Errorf("無法更新圖表格式")
		}
		if reflect.DeepEqual(s, old.Series[i]) {
			continue
		}
		kind := s.Type
		if kind == "" {
			kind = c.Type
		}
		edits = append(edits, drawingEdit{ser.start, ser.end, patchChartReferences(source, old.Series[i], s, genSer, generated, ser, kind)})
	}
	edits = append(edits, chartAxisTitleEdits(source, root, c, old)...)
	source = applyDrawingEdits(source, edits)
	root, _ = drawingNodes(source)
	chart = root.all("chart")[0]
	patched := replaceChartChildren(source, chart, changes, chartChildOrder)
	return restoreChartAxisSettings(restoreChartPlotStyle(applyDrawingEdits(source, []drawingEdit{{chart.start, chart.end, patched}}), c, old), c, old), nil
}

func (w *writer) preserveChartPart(name string, c *model.Chart, generated []byte) error {
	data, err := preserveChartXML(c, generated)
	if err != nil {
		return err
	}
	w.f.Pkg.Store(name, data)
	if !CanPreserveChart(c) || len(c.Source.Resources) == 0 {
		return nil
	}
	destinations := map[string]string{c.Source.Part: name}
	prefix := strings.TrimSuffix(name, ".xml") + "-assets"
	for original, resource := range c.Source.Resources {
		if strings.HasSuffix(original, ".rels") {
			continue
		}
		digest := sha256.Sum256(resource.Data)
		destinations[original] = path.Join(prefix, fmt.Sprintf("%x-%s", digest[:8], path.Base(original)))
	}
	for original, resource := range c.Source.Resources {
		dest := destinations[original]
		payload := resource.Data
		if strings.HasSuffix(original, ".rels") {
			base := strings.TrimSuffix(strings.Replace(original, "/_rels/", "/", 1), ".rels")
			destBase := destinations[base]
			if destBase == "" {
				continue
			}
			dest = chartRelsPart(destBase)
			root, err := drawingNodes(payload)
			if err != nil {
				return err
			}
			edits := []drawingEdit{}
			for _, rel := range root.all("Relationship") {
				if rel.attr("TargetMode") == "External" {
					continue
				}
				target := chartRelTarget(base, rel.attr("Target"))
				if mapped := destinations[target]; mapped != "" {
					relative, err := filepath.Rel(path.Dir(destBase), mapped)
					if err != nil {
						return err
					}
					edits = append(edits, drawingAttribute(payload, rel, "Target", filepath.ToSlash(relative)))
				}
			}
			payload = applyDrawingEdits(payload, edits)
			w.f.Relationships.Delete(dest)
		}
		if dest == "" {
			continue
		}
		w.f.Pkg.Store(dest, payload)
		if resource.ContentType != "" && !strings.HasSuffix(dest, ".rels") && w.f.ContentTypes != nil && len(w.f.ContentTypes.Overrides) > 0 {
			entry := w.f.ContentTypes.Overrides[0]
			entry.PartName = "/" + dest
			entry.ContentType = resource.ContentType
			w.f.ContentTypes.Overrides = append(w.f.ContentTypes.Overrides, entry)
		}
	}
	return nil
}

// Axis direction also handles scatter charts (two value axes) and bar charts.
func chartAxisTitleEdits(data []byte, root *drawingNode, c, old *model.Chart) []drawingEdit {
	edits := []drawingEdit{}
	seen := map[string]bool{}
	for _, kind := range []string{"catAx", "dateAx", "valAx"} {
		for _, node := range root.all(kind) {
			direction := node.child("axPos").attr("val")
			horizontal := direction == "b" || direction == "t"
			key := "y"
			text, prior := c.YTitle, old.YTitle
			if horizontal {
				key = "x"
				text, prior = c.XTitle, old.XTitle
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			if text == prior {
				continue
			}
			title := ""
			if text != "" {
				title = chartTitleText(node.child("title"), data, text)
			}
			patched := replaceChartChildren(data, node, []chartReplacement{{"title", title}}, []string{"axId", "scaling", "delete", "axPos", "majorGridlines", "minorGridlines", "title", "numFmt", "majorTickMark", "minorTickMark", "tickLblPos", "spPr", "txPr", "crossAx", "crosses", "crossesAt", "auto", "lblAlgn", "lblOffset", "tickLblSkip", "tickMarkSkip", "noMultiLvlLbl", "extLst"})
			edits = append(edits, drawingEdit{node.start, node.end, patched})
		}
	}
	return edits
}
func restoreChartAxisTitles(data []byte, c *model.Chart) []byte {
	root, err := drawingNodes(data)
	if err != nil {
		return data
	}
	return applyDrawingEdits(data, chartAxisTitleEdits(data, root, c, &model.Chart{}))
}
