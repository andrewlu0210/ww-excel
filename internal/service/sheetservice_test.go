package service

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ww-excel/internal/document"
	"ww-excel/internal/model"
)

type recorder struct {
	mu     sync.Mutex
	events []string
	diffs  []CellsChanged
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
	if d, ok := data.(CellsChanged); ok {
		r.diffs = append(r.diffs, d)
	}
}

func (r *recorder) lastDiff(t *testing.T) CellsChanged {
	t.Helper()
	if len(r.diffs) == 0 {
		t.Fatal("no diff emitted")
	}
	return r.diffs[len(r.diffs)-1]
}

func newTestSession(fake bool) (*Session, *recorder) {
	wb := model.NewWorkbook()
	if fake {
		model.FillFake(wb.Sheets[0], 20_000, 5, 1)
	}
	rec := &recorder{}
	return NewSession(document.FromWorkbook(wb, ""), rec.emit), rec
}

func TestGetBlocks(t *testing.T) {
	sess, _ := newTestSession(true)
	got := NewSheetService(sess).GetBlocks(bg, 0, []BlockRef{{0, 0}, {5000, 5000}, {0, 1}}, 0).Blocks
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Block (0,0) holds rows 0..63 x cols 0..4.
	if n := len(got[0].Cells); n != model.BlockRows*5 {
		t.Fatalf("block(0,0) cells = %d, want %d", n, model.BlockRows*5)
	}
	if c := got[0].Cells[0]; c.V != "Item" || c.N {
		t.Fatalf("first cell = %+v", c)
	}
	if len(got[1].Cells) != 0 || got[1].Cells == nil {
		t.Fatalf("empty block should have non-nil empty cells: %+v", got[1].Cells)
	}
}

func TestGetBlocksBadSheet(t *testing.T) {
	sess, _ := newTestSession(true)
	got := NewSheetService(sess).GetBlocks(bg, 9, []BlockRef{{0, 0}}, 0).Blocks
	if len(got) != 1 || len(got[0].Cells) != 0 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestEditEmitsOneDiffAndBumpsVersion(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	v0 := svc.GetBlocks(bg, 0, nil, 0).Version

	svc.SetCell(bg, 0, 2, 3, "12.5")
	d := rec.lastDiff(t)
	if len(rec.diffs) != 1 || d.Version <= v0 {
		t.Fatalf("diffs=%d version=%d (was %d)", len(rec.diffs), d.Version, v0)
	}
	if len(d.Cells) != 1 || !reflect.DeepEqual(d.Cells[0], ChangedCell{Row: 2, Col: 3, V: "12.5", N: true}) || len(d.Styles) != 0 {
		t.Fatalf("diff cells = %+v", d.Cells)
	}
	if d.UsedRows != 3 || d.UsedCols != 4 || d.Select != nil {
		t.Fatalf("diff meta = %+v", d)
	}
	if got := svc.GetBlocks(bg, 0, nil, 0).Version; got != d.Version {
		t.Fatalf("GetBlocks version %d != diff version %d", got, d.Version)
	}

	// Same value again: no-op, no event.
	svc.SetCell(bg, 0, 2, 3, "12.5")
	if len(rec.diffs) != 1 {
		t.Fatal("no-op edit emitted a diff")
	}
	if !sess.State().Dirty || !sess.State().CanUndo || sess.State().UndoLabel != "輸入" {
		t.Fatalf("state = %+v", sess.State())
	}
}

func TestUndoSelectsAffectedArea(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	if err := svc.Fill(bg, 0, []RangeDTO{{R0: 1, C0: 1, R1: 3, C1: 2}}, "x", 1, 1); err != nil {
		t.Fatal(err)
	}
	if !svc.Undo(bg) {
		t.Fatal("undo failed")
	}
	d := rec.lastDiff(t)
	if d.Select == nil || *d.Select != (RangeDTO{R0: 1, C0: 1, R1: 3, C1: 2}) {
		t.Fatalf("select = %+v", d.Select)
	}
	if len(d.Cells) != 6 || d.Cells[0].V != "" {
		t.Fatalf("undo diff = %+v", d.Cells)
	}
	if sess.State().Dirty {
		t.Fatal("undo back to start should be clean")
	}
	if svc.Undo(bg) {
		t.Fatal("nothing left to undo")
	}
}

func TestLargeClearInvalidates(t *testing.T) {
	sess, rec := newTestSession(true)
	svc := NewSheetService(sess)
	svc.Clear(bg, 0, []RangeDTO{{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: 4}}, "contents")
	d := rec.lastDiff(t)
	if !d.Invalidate || len(d.Cells) != 0 {
		t.Fatalf("expected invalidation, got %d cells", len(d.Cells))
	}
	if got := svc.EditText(bg, 0, 1, 0); got != "" {
		t.Fatalf("cell not cleared: %q", got)
	}
}

func TestFillTooLarge(t *testing.T) {
	sess, rec := newTestSession(false)
	err := NewSheetService(sess).Fill(bg, 0, []RangeDTO{{R0: 0, C0: 0, R1: model.MaxRows - 1, C1: model.MaxCols - 1}}, "x", 0, 0)
	if err == nil || len(rec.diffs) != 0 {
		t.Fatalf("err=%v diffs=%d", err, len(rec.diffs))
	}
}

func TestFormulaEditing(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	svc.SetCell(bg, 0, 0, 0, "2")
	if _, err := svc.SetCell(bg, 0, 0, 1, "=a1*10"); err != nil {
		t.Fatal(err)
	}
	if got := svc.EditText(bg, 0, 0, 1); got != "=A1*10" {
		t.Fatalf("canonical formula = %q", got)
	}

	// Editing A1 emits one diff that includes the recalculated B1.
	n := len(rec.diffs)
	svc.SetCell(bg, 0, 0, 0, "5")
	if len(rec.diffs) != n+1 {
		t.Fatalf("diffs = %d, want one more", len(rec.diffs)-n)
	}
	d := rec.lastDiff(t)
	if len(d.Cells) != 2 || !reflect.DeepEqual(d.Cells[1], ChangedCell{Row: 0, Col: 1, V: "50", N: true}) {
		t.Fatalf("diff = %+v", d.Cells)
	}

	// Rejected formulas change nothing and explain why.
	n = len(rec.diffs)
	if _, err := svc.SetCell(bg, 0, 0, 2, "=SUM("); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := svc.SetCell(bg, 0, 0, 2, "=ROUND(1)"); err == nil || !strings.Contains(err.Error(), "引數太少") {
		t.Fatalf("err = %v", err)
	}
	if len(rec.diffs) != n {
		t.Fatal("rejected formula emitted a diff")
	}

	// Errors and logicals are centered.
	svc.SetCell(bg, 0, 1, 0, "=1/0")
	if c := rec.lastDiff(t).Cells[0]; c.V != "#DIV/0!" || !c.X || c.N {
		t.Fatalf("error cell = %+v", c)
	}

	// ⌘Enter with a formula adjusts relative references per cell.
	if err := svc.Fill(bg, 0, []RangeDTO{{R0: 3, C0: 0, R1: 5, C1: 0}}, "=ROW()*10+A1", 3, 0); err != nil {
		t.Fatal(err)
	}
	if got := svc.EditText(bg, 0, 5, 0); got != "=ROW()*10+A3" {
		t.Fatalf("filled formula = %q", got)
	}

	// Undo recalculates dependents too.
	svc.SetCell(bg, 0, 0, 0, "7")
	svc.Undo(bg)
	if got := svc.GetBlocks(bg, 0, []BlockRef{{0, 0}}, 0).Blocks[0].Cells[1].V; got != "50" {
		t.Fatalf("B1 after undo = %q", got)
	}

	// Circular references are reported in the document state.
	svc.SetCell(bg, 0, 9, 0, "=A11")
	svc.SetCell(bg, 0, 10, 0, "=A10")
	if st := sess.State(); st.Circular != "A10" {
		t.Fatalf("circular = %q", st.Circular)
	}
}

func TestFormattingAndSheets(t *testing.T) {
	sess, rec := newTestSession(false)
	svc := NewSheetService(sess)
	docs := NewDocumentService(sess, &fakeDialogs{})
	svc.SetCell(bg, 0, 0, 0, "1234.5")
	b := true
	code := "#,##0.00"
	n := len(rec.diffs)
	if err := svc.Format(bg, 0, []RangeDTO{{0, 0, 0, 0}}, FormatPatch{Bold: &b, NumFmt: &code, Label: "粗體"}); err != nil {
		t.Fatal(err)
	}
	d := rec.lastDiff(t)
	if len(rec.diffs) != n+1 || d.Cells[0].V != "1,234.50" || len(d.Styles) != 1 || !d.Styles[0].Font.Bold || d.Cells[0].St != d.Styles[0].ID {
		t.Fatalf("format diff = %+v", d)
	}
	if sess.State().UndoLabel != "粗體" {
		t.Fatalf("undo label = %q", sess.State().UndoLabel)
	}

	// Outside border: only the edges of the range.
	svc.Format(bg, 0, []RangeDTO{{2, 2, 4, 4}}, FormatPatch{Border: "outside"})
	blocks := svc.GetBlocks(bg, 0, []BlockRef{{0, 0}}, 0)
	styles := map[uint32]StyleDTO{}
	for _, st := range blocks.Styles {
		styles[st.ID] = st
	}
	at := func(r, c int) StyleDTO {
		for _, cell := range blocks.Blocks[0].Cells {
			if cell.R == r && cell.C == c {
				return styles[cell.St]
			}
		}
		return StyleDTO{}
	}
	if at(2, 2).Top.Style != "thin" || at(2, 2).Left.Style != "thin" || at(3, 3).Top.Style != "" || at(4, 4).Bottom.Style != "thin" {
		t.Fatalf("outside border: %+v %+v %+v", at(2, 2), at(3, 3), at(4, 4))
	}

	// Sizes and merges change the layout.
	svc.SetSizes(bg, 0, false, []int{1}, 120)
	if err := svc.Merge(bg, 0, RangeDTO{6, 0, 6, 3}, true); err != nil {
		t.Fatal(err)
	}
	l := svc.Layout(bg, 0, 0)
	if len(l.Cols) != 1 || l.Cols[0].Px != 120 || len(l.Merges) != 1 || len(l.Styles) == 0 {
		t.Fatalf("layout = %+v", l)
	}
	if rec.events[len(rec.events)-3] != EventLayoutChanged {
		t.Fatalf("events = %v", rec.events[len(rec.events)-4:])
	}

	// Sheets.
	if idx := docs.AddSheet(bg, 1); idx != 1 || len(sess.State().Sheets) != 2 {
		t.Fatalf("add sheet = %d, %v", idx, sess.State().Sheets)
	}
	if err := docs.RenameSheet(bg, 1, "a:b"); err == nil {
		t.Fatal("invalid name accepted")
	}
	if err := docs.RenameSheet(bg, 1, "報表"); err != nil || sess.State().Sheets[1] != "報表" {
		t.Fatalf("rename: %v %v", err, sess.State().Sheets)
	}
	docs.MoveSheet(bg, 1, 0)
	if sess.State().Sheets[0] != "報表" {
		t.Fatal("move")
	}
	if !docs.DeleteSheet(bg, 0) || len(sess.State().Sheets) != 1 {
		t.Fatal("delete")
	}
}

func TestEditTextAndDataEdge(t *testing.T) {
	sess, _ := newTestSession(true)
	svc := NewSheetService(sess)
	svc.SetCell(bg, 0, 0, 10, "'007")
	if got := svc.EditText(bg, 0, 0, 10); got != "'007" {
		t.Fatalf("EditText = %q", got)
	}
	if got := svc.DataEdge(bg, 0, 0, 0, 1, 0); got != (CellPos{Row: 19_999, Col: 0}) {
		t.Fatalf("DataEdge = %+v", got)
	}
	if got := svc.DataEdge(bg, 0, 0, 0, 1, 1); got != (CellPos{}) {
		t.Fatalf("diagonal should be rejected: %+v", got)
	}
}

func TestInfo(t *testing.T) {
	sess, _ := newTestSession(true)
	info := NewSheetService(sess).Info(bg, 0)
	if info.Rows != model.MaxRows || info.Cols != model.MaxCols {
		t.Fatalf("dims = %dx%d", info.Rows, info.Cols)
	}
	if info.UsedRows != model.MaxRows || info.UsedCols != model.MaxCols {
		t.Fatalf("used = %dx%d (edge markers should extend it)", info.UsedRows, info.UsedCols)
	}
}

type fakeDialogs struct {
	choice   SaveChoice
	openPath string
	savePath string
	asked    int
	errors   []string
}

func (f *fakeDialogs) AskSave(string) SaveChoice         { f.asked++; return f.choice }
func (f *fakeDialogs) PickOpen() (string, error)         { return f.openPath, nil }
func (f *fakeDialogs) PickSave(string) (string, error)   { return f.savePath, nil }
func (f *fakeDialogs) PickImage() (string, error)        { return f.openPath, nil }
func (f *fakeDialogs) PickPDF(string) (string, error)    { return f.savePath, nil }
func (f *fakeDialogs) ShowError(title, msg string)       { f.errors = append(f.errors, title+": "+msg) }
func (f *fakeDialogs) ShowWarning(title, message string) {}
func (f *fakeDialogs) Confirm(title, message, ok string) bool {
	f.asked++
	return f.choice != ChoiceCancel
}

func TestDocumentFlow(t *testing.T) {
	dir := t.TempDir()
	sess, rec := newTestSession(false)
	dlg := &fakeDialogs{savePath: filepath.Join(dir, "book")}
	sheets := NewSheetService(sess)
	docs := NewDocumentService(sess, dlg)

	// Clean document: New does not ask.
	if !docs.New(bg) || dlg.asked != 0 {
		t.Fatal("New on clean doc should not prompt")
	}

	sheets.SetCell(bg, 0, 0, 0, "hello")
	dlg.choice = ChoiceCancel
	if docs.New(bg) || dlg.asked != 1 {
		t.Fatal("cancel must abort New")
	}

	// Save on untitled asks for a path and appends .wwx.
	if !docs.Save(bg) {
		t.Fatalf("save failed: %v", dlg.errors)
	}
	st := docs.State(bg)
	if st.Name != "book.xlsx" || st.Dirty {
		t.Fatalf("state after save = %+v", st)
	}

	// Open the saved file into a fresh session state.
	sheets.SetCell(bg, 0, 0, 1, "dirty")
	dlg.choice = ChoiceDiscard
	dlg.openPath = filepath.Join(dir, "book.xlsx")
	if !docs.Open(bg) {
		t.Fatalf("open failed: %v", dlg.errors)
	}
	if got := sheets.EditText(bg, 0, 0, 1); got != "" {
		t.Fatalf("discarded edit survived: %q", got)
	}
	if got := sheets.EditText(bg, 0, 0, 0); got != "hello" {
		t.Fatalf("opened cell = %q", got)
	}
	if rec.events[len(rec.events)-2] != EventDocReset {
		t.Fatalf("expected doc:reset before state, got %v", rec.events[len(rec.events)-3:])
	}

	// Save choice in the prompt saves first.
	sheets.SetCell(bg, 0, 5, 5, "1")
	dlg.choice = ChoiceSave
	if !docs.ConfirmClose(bg) {
		t.Fatal("save-then-close should proceed")
	}
	if docs.State(bg).Dirty {
		t.Fatal("ConfirmClose with Save should save")
	}

	if err := docs.OpenPath(bg, filepath.Join(dir, "missing.xlsx")); err == nil || len(dlg.errors) == 0 {
		t.Fatal("opening a missing file should report an error")
	}
}

// A typical viewport fetch: ~40 rows x ~15 cols plus prefetch ring
// spans about 3x3 blocks.
func BenchmarkGetBlocksViewport(b *testing.B) {
	sess, _ := newTestSession(true)
	s := NewSheetService(sess)
	refs := make([]BlockRef, 0, 9)
	for r := 10; r < 13; r++ {
		for c := 0; c < 3; c++ {
			refs = append(refs, BlockRef{r, c})
		}
	}
	for b.Loop() {
		s.GetBlocks(bg, 0, refs, 0)
	}
}

var bg = context.Background()

func TestClipboardRoundTrip(t *testing.T) {
	sess, _ := newTestSession(false)
	svc := NewSheetService(sess)
	var system string
	ConnectClipboard(svc, func(s string) bool { system = s; return true }, func() (string, bool) { return system, true })
	svc.SetCell(bg, 0, 0, 0, "2")
	svc.SetCell(bg, 0, 0, 1, "=A1*3")
	res, err := svc.Copy(bg, 0, RangeDTO{R0: 0, C0: 0, R1: 0, C1: 1}, false)
	if err != nil || res.Text != "2\t6\n" || system != res.Text || !strings.Contains(res.HTML, "<td") {
		t.Fatalf("copy = %+v, %v", res, err)
	}
	// Our own text pastes formulas.
	if _, err := svc.Paste(bg, 0, RangeDTO{R0: 3, C0: 0, R1: 3, C1: 0}, nil); err != nil {
		t.Fatal(err)
	}
	if got := svc.EditText(bg, 0, 3, 1); got != "=A4*3" {
		t.Fatalf("internal paste = %q", got)
	}
	// Text from another app is entered as typed.
	system = "x\t=1+1"
	svc.Paste(bg, 0, RangeDTO{R0: 5, C0: 0, R1: 5, C1: 0}, nil)
	if got := svc.EditText(bg, 0, 5, 1); got != "=1+1" {
		t.Fatalf("external paste = %q", got)
	}
	// A cut moves once.
	svc.Copy(bg, 0, RangeDTO{R0: 0, C0: 0, R1: 0, C1: 0}, true)
	r, err := svc.Paste(bg, 0, RangeDTO{R0: 0, C0: 4, R1: 0, C1: 4}, nil)
	if err != nil || !r.Moved || svc.EditText(bg, 0, 0, 1) != "=E1*3" || svc.HasClip(bg) {
		t.Fatalf("cut paste: %+v %v %q", r, err, svc.EditText(bg, 0, 0, 1))
	}
}

func TestRecentFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "recent.json")
	r := LoadRecent(path)
	for i := 0; i < MaxRecent+2; i++ {
		r.Add(fmt.Sprintf("/f%d.xlsx", i))
	}
	r.Add("/f5.xlsx")
	got := LoadRecent(path).List()
	if len(got) != MaxRecent || got[0] != "/f5.xlsx" || got[1] != "/f11.xlsx" {
		t.Fatalf("recent = %v", got)
	}
}
