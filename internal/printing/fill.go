package printing

import (
	"github.com/signintech/gopdf"
	"image"
	"math"
	"ww-excel/internal/model"
)

type fillImageKey struct {
	Spec model.FillSpec
	W, H int
}

func (r *renderer) styleFill(st model.Style, x0, y0, x1, y1 float64) {
	if st.FillSpec.Kind == "" {
		r.fill(st.Fill, x0, y0, x1, y1)
		return
	}
	if x1 <= x0 || y1 <= y0 {
		return
	}
	key := fillImageKey{st.FillSpec, max(1, min(512, int(math.Ceil((x1-x0)*4/3)))), max(1, min(512, int(math.Ceil((y1-y0)*4/3))))}
	if r.fillImages == nil {
		r.fillImages = map[fillImageKey]image.Image{}
	}
	img := r.fillImages[key]
	if img == nil {
		img = model.PaintFill(key.Spec, key.W, key.H)
		if len(r.fillImages) < 64 {
			r.fillImages[key] = img
		}
	}
	_ = r.pdf.ImageFrom(img, x0, y0, &gopdf.Rect{W: x1 - x0, H: y1 - y0})
}
