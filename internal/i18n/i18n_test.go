package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestT(t *testing.T) {
	defer SetLanguage(Chinese)
	tests := []struct{ in, want string }{
		{"確定", "OK"},
		{"未命名（已復原）", "Untitled (Recovered)"},
		{"已經有名為「預算」的工作表", `A sheet named "預算" already exists`},
		// Nested: a parser message with its position suffix.
		{"這裡不能出現「)」（位置 4）", `Unexpected ")" (position 4)`},
		{"函數 SUM 的引數太少（至少需要 1 個）", "Too few arguments for SUM (at least 1 needed)"},
		{"沒有這句", "沒有這句"},
		{"", ""},
	}
	SetLanguage(English)
	for _, tc := range tests {
		if got := T(tc.in); got != tc.want {
			t.Errorf("T(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	SetLanguage(Chinese)
	if got := T("確定"); got != "確定" {
		t.Errorf("Chinese T = %q", got)
	}
}

var cjk = regexp.MustCompile(`[\p{Han}]`)

// Parts of messages built by concatenation; their full forms are keys.
var fragments = map[string]bool{"無法隱藏所有的": true}

// TestCoverage checks that every UI string has an English translation:
// t('...') calls in the frontend and Chinese literals in the Go packages
// whose text reaches the user.
func TestCoverage(t *testing.T) {
	root := "../.."
	missing := map[string]string{}
	check := func(key, where string) {
		if cjk.MatchString(key) && !Has(key) && !fragments[key] {
			missing[key] = where
		}
	}

	tCall := regexp.MustCompile(`\bt\('((?:[^'\\]|\\.)*)'`)
	filepath.WalkDir(filepath.Join(root, "frontend/src"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.Contains(p, "/bench/") || strings.HasSuffix(p, ".test.ts") ||
			!(strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".svelte")) {
			return nil
		}
		src, _ := os.ReadFile(p)
		for _, m := range tCall.FindAllStringSubmatch(string(src), -1) {
			check(strings.ReplaceAll(m[1], `\'`, "'"), p)
		}
		return nil
	})

	goDirs := []string{".", "internal/document", "internal/command", "internal/service", "internal/formula",
		"internal/formula/parser", "internal/formula/lexer", "internal/formula/ast", "internal/formula/functions",
		"internal/io/xlsxfile", "internal/printing"}
	skip := map[string]bool{"fill.go": true, "benchservice.go": true}
	goString := regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	for _, dir := range goDirs {
		files, _ := filepath.Glob(filepath.Join(root, dir, "*.go"))
		for _, p := range files {
			base := filepath.Base(p)
			if strings.HasSuffix(base, "_test.go") || skip[base] || (dir == "internal/formula/ast" && base != "table.go") {
				continue
			}
			src, _ := os.ReadFile(p)
			for _, line := range strings.Split(string(src), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "//") {
					continue
				}
				if i := strings.Index(line, " // "); i >= 0 {
					line = line[:i]
				}
				for _, lit := range goString.FindAllString(line, -1) {
					if s, err := strconv.Unquote(lit); err == nil {
						check(s, p)
					}
				}
			}
		}
	}
	for k, where := range missing {
		t.Errorf("no English for %q (%s)", k, where)
	}
}

func TestSimplifiedChinese(t *testing.T) {
	defer SetLanguage(Chinese)
	SetLanguage(SimplifiedChinese)
	for _, tc := range []struct{ in, want string }{
		{"儲存", "保存"}, {"貼上", "粘贴"}, {"字型", "字体"},
		{"無法開啟「客戶資料.xlsx」", "无法打开「客戶資料.xlsx」"},
		{"這裡不能出現「)」（位置 4）", "这里不能出现「)」（位置 4）"},
	} {
		if got := T(tc.in); got != tc.want {
			t.Errorf("T(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	load()
	for key := range catalogs[English].exact {
		value, ok := Dictionary()[key]
		if !ok {
			t.Errorf("missing simplified translation: %q", key)
			continue
		}
		if strings.Join(verb.FindAllString(key, -1), ",") != strings.Join(verb.FindAllString(value, -1), ",") {
			t.Errorf("placeholders changed: %q -> %q", key, value)
		}
	}
	SetLanguage(English)
	if T("儲存") != "Save" {
		t.Fatal("English catalog lost")
	}
	SetLanguage(Chinese)
	if T("儲存") != "儲存" || len(Dictionary()) != 0 {
		t.Fatal("Traditional Chinese fallback failed")
	}
}
