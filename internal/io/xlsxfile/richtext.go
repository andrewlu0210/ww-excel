package xlsxfile

import (
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"ww-excel/internal/model"
)

// readTextElement streams a shared/inline string, excluding phonetic annotations.
func readTextElement(dec *xml.Decoder, end string, f *excelize.File) (string, *model.RichText, error) {
	var text, run strings.Builder
	var runs []model.TextRun
	var font *model.RichFont
	inRun, inText, inProps, phonetic := false, false, false, false
	for {
		tok, err := dec.RawToken()
		if err != nil {
			return "", nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rPh":
				phonetic = true
			case "r":
				inRun = true
				run.Reset()
				font = nil
			case "t":
				inText = true
			case "rPr":
				inProps = true
				font = &model.RichFont{}
			default:
				if !inProps || phonetic {
					continue
				}
				value := attr(t, "val")
				flag := value != "0" && value != "false" && value != "none"
				switch t.Name.Local {
				case "rFont":
					font.Name = &value
				case "sz":
					if n, e := strconv.ParseFloat(value, 64); e == nil {
						font.Size = &n
					}
				case "b":
					font.Bold = &flag
				case "i":
					font.Italic = &flag
				case "u":
					font.Underline = &flag
				case "strike":
					font.Strike = &flag
				case "color":
					var theme *int
					if v := attr(t, "theme"); v != "" {
						i := atoi(v)
						theme = &i
					}
					color := resolveColor(f, attr(t, "rgb"), theme, xmlFloat(attr(t, "tint")))
					if v := attr(t, "indexed"); v != "" {
						color = normalizeHex(f.GetBaseColor("", atoi(v), nil))
					}
					font.Color = &color
				}
			}
		case xml.CharData:
			if inText && !phonetic {
				text.Write(t)
				if inRun {
					run.Write(t)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case end:
				if len(runs) > 0 {
					return text.String(), &model.RichText{Runs: runs}, nil
				}
				return text.String(), nil, nil
			case "r":
				runs = append(runs, model.TextRun{Text: run.String(), Font: font})
				inRun = false
			case "rPr":
				inProps = false
			case "t":
				inText = false
			case "rPh":
				phonetic = false
			}
		}
	}
}

func richExcelRuns(rich *model.RichText, base model.Font) []excelize.RichTextRun {
	if rich == nil {
		return nil
	}
	out := make([]excelize.RichTextRun, 0, len(rich.Runs))
	for _, run := range rich.Runs {
		font := run.Font.Resolve(base)
		x := &excelize.Font{Family: font.Name, Size: font.Size, Color: font.Color, Bold: font.Bold, Italic: font.Italic, Strike: font.Strike}
		if font.Underline {
			x.Underline = "single"
		}
		out = append(out, excelize.RichTextRun{Text: run.Text, Font: x})
	}
	return out
}

func xmlFloat(value string) float64 { n, _ := strconv.ParseFloat(value, 64); return n }
