package model

// Shape is an imported rectangular DrawingML text box. It is immutable;
// shape editing is not exposed yet, so original XLSX drawing markup is preserved.
type Shape struct {
	Geometry                 string
	Fill                     string
	LineColor                string
	LineWidth                float64
	Radius                   float64
	Font                     Font
	Text                     *RichText
	AlignH, AlignV           string
	Left, Right, Top, Bottom float64
}
