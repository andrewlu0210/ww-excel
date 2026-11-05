package xlsxfile

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"math"
	"path"
	"sort"
	"strings"

	"ww-excel/internal/model"
)

// Pixels per EMU (English Metric Unit) at 96 dpi.
const emuPerPx = 9525

// rawAnchor is where a drawing object sits, from the drawing XML.
type rawAnchor struct {
	Shape   *drawingShape `xml:"sp"`
	Conn    *drawingShape `xml:"cxnSp"`
	Group   *rawGroup     `xml:"grpSp"`
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

// rawGroup is a DrawingML group: children are laid out in the group's own
// child coordinate space (chOff/chExt) and mapped onto the group's frame.
type rawGroup struct {
	Transform drawingTransform `xml:"grpSpPr>xfrm"`
	Items     []rawItem        `xml:",any"`
}

// rawPicture is a picture inside a group.
type rawPicture struct {
	Transform drawingTransform `xml:"spPr>xfrm"`
	Blip      struct {
		RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
	} `xml:"blipFill>blip"`
}

// rawItem is one child of a group, in document (z) order.
type rawItem struct {
	Shape   *drawingShape
	Picture *rawPicture
	Group   *rawGroup
}

func (item *rawItem) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	switch start.Name.Local {
	case "sp", "cxnSp":
		item.Shape = &drawingShape{}
		return d.DecodeElement(item.Shape, &start)
	case "pic":
		item.Picture = &rawPicture{}
		return d.DecodeElement(item.Picture, &start)
	case "grpSp":
		item.Group = &rawGroup{}
		return d.DecodeElement(item.Group, &start)
	}
	return d.Skip()
}

// pxRect is a rectangle in pixels relative to the top-level anchor's corner.
type pxRect struct{ x, y, w, h float64 }

// childRect maps a child frame (EMU in the group's child space) into the
// group's pixel frame.
func (g *rawGroup) childRect(t drawingTransform, frame pxRect) pxRect {
	sx, sy := 1.0/emuPerPx, 1.0/emuPerPx
	if t0 := g.Transform; t0.ChildExt.CX > 0 && t0.ChildExt.CY > 0 {
		sx, sy = frame.w/t0.ChildExt.CX, frame.h/t0.ChildExt.CY
	}
	return pxRect{frame.x + (t.Off.X-g.Transform.ChildOff.X)*sx, frame.y + (t.Off.Y-g.Transform.ChildOff.Y)*sy, t.Ext.CX * sx, t.Ext.CY * sy}
}

// readDrawing reads the charts, pictures and shapes of a sheet's drawing part
// (excelize cannot read charts back).
func readDrawing(parts map[string]*zip.File, drawing string, sh *model.Sheet) {
	if drawing == "" {
		return
	}
	theme := drawingTheme(parts)
	sources := shapeSources(readPart(parts, drawing))
	add := func(obj model.Object) {
		if obj.Shape != nil {
			obj.Shape.RawXML = sources[obj.Shape.SourceID]
		}
		obj.ID = sh.Layout.NextObjectID()
		sh.Layout.Objects = append(sh.Layout.Objects, obj)
	}
	for _, a := range readAnchors(parts, drawing) {
		origin := model.Anchor{Row: a.FromRow, Col: a.FromCol, DX: float64(a.FromCX) / emuPerPx, DY: float64(a.FromRY) / emuPerPx}
		obj := model.Object{Anchor: origin}
		obj.W, obj.H = anchorSize(&sh.Layout, a)
		switch {
		case a.Group != nil:
			readGroup(parts, drawing, &sh.Layout, theme, a.Group, origin, pxRect{0, 0, obj.W, obj.H}, add)
			continue
		case a.Shape != nil || a.Conn != nil:
			raw := a.Shape
			if raw == nil {
				raw = a.Conn
			}
			w, h := obj.W, obj.H
			if ext := raw.Properties.Transform.Ext; ext.CX > 0 && ext.CY > 0 {
				w, h = ext.CX/emuPerPx, ext.CY/emuPerPx
			}
			frame := pxRect{0, 0, w, h}
			if raw.Properties.Transform.Rot != 0 {
				// A rotated shape's anchor spans its rotated bounding box, so the
				// unrotated frame is centered in it.
				frame.x, frame.y = (obj.W-w)/2, (obj.H-h)/2
			}
			addShape(&sh.Layout, theme, raw, origin, frame, false, add)
			continue
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
		add(obj)
	}
}

// readGroup flattens a group into separate read-only objects (children keep
// their z order). frame is the group's pixel frame relative to origin.
func readGroup(parts map[string]*zip.File, drawing string, l *model.Layout, theme map[string]string, g *rawGroup, origin model.Anchor, frame pxRect, add func(model.Object)) {
	for _, item := range g.Items {
		switch {
		case item.Group != nil:
			readGroup(parts, drawing, l, theme, item.Group, origin, g.childRect(item.Group.Transform, frame), add)
		case item.Shape != nil:
			addShape(l, theme, item.Shape, origin, g.childRect(item.Shape.Properties.Transform, frame), true, add)
		case item.Picture != nil:
			media := relTarget(parts, drawing, item.Picture.Blip.RID)
			data := readPart(parts, media)
			if data == nil {
				continue
			}
			r := g.childRect(item.Picture.Transform, frame)
			add(model.Object{Kind: model.ObjectImage, Anchor: shiftAnchor(l, origin, r.x, r.y), W: r.w, H: r.h, ReadOnly: true,
				Image: &model.Image{Data: data, Ext: strings.ToLower(path.Ext(media))}})
		}
	}
}

// addShape converts one DrawingML shape placed at r (relative to origin).
func addShape(l *model.Layout, theme map[string]string, raw *drawingShape, origin model.Anchor, r pxRect, readOnly bool, add func(model.Object)) bool {
	shape := raw.model(theme, r.w, r.h)
	if shape == nil {
		return false
	}
	t := raw.Properties.Transform
	shape.Rot, shape.FlipH, shape.FlipV = math.Mod(t.Rot/60000, 360), t.flipH(), t.flipV()
	box := r
	if shape.Rot != 0 || r.w < 1 || r.h < 1 {
		// The object box is the rotated bounding box (lines may have no extent).
		rad := shape.Rot * math.Pi / 180
		sin, cos := math.Abs(math.Sin(rad)), math.Abs(math.Cos(rad))
		pad := shape.LineWidth + 2
		box.w, box.h = r.w*cos+r.h*sin+pad, r.w*sin+r.h*cos+pad
		box.x, box.y = r.x+r.w/2-box.w/2, r.y+r.h/2-box.h/2
		shape.W, shape.H = r.w, r.h
	}
	add(model.Object{Kind: model.ObjectShape, Anchor: shiftAnchor(l, origin, box.x, box.y), W: box.w, H: box.h, Shape: shape, ReadOnly: readOnly})
	return true
}

// shiftAnchor moves an anchor by (dx, dy) pixels, normalizing the offsets
// into the cell that contains the point.
func shiftAnchor(l *model.Layout, a model.Anchor, dx, dy float64) model.Anchor {
	a.DX, a.DY = a.DX+dx, a.DY+dy
	for i := 0; i < model.MaxCols && a.DX >= l.ColWidth(a.Col); i++ {
		w := l.ColWidth(a.Col)
		a.DX -= w
		a.Col++
	}
	for a.DX < 0 && a.Col > 0 {
		a.Col--
		a.DX += l.ColWidth(a.Col)
	}
	for i := 0; i < 100000 && a.DY >= l.RowHeight(a.Row); i++ {
		a.DY -= l.RowHeight(a.Row)
		a.Row++
	}
	for a.DY < 0 && a.Row > 0 {
		a.Row--
		a.DY += l.RowHeight(a.Row)
	}
	return a
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
type rawLabelLayout struct {
	X struct {
		Val float64 `xml:"val,attr"`
	} `xml:"x"`
	Y struct {
		Val float64 `xml:"val,attr"`
	} `xml:"y"`
	W struct {
		Val float64 `xml:"val,attr"`
	} `xml:"w"`
	H struct {
		Val float64 `xml:"val,attr"`
	} `xml:"h"`
	XMode struct {
		Val string `xml:"val,attr"`
	} `xml:"xMode"`
	YMode struct {
		Val string `xml:"val,attr"`
	} `xml:"yMode"`
}

func (l *rawLabelLayout) model() *model.ChartLabelLayout {
	if l == nil {
		return nil
	}
	return &model.ChartLabelLayout{X: l.X.Val, Y: l.Y.Val, W: l.W.Val, H: l.H.Val, XMode: l.XMode.Val, YMode: l.YMode.Val}
}

type rawLabelFlags struct {
	Val bool `xml:"val,attr"`
}

func labelFlag(f *rawLabelFlags) *bool {
	if f == nil {
		return nil
	}
	v := f.Val
	return &v
}

type rawLabelText struct {
	Runs []struct {
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"rich>p"`
	Ref   string   `xml:"strRef>f"`
	Cache []string `xml:"strRef>strCache>pt>v"`
}
type rawChartLabels struct {
	Delete rawChartBool `xml:"delete"`
	RawXML string       `xml:",innerxml"`
	Format struct {
		Code string `xml:"formatCode,attr"`
	} `xml:"numFmt"`
	Position struct {
		Val string `xml:"val,attr"`
	} `xml:"dLblPos"`
	Separator    string       `xml:"separator"`
	ShowValue    rawChartBool `xml:"showVal"`
	ShowPercent  rawChartBool `xml:"showPercent"`
	ShowCategory rawChartBool `xml:"showCatName"`
	ShowSeries   rawChartBool `xml:"showSerName"`
	LeaderLines  rawChartBool `xml:"showLeaderLines"`
	Font         drawingFont  `xml:"txPr>p>pPr>defRPr"`
	Rotation     struct {
		Value float64 `xml:"rot,attr"`
	} `xml:"txPr>bodyPr"`
	Box struct {
		Fill drawingFill `xml:"solidFill"`
		Line struct {
			Fill drawingFill `xml:"solidFill"`
		} `xml:"ln"`
	} `xml:"spPr"`
	Points []struct {
		Index struct {
			Val int `xml:"val,attr"`
		} `xml:"idx"`
		Delete   rawChartBool `xml:"delete"`
		Position struct {
			Val string `xml:"val,attr"`
		} `xml:"dLblPos"`
		Format struct {
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmt"`
		Font         drawingFont     `xml:"txPr>p>pPr>defRPr"`
		Text         *rawLabelText   `xml:"tx"`
		Manual       *rawLabelLayout `xml:"layout>manualLayout"`
		Legacy       *rawLabelLayout `xml:"extLst>ext>layout>manualLayout"`
		ShowValue    *rawLabelFlags  `xml:"showVal"`
		ShowPercent  *rawLabelFlags  `xml:"showPercent"`
		ShowCategory *rawLabelFlags  `xml:"showCatName"`
		ShowSeries   *rawLabelFlags  `xml:"showSerName"`
	} `xml:"dLbl"`
}

func (l *rawChartLabels) model(theme map[string]string) *model.ChartLabels {
	if l == nil {
		return nil
	}
	labels := &model.ChartLabels{Disabled: l.Delete.Val, Format: l.Format.Code, Position: l.Position.Val, Separator: l.Separator, ShowValue: l.ShowValue.Val, ShowPercent: l.ShowPercent.Val, ShowCategory: l.ShowCategory.Val, ShowSeries: l.ShowSeries.Val, LeaderLines: l.LeaderLines.Val, FontSize: xmlFloat(l.Font.Size) / 100, FontName: l.Font.Latin.Name, Bold: chartBool(l.Font.Bold), Italic: chartBool(l.Font.Italic), Color: l.Font.Fill.color(theme), Rotation: l.Rotation.Value / 60000, Fill: l.Box.Fill.color(theme), Border: l.Box.Line.Fill.color(theme), RawXML: l.RawXML, Points: map[int]model.ChartLabelPoint{}}
	for _, p := range l.Points {
		point := model.ChartLabelPoint{Hidden: p.Delete.Val, Position: p.Position.Val, Format: p.Format.Code, FontName: p.Font.Latin.Name, FontSize: xmlFloat(p.Font.Size) / 100, Color: p.Font.Fill.color(theme), Layout: p.Manual.model(), ShowValue: labelFlag(p.ShowValue), ShowPercent: labelFlag(p.ShowPercent), ShowCategory: labelFlag(p.ShowCategory), ShowSeries: labelFlag(p.ShowSeries)}
		if p.Font.Bold != "" {
			v := chartBool(p.Font.Bold)
			point.Bold = &v
		}
		if p.Font.Italic != "" {
			v := chartBool(p.Font.Italic)
			point.Italic = &v
		}
		if p.Manual != nil && p.Manual.W.Val > .9 && p.Manual.H.Val > .9 {
			point.Center = true
			point.Layout = nil
		}
		if p.Legacy != nil && p.Legacy.W.Val > .9 && p.Legacy.H.Val > .9 {
			point.Center = true
		}
		if p.Text != nil {
			if p.Text.Ref != "" {
				point.Reference = p.Text.Ref
			} else {
				lines := []string{}
				for _, para := range p.Text.Runs {
					text := ""
					for _, r := range para.Runs {
						text += r.Text
					}
					lines = append(lines, text)
				}
				text := strings.Join(lines, "\n")
				point.CustomText = &text
			}
		}
		labels.Points[p.Index.Val] = point
	}
	if labels.ShowPercent && !labels.ShowValue && strings.Contains(labels.Format, "%") {
		labels.PercentFormat = labels.Format
	}
	labels.SourceConfig = labelConfig(labels)
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
	Index *struct {
		Val int `xml:"val,attr"`
	} `xml:"idx"`
	Line struct {
		Width float64     `xml:"w,attr"`
		Fill  drawingFill `xml:"solidFill"`
	} `xml:"spPr>ln"`
	Marker struct {
		Val string `xml:"val,attr"`
	} `xml:"marker>symbol"`
	Categories *rawChartCategories `xml:"cat>strLit"`
	Fill       drawingFill         `xml:"spPr>solidFill"`
	Points     []struct {
		Index struct {
			Val int `xml:"val,attr"`
		} `xml:"idx"`
		Appearance rawChartAppearance `xml:"spPr"`
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
	TitleText []string `xml:"title>tx>rich>p>r>t"`
	Position  struct {
		Val string `xml:"val,attr"`
	} `xml:"axPos"`
	Cross struct {
		Val int `xml:"val,attr"`
	} `xml:"crossAx"`
	ID struct {
		Val int `xml:"val,attr"`
	} `xml:"axId"`
	Font   drawingFont `xml:"txPr>p>pPr>defRPr"`
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
	out := &model.ChartAxis{FontSize: xmlFloat(a.Font.Size) / 100, FontName: a.Font.Latin.Name, Format: a.Format.Code, MajorUnit: a.MajorUnit.Val, LineColor: a.Line.Fill.color(theme), LineHidden: a.Line.NoFill != nil, Hidden: a.Delete.Val, MajorTick: a.MajorTick.Val, GridLines: a.GridLines != nil, HideLabels: a.TickLabels.Val == "none"}
	if a.Scaling.Minimum != nil {
		out.Minimum = &a.Scaling.Minimum.Val
	}
	if a.Scaling.Maximum != nil {
		out.Maximum = &a.Scaling.Maximum.Val
	}
	return out
}

type rawPlot struct {
	AxisIDs []struct {
		Val int `xml:"val,attr"`
	} `xml:"axId"`
	GapWidth *rawAxisNumber `xml:"gapWidth"`
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
		Font       drawingFont `xml:"txPr>p>pPr>defRPr"`
		Properties struct {
			NoFill *struct{}   `xml:"noFill"`
			Fill   drawingFill `xml:"solidFill"`
			Line   struct {
				NoFill *struct{}   `xml:"noFill"`
				Fill   drawingFill `xml:"solidFill"`
			} `xml:"ln"`
		} `xml:"spPr"`
		Chart struct {
			TitleEl *struct {
				Text *struct {
					Runs []string `xml:"rich>p>r>t"`
				} `xml:"tx"`
				Font drawingFont `xml:"txPr>p>pPr>defRPr"`
			} `xml:"title"`
			Plot struct {
				Manual       *rawChartLayout `xml:"layout>manualLayout"`
				ValueAxis    []rawChartAxis  `xml:"valAx"`
				CategoryAxis []rawChartAxis  `xml:"catAx"`
				DateAxis     []rawChartAxis  `xml:"dateAx"`
				Bar          []rawPlot       `xml:"barChart"`
				Bar3D        []rawPlot       `xml:"bar3DChart"`
				Line         []rawPlot       `xml:"lineChart"`
				Line3D       []rawPlot       `xml:"line3DChart"`
				Pie          []rawPlot       `xml:"pieChart"`
				Pie3D        []rawPlot       `xml:"pie3DChart"`
				Doughnut     []rawPlot       `xml:"doughnutChart"`
				Scatter      []rawPlot       `xml:"scatterChart"`
				Area         []rawPlot       `xml:"areaChart"`
				Area3D       []rawPlot       `xml:"area3DChart"`
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
	if len(p.CategoryAxis) == 0 {
		p.CategoryAxis = p.DateAxis
	}
	var valueAxis, categoryAxis *model.ChartAxis
	if len(p.ValueAxis) > 0 {
		valueAxis = p.ValueAxis[0].model(theme)
	}
	if len(p.CategoryAxis) > 0 {
		categoryAxis = p.CategoryAxis[0].model(theme)
	}
	c := &model.Chart{FontName: doc.Font.Latin.Name, ValueAxis: valueAxis, CategoryAxis: categoryAxis, Legend: "none"}
	c.PlotArea = p.Manual.model()
	for _, axis := range append(append(append([]rawChartAxis{}, p.CategoryAxis...), p.DateAxis...), p.ValueAxis...) {
		title := strings.Join(axis.TitleText, "")
		switch axis.Position.Val {
		case "b", "t":
			if c.XTitle == "" {
				c.XTitle = title
			}
		case "l", "r":
			if c.YTitle == "" {
				c.YTitle = title
			}
		}
	}

	if t := doc.Chart.TitleEl; t != nil {
		// A title without its own text shows the series name (single series).
		c.AutoTitle = t.Text == nil
		c.TitleSize = xmlFloat(t.Font.Size) / 100
		if t.Text != nil {
			c.Title = strings.Join(t.Text.Runs, "")
		}
	}
	if doc.Properties.Line.NoFill == nil {
		c.Border = doc.Properties.Line.Fill.color(theme)
	}
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
	if plot.GapWidth != nil {
		c.GapWidth = &plot.GapWidth.Val
	}
	if c.Type == "column" && plot.BarDir.Val == "bar" {
		c.Type = "bar"
	}
	if c.Type == "doughnut" {
		c.HoleSize = plot.HoleSize.Val
	}
	if c.Type == "scatter" {
		for i := range p.ValueAxis {
			axis := &p.ValueAxis[i]
			if axis.Position.Val == "b" || axis.Position.Val == "t" {
				c.CategoryAxis = axis.model(theme)
			} else {
				c.ValueAxis = axis.model(theme)
			}
		}
	}
	combo := len(p.Bar) > 0 && p.Bar[0].BarDir.Val != "bar" && len(p.Line) > 0
	primaryID := 0
	axisFor := func(plot *rawPlot, axes []rawChartAxis) *rawChartAxis {
		for _, id := range plot.AxisIDs {
			for i := range axes {
				if axes[i].ID.Val == id.Val {
					return &axes[i]
				}
			}
		}
		return nil
	}
	if combo {
		c.Type = "combo"
		primary := axisFor(plot, p.ValueAxis)
		for i := range p.ValueAxis {
			if p.ValueAxis[i].Position.Val == "l" {
				primary = &p.ValueAxis[i]
				break
			}
		}
		if primary != nil {
			primaryID = primary.ID.Val
			c.ValueAxis = primary.model(theme)
			for i := range p.CategoryAxis {
				if p.CategoryAxis[i].ID.Val == primary.Cross.Val {
					c.CategoryAxis = p.CategoryAxis[i].model(theme)
				}
			}
		}
	}
	var seriesOrder []int

	c.Stacked = strings.HasPrefix(plot.Grouping.Val, "stacked") || plot.Grouping.Val == "percentStacked"
	plots := []*rawPlot{plot}
	if combo {
		for i := range p.Line {
			plots = append(plots, &p.Line[i])
		}
	}
	for pi, current := range plots {
		seriesType, secondary := "", false
		if combo {
			seriesType = "column"
			if pi > 0 {
				seriesType = "line"
			}
			if axis := axisFor(current, p.ValueAxis); axis != nil && axis.ID.Val != primaryID {
				secondary = true
				c.SecondaryAxis = axis.model(theme)
				if cat := axisFor(current, p.CategoryAxis); cat != nil {
					c.SecondaryCategoryAxis = cat.model(theme)
				}
			}
		}
		for _, s := range current.Series {
			cs := model.ChartSeries{Type: seriesType, Secondary: secondary, LineWidth: s.Line.Width / emuPerPx, Marker: s.Marker.Val, Name: s.Name, Categories: s.CatStr, Values: s.Val, Color: s.Fill.color(theme), PointColors: map[int]string{}, PointStyles: map[int]model.ChartPointStyle{}}
			if seriesType == "line" || c.Type == "line" || c.Type == "scatter" {
				if color := s.Line.Fill.color(theme); color != "" {
					cs.Color = color
				}
			}
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
				if color := p.Appearance.Fill.color(theme); color != "" {
					cs.PointColors[p.Index.Val] = color
				}
			}
			for _, point := range s.Points {
				if style, ok := point.Appearance.model(theme); ok {
					cs.PointStyles[point.Index.Val] = style
					if style.Gradient != nil && len(style.Gradient.Stops) > 0 {
						cs.PointColors[point.Index.Val] = style.Gradient.Stops[len(style.Gradient.Stops)/2].Color
					}
				}
			}
			labels := s.Labels
			if labels == nil {
				labels = current.Labels
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
			order := len(seriesOrder)
			if s.Index != nil {
				order = s.Index.Val
			}
			seriesOrder = append(seriesOrder, order)
			c.Series = append(c.Series, cs)
		}
	}
	if combo {
		indices := make([]int, len(c.Series))
		for i := range indices {
			indices[i] = i
		}
		sort.SliceStable(indices, func(i, j int) bool { return seriesOrder[indices[i]] < seriesOrder[indices[j]] })
		sorted := make([]model.ChartSeries, len(indices))
		for i, index := range indices {
			sorted[i] = c.Series[index]
		}
		c.Series = sorted
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
	attachChartSource(parts, name, c)

	return c
}
