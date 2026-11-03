package xlsxfile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"ww-excel/internal/model"
)

const drawingNS = "http://schemas.openxmlformats.org/drawingml/2006/main"
const spreadsheetDrawingNS = "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"

type drawingNode struct {
	name                    string
	attrs                   []xml.Attr
	start, open, close, end int
	parent                  *drawingNode
	children                []*drawingNode
}

func drawingNodes(data []byte) (*drawingNode, error) {
	root := &drawingNode{}
	stack := []*drawingNode{root}
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		start := int(d.InputOffset())
		token, err := d.Token()
		if err == io.EOF {
			return root, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			parent := stack[len(stack)-1]
			node := &drawingNode{name: t.Name.Local, attrs: t.Attr, start: start, open: int(d.InputOffset()), parent: parent}
			parent.children = append(parent.children, node)
			stack = append(stack, node)
		case xml.EndElement:
			node := stack[len(stack)-1]
			node.close = start
			node.end = int(d.InputOffset())
			stack = stack[:len(stack)-1]
		}
	}
}
func (n *drawingNode) child(name string) *drawingNode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}
func (n *drawingNode) all(name string) []*drawingNode {
	var out []*drawingNode
	for _, c := range n.children {
		if c.name == name {
			out = append(out, c)
		}
		out = append(out, c.all(name)...)
	}
	return out
}
func (n *drawingNode) attr(name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
func (n *drawingNode) raw(data []byte) string {
	if n == nil {
		return ""
	}
	return string(data[n.start:n.end])
}
func (n *drawingNode) id() int {
	id, _ := strconv.Atoi(n.child("nvSpPr").child("cNvPr").attr("id"))
	return id
}
func findShape(root *drawingNode, id int) *drawingNode {
	for _, n := range root.all("sp") {
		if n.id() == id {
			return n
		}
	}
	return nil
}
func escapeDrawing(text string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(text))
	return b.String()
}

type drawingEdit struct {
	start, end int
	text       string
}

func applyDrawingEdits(data []byte, edits []drawingEdit) []byte {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	for _, e := range edits {
		data = append(append(append([]byte{}, data[:e.start]...), []byte(e.text)...), data[e.end:]...)
	}
	return data
}
func drawingAttribute(data []byte, node *drawingNode, name, value string) drawingEdit {
	tag := string(data[node.start:node.open])
	pattern := regexp.MustCompile(`\s+` + regexp.QuoteMeta(name) + `\s*=\s*(?:"[^"]*"|'[^']*')`)
	attr := ` ` + name + `="` + escapeDrawing(value) + `"`
	if pattern.MatchString(tag) {
		tag = pattern.ReplaceAllStringFunc(tag, func(string) string { return attr })
	} else {
		at := strings.LastIndex(tag, ">")
		if at > 0 && tag[at-1] == '/' {
			at--
		}
		tag = tag[:at] + attr + tag[at:]
	}
	return drawingEdit{node.start, node.open, tag}
}
func standaloneShape(data []byte, root, node *drawingNode) string {
	raw := node.raw(data)
	at := strings.Index(raw, ">")
	if at < 0 {
		return raw
	}
	var ns strings.Builder
	if len(root.children) > 0 {
		for _, a := range root.children[0].attrs {
			if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
				key := "xmlns"
				if a.Name.Space == "xmlns" {
					key += ":" + a.Name.Local
				}
				if !strings.Contains(raw[:at], key+"=") {
					ns.WriteString(` ` + key + `="` + escapeDrawing(a.Value) + `"`)
				}
			}
		}
	}
	return raw[:at] + ns.String() + raw[at:]
}
func shapeSources(data []byte) map[int]string {
	out := map[int]string{}
	root, err := drawingNodes(data)
	if err != nil {
		return out
	}
	for _, node := range root.all("sp") {
		out[node.id()] = standaloneShape(data, root, node)
	}
	return out
}
func fontDrawing(font model.Font, tag string) string {
	size := font.Size
	if size <= 0 {
		size = 11
	}
	u, strike := "none", "noStrike"
	if font.Underline {
		u = "sng"
	}
	if font.Strike {
		strike = "sngStrike"
	}
	text := fmt.Sprintf(`<a:%s sz="%d" b="%d" i="%d" u="%s" strike="%s">`, tag, int(mathRound(size*100)), boolInt(font.Bold), boolInt(font.Italic), u, strike)
	if font.Color != "" {
		text += `<a:solidFill><a:srgbClr val="` + escapeDrawing(font.Color) + `"/></a:solidFill>`
	}
	if font.Name != "" {
		for _, name := range []string{"latin", "ea", "cs"} {
			text += `<a:` + name + ` typeface="` + escapeDrawing(font.Name) + `"/>`
		}
	}
	return text + `</a:` + tag + `>`
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func mathRound(v float64) float64 { return math.Round(v) }
func shapeTextBody(data []byte, body *drawingNode, shape *model.Shape) string {
	// Keep body/list/paragraph properties, including margins, bullets and spacing.
	var paragraphs [][]model.TextRun
	paragraphs = append(paragraphs, nil)
	if shape.Text != nil {
		for _, run := range shape.Text.Runs {
			for i, part := range strings.Split(run.Text, "\n") {
				if i > 0 {
					paragraphs = append(paragraphs, nil)
				}
				paragraphs[len(paragraphs)-1] = append(paragraphs[len(paragraphs)-1], model.TextRun{Text: part, Font: run.Font})
			}
		}
	}
	old := body.all("p")
	var b strings.Builder
	b.WriteString(`<xdr:txBody xmlns:xdr="` + spreadsheetDrawingNS + `" xmlns:a="` + drawingNS + `">`)
	properties := body.child("bodyPr").raw(data)
	if properties == "" {
		properties = `<a:bodyPr/>`
	}
	b.WriteString(properties)
	list := body.child("lstStyle").raw(data)
	if list == "" {
		list = `<a:lstStyle/>`
	}
	b.WriteString(list)
	for i, runs := range paragraphs {
		b.WriteString(`<a:p>`)
		if len(old) > 0 {
			b.WriteString(old[min(i, len(old)-1)].child("pPr").raw(data))
		}
		last := shape.Font
		for _, run := range runs {
			last = run.Font.Resolve(shape.Font)
			if run.Text != "" {
				b.WriteString(`<a:r>` + fontDrawing(last, "rPr") + `<a:t xml:space="preserve">` + escapeDrawing(run.Text) + `</a:t></a:r>`)
			}
		}
		b.WriteString(fontDrawing(last, "endParaRPr") + `</a:p>`)
	}
	b.WriteString(`</xdr:txBody>`)
	return b.String()
}
func replaceShapeText(data []byte, node *drawingNode, shape *model.Shape) []byte {
	body := node.child("txBody")
	templateData, templateBody := data, body
	// The immutable import template retains paragraph settings even after a
	// previous save removed paragraphs and the user later undoes that edit.
	if shape.RawXML != "" {
		raw := []byte(shape.RawXML)
		if root, err := drawingNodes(raw); err == nil {
			if source := findShape(root, shape.SourceID); source != nil && source.child("txBody") != nil {
				templateData, templateBody = raw, source.child("txBody")
			}
		}
	}
	if templateBody == nil {
		templateBody = &drawingNode{}
	}
	replacement := shapeTextBody(templateData, templateBody, shape)
	if body == nil {
		return applyDrawingEdits(data, []drawingEdit{{node.close, node.close, replacement}})
	}
	return applyDrawingEdits(data, []drawingEdit{{body.start, body.end, replacement}})
}
