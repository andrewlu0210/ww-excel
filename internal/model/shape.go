package model

// Shape is an imported DrawingML preset shape (text box, ellipse, line, callout...). It is immutable;
// text edits replace the immutable shape while retaining original drawing markup.
type Shape struct {
	SourceID                 int    `json:",omitempty"`
	RawXML                   string `json:",omitempty"`
	Geometry                 string
	Fill                     string
	LineColor                string
	LineDash                 string
	LineWidth                float64
	Radius                   float64
	Font                     Font
	Text                     *RichText
	AlignH, AlignV           string
	Left, Right, Top, Bottom float64
	// W and H are the unrotated size in pixels; zero means the object size.
	// Rot is clockwise degrees. The object box is the rotated bounding box.
	W, H         float64
	Rot          float64
	FlipH, FlipV bool
	// Adjust holds preset geometry adjust values (adj, adj1...) as in DrawingML.
	Adjust map[string]float64
	// Path is a custom geometry (custGeom) with points normalized to 0..1.
	Path []PathCmd
	// HeadArrow and TailArrow draw arrowheads at the start and end of open paths.
	HeadArrow, TailArrow bool
}

// PathCmd is a custom geometry path command: moveTo, lnTo, cubicBezTo,
// quadBezTo or close, with its points as x,y pairs.
type PathCmd struct {
	Op  string    `json:"op"`
	Pts []float64 `json:"pts"`
}
