package service

import "testing"

func TestShouldReadFirstRenderableFrame(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		mimeType string
		want     bool
	}{
		{
			name:     "tif extension",
			filename: "scan.TIF",
			want:     true,
		},
		{
			name:     "tiff extension",
			filename: "scan.tiff",
			want:     true,
		},
		{
			name:     "tiff mime type",
			filename: "scan",
			mimeType: "image/tiff",
			want:     true,
		},
		{
			name:     "x tiff mime type with parameters",
			filename: "scan",
			mimeType: "image/x-tiff; charset=binary",
			want:     true,
		},
		{
			name:     "pdf extension",
			filename: "document.pdf",
			want:     true,
		},
		{
			name:     "jpg remains single image input",
			filename: "photo.jpg",
			mimeType: "image/jpeg",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReadFirstRenderableFrame(tt.filename, tt.mimeType); got != tt.want {
				t.Fatalf("shouldReadFirstRenderableFrame(%q, %q) = %v, want %v", tt.filename, tt.mimeType, got, tt.want)
			}
		})
	}
}

func TestIsPDFPreviewSource(t *testing.T) {
	tests := []struct {
		filename string
		mimeType string
		want     bool
	}{
		{filename: "document.PDF", want: true},
		{filename: "document", mimeType: "application/pdf; charset=binary", want: true},
		{filename: "photo.jpg", mimeType: "image/jpeg", want: false},
	}
	for _, tt := range tests {
		if got := isPDFPreviewSource(tt.filename, tt.mimeType); got != tt.want {
			t.Fatalf("isPDFPreviewSource(%q, %q) = %v, want %v", tt.filename, tt.mimeType, got, tt.want)
		}
	}
}
