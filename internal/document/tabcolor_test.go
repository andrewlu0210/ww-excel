package document

import (
	"strings"
	"testing"
)

func TestSetTabColorCommand(t *testing.T) {
	d := New()
	defer d.Close()
	command, err := d.SetTabColorCommand(0, "#ff0000")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(command)
	if d.wb.Sheets[0].Layout.TabColor != "FF0000" {
		t.Fatal("color not normalized")
	}
	if cmd, err := d.SetTabColorCommand(0, "FF0000"); err != nil || cmd != nil {
		t.Fatal("same color should be a no-op")
	}
	cmd, err := d.SetTabColorCommand(0, "")
	if err != nil {
		t.Fatal(err)
	}
	d.Exec(cmd)
	if d.wb.Sheets[0].Layout.TabColor != "" {
		t.Fatal("color not cleared")
	}
	d.Undo()
	if d.wb.Sheets[0].Layout.TabColor != "FF0000" {
		t.Fatal("undo lost color")
	}
	d.Redo()
	if d.wb.Sheets[0].Layout.TabColor != "" {
		t.Fatal("redo failed")
	}
	for _, invalid := range []string{"red", "FF000000", "12345", strings.Repeat("F", 100), "GGGGGG"} {
		if _, err := d.SetTabColorCommand(0, invalid); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	if _, err := d.SetTabColorCommand(-1, "FF0000"); err == nil {
		t.Fatal("accepted missing sheet")
	}
}
