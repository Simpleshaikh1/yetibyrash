package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"slices"
	"strings"
	"testing"
)

func pngOf(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestProcessMakesSquareSizes(t *testing.T) {
	p, err := Process(bytes.NewReader(pngOf(2000, 1500, color.RGBA{240, 220, 210, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if p.Side != 1500 {
		t.Fatalf("side = %d, want 1500 (shorter side)", p.Side)
	}
	for _, w := range Sizes {
		img, err := jpeg.Decode(bytes.NewReader(p.Files[w]))
		if err != nil {
			t.Fatalf("size %d: %v", w, err)
		}
		want := min(w, 1500) // never enlarged past the source
		if b := img.Bounds(); b.Dx() != want || b.Dy() != want {
			t.Fatalf("size %d is %dx%d, want %dx%d", w, b.Dx(), b.Dy(), want, want)
		}
	}
	if len(p.Warnings) != 0 {
		t.Fatalf("warnings = %v, want none for a big bright photo", p.Warnings)
	}
}

func TestProcessWarnsSmallAndDark(t *testing.T) {
	p, err := Process(bytes.NewReader(pngOf(600, 800, color.RGBA{20, 15, 15, 255})))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Warnings, WarnSmall) || !slices.Contains(p.Warnings, WarnDark) {
		t.Fatalf("warnings = %v, want small and dark", p.Warnings)
	}
	// Small photos are never enlarged: the 1600 file is 600px.
	img, _ := jpeg.Decode(bytes.NewReader(p.Files[1600]))
	if img.Bounds().Dx() != 600 {
		t.Fatalf("1600 file is %dpx wide, want 600 (not enlarged)", img.Bounds().Dx())
	}
}

func TestProcessRejectsNonImages(t *testing.T) {
	if _, err := Process(strings.NewReader("<html>not a photo</html>")); err != ErrNotAnImage {
		t.Fatalf("err = %v, want ErrNotAnImage", err)
	}
}

func TestApplyOrientationRotates(t *testing.T) {
	// 4x2 image, left half red. Orientation 6 means "rotate 90° clockwise to view".
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{0, 0, 255, 255}
			if x < 2 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	out := applyOrientation(img, 6)
	if b := out.Bounds(); b.Dx() != 2 || b.Dy() != 4 {
		t.Fatalf("rotated size = %dx%d, want 2x4", b.Dx(), b.Dy())
	}
	// After a clockwise turn the left (red) half is on top.
	if r, _, _, _ := out.At(0, 0).RGBA(); r>>8 != 255 {
		t.Fatal("top of rotated image should be red")
	}
	if r, _, _, _ := out.At(0, 3).RGBA(); r>>8 != 0 {
		t.Fatal("bottom of rotated image should be blue")
	}
}
