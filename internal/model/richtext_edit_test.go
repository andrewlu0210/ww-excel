package model

import (
	"strings"
	"testing"
)

func TestPreserveRichTextUnicodeAndInheritance(t *testing.T) {
	red, yes, no := "FF0000", true, false
	font := &RichFont{Color: &red, Bold: &yes}
	plain := &RichFont{Bold: &no}
	old := StringCell("甲😀乙👩‍💻丙")
	old.Rich = &RichText{Runs: []TextRun{{Text: "甲😀", Font: font}, {Text: "乙👩‍💻丙", Font: plain}}}
	got := PreserveRichText(old, StringCell("甲🙂乙👩‍💻丁"))
	if got.Rich == nil || len(got.Rich.Runs) != 2 ||
		got.Rich.Runs[0].Text != "甲🙂" || got.Rich.Runs[0].Font != font ||
		got.Rich.Runs[1].Text != "乙👩‍💻丁" || got.Rich.Runs[1].Font != plain {
		t.Fatalf("Unicode edit lost run boundaries: %+v", got.Rich)
	}
	base := DefaultFont
	base.Bold = true
	if got.Rich.Runs[1].Font.Resolve(base).Bold {
		t.Fatal("explicit false must not inherit the cell's bold font")
	}
}

func TestPreserveRichTextCoalescesEquivalentFonts(t *testing.T) {
	colorA, colorB := "FF0000", "FF0000"
	old := StringCell("ab")
	old.Rich = &RichText{Runs: []TextRun{
		{Text: "a", Font: &RichFont{Color: &colorA}},
		{Text: "b", Font: &RichFont{Color: &colorB}},
	}}
	got := PreserveRichText(old, StringCell("abc"))
	if len(got.Rich.Runs) != 1 || got.Rich.Runs[0].Text != "abc" {
		t.Fatalf("equivalent adjacent fonts should merge: %+v", got.Rich)
	}
}

func TestPreserveRichTextExplicitAndStaleRuns(t *testing.T) {
	old := StringCell("old")
	old.Rich = &RichText{Runs: []TextRun{{Text: "stale"}}}
	next := StringCell("new")
	if got := PreserveRichText(old, next); got.Rich != nil {
		t.Fatal("stale run offsets must not be reused")
	}
	next.Rich = &RichText{Runs: []TextRun{{Text: "new"}}}
	if got := PreserveRichText(old, next); got.Rich != next.Rich {
		t.Fatal("explicit formatting must take precedence")
	}
}

func TestPreserveRichTextLargeChanges(t *testing.T) {
	yes := true
	font := &RichFont{Bold: &yes}
	old := StringCell("標題" + strings.Repeat("甲", 3000) + "結尾")
	old.Rich = &RichText{Runs: []TextRun{
		{Text: "標題", Font: nil},
		{Text: strings.Repeat("甲", 3000), Font: font},
		{Text: "結尾", Font: nil},
	}}
	text := "標題" + strings.Repeat("乙", 3000) + "結尾"
	got := PreserveRichText(old, StringCell(text))
	if len(got.Rich.Runs) != 3 || got.Rich.Runs[1].Font != font || got.Rich.Runs[1].Text != strings.Repeat("乙", 3000) {
		t.Fatal("large replacement lost surrounding formats or replacement font")
	}
	long := StringCell(strings.Repeat("字", 32767))
	long.Rich = &RichText{Runs: []TextRun{{Text: long.Str, Font: font}}}
	got = PreserveRichText(long, StringCell("新"+long.Str))
	if len(got.Rich.Runs) != 1 || got.Rich.Runs[0].Text != got.Str {
		t.Fatal("long insertion lost text or formatting")
	}
}
