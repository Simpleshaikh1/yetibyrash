package media

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"golang.org/x/image/draw"
)

// Banner crops: phones get a tall 4:5 picture, wider screens a 16:7 strip.
var bannerCrops = []struct {
	Name          string
	Width, Height int
}{
	{"phone", 1080, 1350},
	{"wide", 2000, 875},
}

// ProcessedBanner holds a banner photo's two crops, keyed "phone" and "wide".
type ProcessedBanner struct {
	Files    map[string][]byte
	Warnings []string
}

// ProcessBanner turns a photo upright and makes the phone and wide crops from its centre.
// A photo narrower than 1600 px gets the "small" warning: it will look soft on a laptop.
func ProcessBanner(r io.Reader) (*ProcessedBanner, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxUploadBytes {
		return nil, fmt.Errorf("photo is larger than %d MB", MaxUploadBytes>>20)
	}
	src, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotAnImage
	}
	if format == "jpeg" {
		src = applyOrientation(src, exifOrientation(data))
	}

	out := &ProcessedBanner{Files: map[string][]byte{}, Warnings: []string{}}
	if src.Bounds().Dx() < 1600 {
		out.Warnings = append(out.Warnings, WarnSmall)
	}
	for _, c := range bannerCrops {
		crop := cropAspect(src, c.Width, c.Height)
		w := min(c.Width, crop.Bounds().Dx()) // never enlarge
		h := w * c.Height / c.Width
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), crop, crop.Bounds(), draw.Src, nil)
		if c.Name == "phone" && averageBrightness(dst) < 70 {
			out.Warnings = append(out.Warnings, WarnDark)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
			return nil, err
		}
		out.Files[c.Name] = buf.Bytes()
	}
	return out, nil
}

// cropAspect returns the largest centred part of img with the aspect w:h.
func cropAspect(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	cw, ch := b.Dx(), b.Dx()*h/w
	if ch > b.Dy() {
		ch = b.Dy()
		cw = ch * w / h
	}
	x0 := b.Min.X + (b.Dx()-cw)/2
	y0 := b.Min.Y + (b.Dy()-ch)/2
	rect := image.Rect(0, 0, cw, ch)
	dst := image.NewRGBA(rect)
	draw.Draw(dst, rect, img, image.Pt(x0, y0), draw.Src)
	return dst
}
