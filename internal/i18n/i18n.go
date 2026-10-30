// Package i18n translates the UI. Source strings are the Traditional
// Chinese text used throughout the code; JSON catalogs provide other languages.
//
// Keys may contain fmt verbs (%s, %d, %v, %q, %c, %w): T then also translates
// strings that were formatted from that key, so messages can be built
// with fmt as usual and translated where they reach the user.
package i18n

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	Chinese           = "zh-TW"
	English           = "en"
	SimplifiedChinese = "zh-CN"
)

//go:embed en.json
var enJSON []byte

//go:embed zh-CN.json
var simplifiedJSON []byte

type pattern struct {
	re *regexp.Regexp
	// en holds the translated format with every verb turned into %s.
	en string
	// literal is the length of the key without its verbs.
	literal int
}

type catalog struct {
	exact    map[string]string
	patterns []pattern
}

var (
	loadOnce sync.Once
	catalogs map[string]*catalog
	lang     atomic.Value
)

var verb = regexp.MustCompile(`%[sdvqcw]`)

func load() {
	loadOnce.Do(func() {
		catalogs = map[string]*catalog{}
		for language, data := range map[string][]byte{English: enJSON, SimplifiedChinese: simplifiedJSON} {
			var exact map[string]string
			var patterns []pattern
			exact = map[string]string{}
			if err := json.Unmarshal(data, &exact); err != nil {
				panic("i18n: catalog: " + err.Error())
			}
			for zh, en := range exact {
				if !verb.MatchString(zh) {
					continue
				}
				parts := verb.Split(zh, -1)
				for i, p := range parts {
					parts[i] = regexp.QuoteMeta(p)
				}
				re := regexp.MustCompile("(?s)^" + strings.Join(parts, "(.+?)") + "$")
				patterns = append(patterns, pattern{re: re, en: verb.ReplaceAllString(en, "%s"), literal: len(verb.ReplaceAllString(zh, ""))})
			}
			// Most specific first, so "%s（位置 %d）" does not shadow longer
			// keys; ties by text for a stable order.
			sort.Slice(patterns, func(i, j int) bool {
				if patterns[i].literal != patterns[j].literal {
					return patterns[i].literal > patterns[j].literal
				}
				return patterns[i].re.String() < patterns[j].re.String()
			})
			catalogs[language] = &catalog{exact: exact, patterns: patterns}
		}
	})
}

// SetLanguage selects a supported UI language.
func SetLanguage(l string) {
	if l != English && l != SimplifiedChinese {
		l = Chinese
	}
	lang.Store(l)
}

func Language() string {
	if l, ok := lang.Load().(string); ok {
		return l
	}
	return Chinese
}

// T translates s into the current language. The parts of a formatted
// message are translated too, so nested messages ("%s（位置 %d）")
// use the selected language.
func T(s string) string {
	language := Language()
	if language == Chinese || s == "" {
		return s
	}
	load()
	c := catalogs[language]
	if en, ok := c.exact[s]; ok {
		return en
	}
	for _, p := range c.patterns {
		m := p.re.FindStringSubmatch(s)
		if m == nil {
			continue
		}
		out := p.en
		for _, arg := range m[1:] {
			out = strings.Replace(out, "%s", T(arg), 1)
		}
		return out
	}
	return s
}

// Dictionary returns the current language catalog for the frontend.
func Dictionary() map[string]string {
	load()
	if c := catalogs[Language()]; c != nil {
		return c.exact
	}
	return map[string]string{}
}

// Has reports whether s is a key of the English table; for tests that
// check coverage.
func Has(s string) bool {
	load()
	_, ok := catalogs[English].exact[s]
	return ok
}
