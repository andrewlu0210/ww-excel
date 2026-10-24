package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ww-excel/internal/command"
	"ww-excel/internal/document"
	"ww-excel/internal/model"
	"ww-excel/internal/printing"
)

// ---- Floating objects ----

type ChartSeriesDTO struct {
	Name       string `json:"name"`
	Categories string `json:"categories"`
	Values     string `json:"values"`
}

type ChartDTO struct {
	Type    string           `json:"type"`
	Title   string           `json:"title"`
	Series  []ChartSeriesDTO `json:"series"`
	Legend  string           `json:"legend"`
	XTitle  string           `json:"xTitle"`
	YTitle  string           `json:"yTitle"`
	Stacked bool             `json:"stacked"`
}

// ObjectDTO is a chart or picture. Pictures are fetched separately
// (ObjectImage); ImageKey changes when the picture does.
type ObjectDTO struct {
	ID       int       `json:"id"`
	Kind     string    `json:"kind"`
	Row      int       `json:"row"`
	Col      int       `json:"col"`
	DX       float64   `json:"dx"`
	DY       float64   `json:"dy"`
	W        float64   `json:"w"`
	H        float64   `json:"h"`
	Chart    *ChartDTO `json:"chart"`
	ImageKey string    `json:"imageKey"`
}

func chartDTO(c *model.Chart) *ChartDTO {
	if c == nil {
		return nil
	}
	out := &ChartDTO{Type: c.Type, Title: c.Title, Legend: c.Legend, XTitle: c.XTitle, YTitle: c.YTitle, Stacked: c.Stacked, Series: []ChartSeriesDTO{}}
	for _, s := range c.Series {
		out.Series = append(out.Series, ChartSeriesDTO{Name: refText(s.Name), Categories: refText(s.Categories), Values: refText(s.Values)})
	}
	return out
}

// refText shows a stored reference with its "=" (literals stay quoted).
func refText(s string) string {
	if s == "" || strings.HasPrefix(s, `"`) {
		return s
	}
	return "=" + s
}

func (c *ChartDTO) model() *model.Chart {
	if c == nil {
		return nil
	}
	out := &model.Chart{Type: c.Type, Title: c.Title, Legend: c.Legend, XTitle: c.XTitle, YTitle: c.YTitle, Stacked: c.Stacked}
	strip := func(f string) string { return strings.TrimPrefix(strings.TrimSpace(f), "=") }
	for _, s := range c.Series {
		out.Series = append(out.Series, model.ChartSeries{Name: strip(s.Name), Categories: strip(s.Categories), Values: strip(s.Values)})
	}
	return out
}

func objectDTO(o model.Object) ObjectDTO {
	dto := ObjectDTO{ID: o.ID, Kind: o.Kind, Row: o.Anchor.Row, Col: o.Anchor.Col, DX: o.Anchor.DX, DY: o.Anchor.DY, W: o.W, H: o.H, Chart: chartDTO(o.Chart)}
	if o.Image != nil {
		h := fnv.New64a()
		h.Write(o.Image.Data)
		dto.ImageKey = hex.EncodeToString(h.Sum(nil))
	}
	return dto
}

// ChartFromSelection suggests a chart for the selected data.
func (s *SheetService) ChartFromSelection(ctx context.Context, sheet int, sel RangeDTO, typ string) (*ChartDTO, error) {
	var out *ChartDTO
	var err error
	s.at(ctx).read(func(d *document.Document) {
		rg := sel.model()
		if rg.R0 == rg.R1 && rg.C0 == rg.C1 {
			rg = d.CurrentRegion(sheet, rg.R0, rg.C0)
		}
		var c *model.Chart
		if c, err = d.ChartFromRange(sheet, rg, typ); err == nil {
			out = chartDTO(c)
		}
	})
	return out, err
}

// AddChart places a chart at a cell and returns its ID.
func (s *SheetService) AddChart(ctx context.Context, sheet, row, col int, w, h float64, chart ChartDTO) (int, error) {
	var id int
	err := s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		cmd, nid, err := d.AddObjectCommand(sheet, model.Object{Kind: model.ObjectChart, Anchor: model.Anchor{Row: row, Col: col}, W: w, H: h, Chart: chart.model()})
		id = nid
		return cmd, err
	})
	return id, err
}

// UpdateObject moves, resizes or edits an object. A nil chart keeps the
// current one.
func (s *SheetService) UpdateObject(ctx context.Context, sheet int, o ObjectDTO, label string) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		cur := d.Workbook().Sheets[sheet].Layout.ObjectByID(o.ID)
		if cur == nil {
			return nil, errors.New("物件不存在")
		}
		next := *cur
		next.Anchor = model.Anchor{Row: max(o.Row, 0), Col: max(o.Col, 0), DX: max(o.DX, 0), DY: max(o.DY, 0)}
		next.W, next.H = max(o.W, 10), max(o.H, 10)
		if o.Chart != nil {
			next.Chart = o.Chart.model()
		}
		if label == "" {
			label = "移動物件"
		}
		return d.UpdateObjectCommand(sheet, next, label)
	})
}

func (s *SheetService) DeleteObjects(ctx context.Context, sheet int, ids []int) error {
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.DeleteObjectCommand(sheet, ids)
	})
}

// ObjectImage returns a picture as a data URL.
func (s *SheetService) ObjectImage(ctx context.Context, sheet, id int) string {
	var out string
	s.at(ctx).read(func(d *document.Document) {
		if sh := sheetAt(d, sheet); sh != nil {
			if o := sh.Layout.ObjectByID(id); o != nil && o.Image != nil {
				out = dataURL(o.Image)
			}
		}
	})
	return out
}

func dataURL(img *model.Image) string {
	mime := "image/png"
	switch strings.ToLower(img.Ext) {
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".gif":
		mime = "image/gif"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
}

type SeriesDataDTO struct {
	Name   string     `json:"name"`
	Values []*float64 `json:"values"`
	X      []*float64 `json:"x"`
}

type ChartDataDTO struct {
	Categories []string        `json:"categories"`
	Series     []SeriesDataDTO `json:"series"`
}

// ChartData evaluates a chart's series for drawing.
func (s *SheetService) ChartData(ctx context.Context, sheet, id int) ChartDataDTO {
	out := ChartDataDTO{Categories: []string{}, Series: []SeriesDataDTO{}}
	s.at(ctx).read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			return
		}
		o := sh.Layout.ObjectByID(id)
		if o == nil || o.Chart == nil {
			return
		}
		data := d.ChartData(sheet, o.Chart, shownText(d.Workbook()))
		out.Categories = append(out.Categories, data.Categories...)
		for _, sd := range data.Series {
			out.Series = append(out.Series, SeriesDataDTO{Name: sd.Name, Values: orEmpty(sd.Values), X: orEmpty(sd.X)})
		}
	})
	return out
}

// ChartPreview evaluates a chart that is not placed yet (insert dialog).
func (s *SheetService) ChartPreview(ctx context.Context, sheet int, chart ChartDTO) ChartDataDTO {
	out := ChartDataDTO{Categories: []string{}, Series: []SeriesDataDTO{}}
	s.at(ctx).read(func(d *document.Document) {
		if sheetAt(d, sheet) == nil {
			return
		}
		data := d.ChartData(sheet, chart.model(), shownText(d.Workbook()))
		out.Categories = append(out.Categories, data.Categories...)
		for _, sd := range data.Series {
			out.Series = append(out.Series, SeriesDataDTO{Name: sd.Name, Values: orEmpty(sd.Values), X: orEmpty(sd.X)})
		}
	})
	return out
}

func orEmpty(v []*float64) []*float64 {
	if v == nil {
		return []*float64{}
	}
	return v
}

// InsertPicture asks for a picture file and places it at a cell, at
// most 480 px wide.
func (ds *DocumentService) InsertPicture(ctx context.Context, sheet, row, col int) error {
	path, err := ds.dialogs.PickImage()
	if err != nil || path == "" {
		return err
	}
	return ds.AddPictureFile(ctx, sheet, row, col, path)
}

// AddPictureFile places a picture file at a cell (also used by tests).
func (ds *DocumentService) AddPictureFile(ctx context.Context, sheet, row, col int, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cfg, _, err := image.DecodeConfig(strings.NewReader(string(data)))
	if err != nil {
		return errors.New("無法辨識的圖片格式")
	}
	w, h := float64(cfg.Width), float64(cfg.Height)
	if w > 480 {
		w, h = 480, h*480/w
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	return ds.sheetChange(ctx, func(d *document.Document) (command.Command, error) {
		cmd, _, err := d.AddObjectCommand(sheet, model.Object{Kind: model.ObjectImage, Anchor: model.Anchor{Row: row, Col: col}, W: w, H: h, Image: &model.Image{Data: data, Ext: ext}})
		return cmd, err
	})
}

// ---- Page setup and printing ----

type PageSetupDTO struct {
	Paper      string    `json:"paper"`
	Landscape  bool      `json:"landscape"`
	Top        float64   `json:"top"`
	Bottom     float64   `json:"bottom"`
	Left       float64   `json:"left"`
	Right      float64   `json:"right"`
	Header     float64   `json:"header"`
	Footer     float64   `json:"footer"`
	Scale      int       `json:"scale"`
	FitWide    int       `json:"fitWide"`
	FitTall    int       `json:"fitTall"`
	PrintArea  *RangeDTO `json:"printArea"`
	Gridlines  bool      `json:"gridlines"`
	Headings   bool      `json:"headings"`
	CenterH    bool      `json:"centerH"`
	CenterV    bool      `json:"centerV"`
	HeaderText string    `json:"headerText"`
	FooterText string    `json:"footerText"`
}

func pageSetupOf(sh *model.Sheet) model.PageSetup {
	if sh.Layout.PageSetup != nil {
		return *sh.Layout.PageSetup
	}
	return model.DefaultPageSetup()
}

// PageSetup returns a sheet's print settings.
func (s *SheetService) PageSetup(ctx context.Context, sheet int) PageSetupDTO {
	var out PageSetupDTO
	s.at(ctx).read(func(d *document.Document) {
		ps := model.DefaultPageSetup()
		if sh := sheetAt(d, sheet); sh != nil {
			ps = pageSetupOf(sh)
		}
		out = PageSetupDTO{Paper: ps.Paper, Landscape: ps.Landscape, Top: ps.Top, Bottom: ps.Bottom, Left: ps.Left, Right: ps.Right,
			Header: ps.Header, Footer: ps.Footer, Scale: ps.Scale, FitWide: ps.FitWide, FitTall: ps.FitTall, Gridlines: ps.Gridlines,
			Headings: ps.Headings, CenterH: ps.CenterH, CenterV: ps.CenterV, HeaderText: ps.HeaderText, FooterText: ps.FooterText}
		if ps.PrintArea != nil {
			a := rangeDTO(*ps.PrintArea)
			out.PrintArea = &a
		}
	})
	return out
}

func (s *SheetService) SetPageSetup(ctx context.Context, sheet int, p PageSetupDTO) error {
	ps := model.PageSetup{Paper: p.Paper, Landscape: p.Landscape, Top: p.Top, Bottom: p.Bottom, Left: p.Left, Right: p.Right,
		Header: p.Header, Footer: p.Footer, Scale: p.Scale, FitWide: p.FitWide, FitTall: p.FitTall, Gridlines: p.Gridlines,
		Headings: p.Headings, CenterH: p.CenterH, CenterV: p.CenterV, HeaderText: p.HeaderText, FooterText: p.FooterText}
	if p.PrintArea != nil {
		a := p.PrintArea.model().Clamp()
		ps.PrintArea = &a
	}
	return s.exec(ctx, sheet, func(d *document.Document) (command.Command, error) {
		return d.PageSetupCommand(sheet, ps)
	})
}

// printSource adapts a sheet to the PDF renderer. Charts come as PNG
// pictures drawn by the frontend.
type printSource struct {
	wb     *model.Workbook
	sh     *model.Sheet
	look   *sheetLook
	charts map[int][]byte
}

func (p printSource) View(row, col int) (printing.CellView, bool) {
	c := p.sh.Get(row, col)
	var dto CellDTO
	show := false
	if p.look != nil {
		dto, show = p.look.view(row, col, c)
	} else {
		v, n, x, st, fc := cellView(p.wb, p.sh, row, col, c)
		dto, show = CellDTO{V: v, N: n, X: x, St: st, Fc: fc}, c.HasContent() || st != 0
	}
	if !show {
		return printing.CellView{}, false
	}
	return printing.CellView{Text: dto.V, Numeric: dto.N, Center: dto.X, Style: p.wb.StyleTable().Get(dto.St), Color: dto.Fc}, true
}

func (p printSource) ObjectImage(o model.Object) []byte {
	if o.Image != nil {
		return o.Image.Data
	}
	return p.charts[o.ID]
}

// decodeCharts turns data URLs from the frontend into PNG bytes.
func decodeCharts(charts map[int]string) map[int][]byte {
	out := map[int][]byte{}
	for id, url := range charts {
		if i := strings.Index(url, ","); i >= 0 {
			url = url[i+1:]
		}
		if b, err := base64.StdEncoding.DecodeString(url); err == nil {
			out[id] = b
		}
	}
	return out
}

// renderPDF lays out and renders a sheet.
func renderPDF(sess *Session, sheet int, charts map[int]string) ([]byte, int, error) {
	var data []byte
	var pages int
	var err error
	sess.read(func(d *document.Document) {
		sh := sheetAt(d, sheet)
		if sh == nil {
			err = errors.New("無效的工作表")
			return
		}
		var plan *printing.Plan
		if plan, err = printing.Paginate(sh, pageSetupOf(sh)); err != nil {
			return
		}
		pages = len(plan.Pages)
		src := printSource{wb: d.Workbook(), sh: sh, look: newLook(d, sheet), charts: decodeCharts(charts)}
		data, err = printing.Render(sh, plan, src, printing.Info{FileName: d.Name(), SheetName: sh.Name, Now: time.Now()})
	})
	return data, pages, err
}

// PrintStore keeps rendered PDFs for the preview, served by the asset
// server at /print/<token>.pdf.
type PrintStore struct {
	mu    sync.Mutex
	files map[string][]byte
	order []string
}

var printStore = &PrintStore{files: map[string][]byte{}}

// PrintHandler serves preview PDFs; main installs it as asset middleware.
func PrintHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/print/") {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/print/"), ".pdf")
		printStore.mu.Lock()
		data, ok := printStore.files[token]
		printStore.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(data)
	})
}

func (p *PrintStore) put(data []byte) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.files[token] = data
	p.order = append(p.order, token)
	for len(p.order) > 4 {
		delete(p.files, p.order[0])
		p.order = p.order[1:]
	}
	return token
}

// PreviewResult is a rendered preview: its URL and page count.
type PreviewResult struct {
	URL   string `json:"url"`
	Pages int    `json:"pages"`
}

// PrintPreview renders a sheet to PDF for the preview. charts maps
// chart object IDs to PNG data URLs drawn by the frontend.
func (s *SheetService) PrintPreview(ctx context.Context, sheet int, charts map[int]string) (PreviewResult, error) {
	data, pages, err := renderPDF(s.at(ctx), sheet, charts)
	if err != nil {
		return PreviewResult{}, err
	}
	return PreviewResult{URL: "/print/" + printStore.put(data) + ".pdf", Pages: pages}, nil
}

// ExportPDF renders a sheet and saves it where the user chooses.
func (ds *DocumentService) ExportPDF(ctx context.Context, sheet int, charts map[int]string) (bool, error) {
	data, _, err := renderPDF(ds.at(ctx), sheet, charts)
	if err != nil {
		return false, err
	}
	var name string
	ds.at(ctx).read(func(d *document.Document) { name = d.Name() })
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ".pdf"
	path, err := ds.dialogs.PickPDF(name)
	if err != nil || path == "" {
		return false, err
	}
	if filepath.Ext(path) == "" {
		path += ".pdf"
	}
	return true, os.WriteFile(path, data, 0o644)
}

// PrintSheet renders a sheet and opens the system print dialog for it.
func (ds *DocumentService) PrintSheet(ctx context.Context, sheet int, charts map[int]string) error {
	if ds.PrintPDF == nil {
		return errors.New("無法列印")
	}
	data, _, err := renderPDF(ds.at(ctx), sheet, charts)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp("", "ww-excel-print-*.pdf")
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	f.Close()
	ds.PrintPDF(f.Name())
	return nil
}
