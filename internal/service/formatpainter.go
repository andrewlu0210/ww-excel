package service

import (
	"context"

	"ww-excel/internal/document"
)

// CaptureFormats keeps a per-window format sample without touching the clipboard.
func (s *SheetService) CaptureFormats(ctx context.Context, sheet int, area RangeDTO) error {
	session := s.at(ctx)
	session.mu.Lock()
	defer session.mu.Unlock()
	sample, err := session.doc.CaptureFormats(sheet, area.model())
	session.formatSample = sample
	return err
}

// PaintFormats is one undoable operation. The sample is reusable until replaced.
func (s *SheetService) PaintFormats(ctx context.Context, sheet int, ranges []RangeDTO) error {
	session := s.at(ctx)
	var err error
	session.change(true, func(d *document.Document) (document.Change, bool) {
		cmd, buildErr := d.PaintFormats(session.formatSample, sheet, toModelRanges(ranges))
		err = buildErr
		if err != nil || cmd == nil {
			return document.Change{}, false
		}
		return d.Exec(cmd)
	})
	return err
}
