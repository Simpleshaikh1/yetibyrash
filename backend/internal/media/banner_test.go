package media

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"slices"
	"testing"
)

func TestProcessBannerMakesBothCrops(t *testing.T) {
	p, err := ProcessBanner(bytes.NewReader(pngOf(3000, 2000, color.RGBA{240, 220, 210, 255})))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"phone": {1080, 1350}, "wide": {2000, 875}}
	for name, size := range want {
		img, err := jpeg.Decode(bytes.NewReader(p.Files[name]))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if b := img.Bounds(); b.Dx() != size[0] || b.Dy() != size[1] {
			t.Errorf("%s = %dx%d, want %dx%d", name, b.Dx(), b.Dy(), size[0], size[1])
		}
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", p.Warnings)
	}
}

func TestProcessBannerNeverEnlarges(t *testing.T) {
	p, err := ProcessBanner(bytes.NewReader(pngOf(900, 900, color.RGBA{240, 220, 210, 255})))
	if err != nil {
		t.Fatal(err)
	}
	img, _ := jpeg.Decode(bytes.NewReader(p.Files["wide"]))
	if b := img.Bounds(); b.Dx() != 900 || b.Dy() != 393 {
		t.Errorf("wide = %dx%d, want 900x393 (the photo's own width)", b.Dx(), b.Dy())
	}
	if !slices.Contains(p.Warnings, WarnSmall) {
		t.Errorf("warnings = %v, want small", p.Warnings)
	}
}
