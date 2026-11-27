package assetworkbench

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"testing"
)

func TestArchiveTileRepresentation(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 640, 320))
	rand.New(rand.NewSource(10)).Read(img.Pix)
	for _, mime := range []string{"image/jpeg", "image/png"} {
		t.Run(mime, func(t *testing.T) {
			var raw bytes.Buffer
			var err error
			if mime == "image/jpeg" {
				err = jpeg.Encode(&raw, img, nil)
			} else {
				err = png.Encode(&raw, img)
			}
			if err != nil {
				t.Fatal(err)
			}
			stream := ArchiveEntryStream{Body: io.NopCloser(bytes.NewReader(raw.Bytes())), FileSize: int64(raw.Len()), MimeType: mime}
			defer stream.Close()
			if err := stream.PrepareThumbnail(); err != nil {
				t.Fatal(err)
			}
			tile, err := io.ReadAll(stream.Body)
			if err != nil || int64(len(tile)) != stream.FileSize || len(tile) >= raw.Len() {
				t.Fatalf("tile bytes=%d original=%d err=%v", len(tile), raw.Len(), err)
			}
			cfg, fmt, err := image.DecodeConfig(bytes.NewReader(tile))
			if err != nil || "image/"+fmt != mime || cfg.Width != archiveThumbEdge || cfg.Height != archiveThumbEdge/2 {
				t.Fatalf("config=%+v format=%s err=%v", cfg, fmt, err)
			}
		})
	}
}

func TestArchiveTileOriginalFallback(t *testing.T) {
	for _, tc := range []struct {
		mime string
		size int
	}{
		{"image/jpeg", 320}, {"image/jpeg", 15000}, {"image/png", archiveThumbMaxBytes + 1}, {"image/gif", 12000},
	} {
		raw := bytes.Repeat([]byte("x"), tc.size)
		s := ArchiveEntryStream{Body: io.NopCloser(bytes.NewReader(raw)), FileSize: int64(len(raw)), MimeType: tc.mime}
		if err := s.PrepareThumbnail(); err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(s.Body)
		s.Close()
		if err != nil || !bytes.Equal(raw, got) || s.FileSize != int64(len(got)) {
			t.Fatalf("fallback corrupted %s (%d bytes)", tc.mime, tc.size)
		}
	}
}
