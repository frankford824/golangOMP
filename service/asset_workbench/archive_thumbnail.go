package assetworkbench

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/disintegration/imaging"
)

const (
	archiveThumbEdge      = 320
	archiveThumbMaxBytes  = 2 << 20
	archiveThumbMaxPixels = 8_000_000
)

// PrepareThumbnail selects a smaller representation for directory tiles. The
// archive reader's concurrency slot remains held while decoding untrusted input.
func (s *ArchiveEntryStream) PrepareThumbnail() error {
	if s.FileSize <= 4096 || s.FileSize > archiveThumbMaxBytes || (s.MimeType != "image/jpeg" && s.MimeType != "image/png") {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(s.Body, s.FileSize+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) != s.FileSize {
		return errors.New("archive member size mismatch")
	}
	_ = s.Body.Close()
	s.Body = io.NopCloser(bytes.NewReader(raw))
	var config image.Config
	if s.MimeType == "image/jpeg" {
		config, err = jpeg.DecodeConfig(bytes.NewReader(raw))
	} else {
		config, err = png.DecodeConfig(bytes.NewReader(raw))
	}
	if err != nil || config.Height <= 0 || config.Width <= 0 ||
		max(config.Width, config.Height) > 16384 || config.Width > archiveThumbMaxPixels/config.Height || max(config.Width, config.Height) <= archiveThumbEdge {
		return nil
	}
	im, err := imaging.Decode(bytes.NewReader(raw), imaging.AutoOrientation(true))
	if err != nil {
		return nil
	}
	thumb := imaging.Fit(im, archiveThumbEdge, archiveThumbEdge, imaging.Box)
	var out bytes.Buffer
	if s.MimeType == "image/jpeg" {
		err = jpeg.Encode(&out, thumb, &jpeg.Options{Quality: 75})
	} else {
		err = png.Encode(&out, thumb)
	}
	if err == nil && out.Len() < len(raw) {
		s.FileSize = int64(out.Len())
		s.Body = io.NopCloser(bytes.NewReader(out.Bytes()))
	}
	return nil
}
