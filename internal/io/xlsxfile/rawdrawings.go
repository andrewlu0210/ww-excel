package xlsxfile

import (
	"archive/zip"
	"io"
	"path"
	"strings"

	"ww-excel/internal/model"
)

// Pixels per EMU (English Metric Unit) at 96 dpi.
const emuPerPx = 9525

// rawAnchor is where a drawing object sits, from the drawing XML.
type rawAnchor struct {
	Shape   *drawingShape `xml:"sp"`
	FromCol int           `xml:"from>col"`
	FromCX  int64         `xml:"from>colOff"`
	FromRow int           `xml:"from>row"`
	FromRY  int64         `xml:"from>rowOff"`
	ToCol   int           `xml:"to>col"`
	ToCX    int64         `xml:"to>colOff"`
	ToRow   int           `xml:"to>row"`
	ToRY    int64         `xml:"to>rowOff"`
	Ext     struct {
		CX int64 `xml:"cx,attr"`
		CY int64 `xml:"cy,attr"`
	} `xml:"ext"`
	Chart struct {
		RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	} `xml:"graphicFrame>graphic>graphicData>chart"`
	Blip struct {
		RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
	} `xml:"pic>blipFill>blip"`
	twoCell bool
}

// readDrawing reads the charts and pictures of a sheet's drawing part
// (excelize cannot read charts back).
func readDrawing(parts map[string]*zip.File, drawing string, sh *model.Sheet) {
	if drawing == "" {
		return
	}
	theme := drawingTheme(parts)
	for _, a := range readAnchors(parts, drawing) {
		obj := model.Object{Anchor: model.Anchor{Row: a.FromRow, Col: a.FromCol, DX: float64(a.FromCX) / emuPerPx, DY: float64(a.FromRY) / emuPerPx}}
		obj.W, obj.H = anchorSize(&sh.Layout, a)
		switch {
		case a.Shape != nil:
			if ext := a.Shape.Properties.Transform.Ext; ext.CX > 0 && ext.CY > 0 {
				obj.W, obj.H = ext.CX/emuPerPx, ext.CY/emuPerPx
			}
			obj.Shape = a.Shape.model(theme, obj.W, obj.H)
			if obj.Shape == nil {
				continue
			}
			obj.Kind = model.ObjectShape
		case a.Chart.RID != "":
			c := readChart(parts, relTarget(parts, drawing, a.Chart.RID))
			if c == nil {
				continue
			}
			obj.Kind, obj.Chart = model.ObjectChart, c
		case a.Blip.RID != "":
			media := relTarget(parts, drawing, a.Blip.RID)
			data := readPart(parts, media)
			if data == nil {
				continue
			}
			obj.Kind = model.ObjectImage
			obj.Image = &model.Image{Data: data, Ext: strings.ToLower(path.Ext(media))}
		default:
			continue
		}
		obj.ID = sh.Layout.NextObjectID()
		sh.Layout.Objects = append(sh.Layout.Objects, obj)
	}
}

// relTarget resolves a relationship of part (by its .rels file).
func relTarget(parts map[string]*zip.File, part, rid string) string {
	dir, base := path.Split(part)
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if decodePart(parts, dir+"_rels/"+base+".rels", &rels) != nil {
		return ""
	}
	for _, r := range rels.Rels {
		if r.ID == rid {
			if strings.HasPrefix(r.Target, "/") {
				return strings.TrimPrefix(r.Target, "/")
			}
			return path.Join(dir, r.Target)
		}
	}
	return ""
}

func readPart(parts map[string]*zip.File, name string) []byte {
	f, ok := parts[name]
	if !ok {
		return nil
	}
	r, err := f.Open()
	if err != nil {
		return nil
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	return data
}

func readAnchors(parts map[string]*zip.File, drawing string) []rawAnchor {
	var doc struct {
		Two []rawAnchor `xml:"twoCellAnchor"`
		One []rawAnchor `xml:"oneCellAnchor"`
	}
	if decodePart(parts, drawing, &doc) != nil {
		return nil
	}
	for i := range doc.Two {
		doc.Two[i].twoCell = true
	}
	return append(doc.Two, doc.One...)
}

// anchorSize computes an object's size in pixels.
func anchorSize(l *model.Layout, a rawAnchor) (float64, float64) {
	if !a.twoCell {
		return float64(a.Ext.CX) / emuPerPx, float64(a.Ext.CY) / emuPerPx
	}
	w := -float64(a.FromCX) / emuPerPx
	for c := a.FromCol; c < a.ToCol; c++ {
		w += l.ColWidth(c)
	}
	w += float64(a.ToCX) / emuPerPx
	h := -float64(a.FromRY) / emuPerPx
	for r := a.FromRow; r < a.ToRow; r++ {
		h += l.RowHeight(r)
	}
	h += float64(a.ToRY) / emuPerPx
	return max(w, 20), max(h, 20)
}

// The parts of chartN.xml we understand.
type rawChartBool struct {
	Val bool `xml:"val,attr"`
}
type rawChartLabels struct {
	RawXML string `xml:",innerxml"`
	Format struct {
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmt"`
	ShowValue   rawChartBool `xml:"showVal"`
	ShowPercent rawChartBool `xml:"showPercent"`
	Font        drawingFont  `xml:"txPr>p>pPr>defRPr"`
	Points      []struct {
		Index struct {
			Val int `xml:"val,attr"`
		} `xml:"idx"`
		Delete rawChartBool `xml:"delete"`
		Layout struct {
			W struct {
				Val float64 `xml:"val,attr"`
			} `xml:"w"`
			H struct {
				Val float64 `xml:"val,attr"`
			} `xml:"h"`
		} `xml:"extLst>ext>layout>manualLayout"`
	} `xml:"dLbl"`
}

func (l *rawChartLabels) model(theme map[string]string) *model.ChartLabels {
	if l == nil {
		return nil
	}
	labels := &model.ChartLabels{Format: l.Format.Code, ShowValue: l.ShowValue.Val, ShowPercent: l.ShowPercent.Val,
		FontSize: xmlFloat(l.Font.Size) / 100, Color: l.Font.Fill.color(theme), RawXML: l.RawXML, Points: map[int]model.ChartLabelPoint{}}
	for _, p := range l.Points {
		// Excel uses a plot-sized manual label box to center a doughnut KPI.
		labels.Points[p.Index.Val] = model.ChartLabelPoint{Hidden: p.Delete.Val, Center: p.Layout.W.Val > .9 && p.Layout.H.Val > .9}
	}
	return labels
}

type rawChartCategories struct {
	Count struct {
		Val int `xml:"val,attr"`
	} `xml:"ptCount"`
	Points []struct {
		Index int    `xml:"idx,attr"`
		Value string `xml:"v"`
	} `xml:"pt"`
}

type rawSeries struct {
	Categories *rawChartCategories `xml:"cat>strLit"`
	Fill       drawingFill         `xml:"spPr>solidFill"`
	Points     []struct {
		Index struct {
			Val int `xml:"val,attr"`
		} `xml:"idx"`
		Fill drawingFill `xml:"spPr>solidFill"`
	} `xml:"dPt"`
	Labels *rawChartLabels `xml:"dLbls"`
	Name   string          `xml:"tx>strRef>f"`
	NameV  string          `xml:"tx>v"`
	CatStr string          `xml:"cat>strRef>f"`
	CatNum string          `xml:"cat>numRef>f"`
	Val    string          `xml:"val>numRef>f"`
	XStr   string          `xml:"xVal>strRef>f"`
	XNum   string          `xml:"xVal>numRef>f"`
	Y      string          `xml:"yVal>numRef>f"`
}

type rawAxisNumber struct {
	Val float64 `xml:"val,attr"`
}
type rawChartAxis struct {
	Format struct {
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmt"`
	Scaling struct {
		Minimum *rawAxisNumber `xml:"min"`
		Maximum *rawAxisNumber `xml:"max"`
	} `xml:"scaling"`
	MajorUnit rawAxisNumber `xml:"majorUnit"`
	Delete    rawChartBool  `xml:"delete"`
	MajorTick struct {
		Val string `xml:"val,attr"`
	} `xml:"majorTickMark"`
	TickLabels struct {
		Val string `xml:"val,attr"`
	} `xml:"tickLblPos"`
	GridLines *struct{} `xml:"majorGridlines"`
	Line      struct {
		NoFill *struct{}   `xml:"noFill"`
		Fill   drawingFill `xml:"solidFill"`
	} `xml:"spPr>ln"`
}

func (a *rawChartAxis) model(theme map[string]string) *model.ChartAxis {
	if a == nil {
		return nil
	}
	out := &model.ChartAxis{Format: a.Format.Code, MajorUnit: a.MajorUnit.Val, LineColor: a.Line.Fill.color(theme), LineHidden: a.Line.NoFill != nil, Hidden: a.Delete.Val, MajorTick: a.MajorTick.Val, GridLines: a.GridLines != nil, HideLabels: a.TickLabels.Val == "none"}
	if a.Scaling.Minimum != nil {
		out.Minimum = &a.Scaling.Minimum.Val
	}
	if a.Scaling.Maximum != nil {
		out.Maximum = &a.Scaling.Maximum.Val
	}
	return out
}

type rawPlot struct {
	HoleSize struct {
		Val int `xml:"val,attr"`
	} `xml:"holeSize"`
	BarDir struct {
		Val string `xml:"val,attr"`
	} `xml:"barDir"`
	Grouping struct {
		Val string `xml:"val,attr"`
	} `xml:"grouping"`
	Series []rawSeries     `xml:"ser"`
	Labels *rawChartLabels `xml:"dLbls"`
}

func readChart(parts map[string]*zip.File, name string) *model.Chart {
	var doc struct {
		Properties struct {
			NoFill *struct{}   `xml:"noFill"`
			Fill   drawingFill `xml:"solidFill"`
		} `xml:"spPr"`
		Chart struct {
			Title []string `xml:"title>tx>rich>p>r>t"`
			Plot  struct {
				ValueAxis    *rawChartAxis `xml:"valAx"`
				CategoryAxis *rawChartAxis `xml:"catAx"`
				Bar          []rawPlot     `xml:"barChart"`
				Bar3D        []rawPlot     `xml:"bar3DChart"`
				Line         []rawPlot     `xml:"lineChart"`
				Line3D       []rawPlot     `xml:"line3DChart"`
				Pie          []rawPlot     `xml:"pieChart"`
				Pie3D        []rawPlot     `xml:"pie3DChart"`
				Doughnut     []rawPlot     `xml:"doughnutChart"`
				Scatter      []rawPlot     `xml:"scatterChart"`
				Area         []rawPlot     `xml:"areaChart"`
				Area3D       []rawPlot     `xml:"area3DChart"`
			} `xml:"plotArea"`
			Legend *struct {
				Pos struct {
					Val string `xml:"val,attr"`
				} `xml:"legendPos"`
			} `xml:"legend"`
		} `xml:"chart"`
	}
	if name == "" || decodePart(parts, name, &doc) != nil {
		return nil
	}
	p := doc.Chart.Plot
	theme := drawingTheme(parts)
	c := &model.Chart{ValueAxis: p.ValueAxis.model(theme), CategoryAxis: p.CategoryAxis.model(theme), Title: strings.Join(doc.Chart.Title, ""), Legend: "none"}
	if doc.Properties.NoFill != nil {
		c.Background = "transparent"
	} else {
		c.Background = doc.Properties.Fill.color(drawingTheme(parts))
	}
	var plot *rawPlot
	pick := func(list []rawPlot, typ string) {
		if plot == nil && len(list) > 0 {
			plot, c.Type = &list[0], typ
		}
	}
	pick(append(p.Bar, p.Bar3D...), "column")
	pick(append(p.Line, p.Line3D...), "line")
	pick(append(p.Pie, p.Pie3D...), "pie")
	pick(p.Doughnut, "doughnut")
	pick(p.Scatter, "scatter")
	pick(append(p.Area, p.Area3D...), "area")
	if plot == nil {
		return nil
	}
	if c.Type == "column" && plot.BarDir.Val == "bar" {
		c.Type = "bar"
	}
	if c.Type == "doughnut" {
		c.HoleSize = plot.HoleSize.Val
	}
	c.Stacked = strings.HasPrefix(plot.Grouping.Val, "stacked") || plot.Grouping.Val == "percentStacked"
	for _, s := range plot.Series {
		cs := model.ChartSeries{Name: s.Name, Categories: s.CatStr, Values: s.Val, Color: s.Fill.color(theme), PointColors: map[int]string{}}
		if s.Categories != nil {
			count := min(max(0, s.Categories.Count.Val), 100000)
			cs.CategoryLabels = make([]string, count)
			for _, p := range s.Categories.Points {
				if p.Index >= 0 && p.Index < count {
					cs.CategoryLabels[p.Index] = p.Value
				}
			}
		}
		for _, p := range s.Points {
			if color := p.Fill.color(theme); color != "" {
				cs.PointColors[p.Index.Val] = color
			}
		}
		labels := s.Labels
		if labels == nil {
			labels = plot.Labels
		}
		cs.Labels = labels.model(theme)
		if cs.Categories == "" {
			cs.Categories = s.CatNum
		}
		if c.Type == "scatter" {
			cs.Values, cs.Categories = s.Y, s.XNum
			if cs.Categories == "" {
				cs.Categories = s.XStr
			}
		}
		if cs.Name == "" && s.NameV != "" {
			cs.Name = `"` + s.NameV + `"`
		}
		if cs.Values == "" {
			continue
		}
		c.Series = append(c.Series, cs)
	}
	if len(c.Series) == 0 {
		return nil
	}
	if doc.Chart.Legend != nil {
		switch doc.Chart.Legend.Pos.Val {
		case "b":
			c.Legend = "bottom"
		case "t":
			c.Legend = "top"
		default:
			c.Legend = "right"
		}
	}
	return c
}
