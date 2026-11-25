package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoding
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoding
)

// MaxUploadBytes is the largest photo staff can upload.
const MaxUploadBytes = 15 << 20 // 15 MB

// Sizes are the square widths saved for every photo: grid cards use 800, the product page 1600.
var Sizes = []int{400, 800, 1600}

// Warnings shown on the admin's photo tile. Neither blocks the upload.
const (
	WarnSmall = "small" // shorter side under 1200 px: zoom looks soft
	WarnDark  = "dark"  // average brightness low: hard to see the product
)

var ErrNotAnImage = errors.New("the file is not a JPEG, PNG or WebP photo")

// Processed is an uploaded photo ready to store.
type Processed struct {
	Side     int            // side of the square crop in the original, in px
	Files    map[int][]byte // width → JPEG bytes
	Warnings []string
}

// Process decodes a photo, turns it upright (phone photos store rotation separately),
// crops the centre square, and saves each size in Sizes as a JPEG.
func Process(r io.Reader) (*Processed, error) {
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

	square := cropSquare(src)
	side := square.Bounds().Dx()

	out := &Processed{Side: side, Files: map[int][]byte{}, Warnings: []string{}}
	if side < 1200 {
		out.Warnings = append(out.Warnings, WarnSmall)
	}
	for _, w := range Sizes {
		target := min(w, side) // never enlarge a small photo
		dst := image.NewRGBA(image.Rect(0, 0, target, target))
		draw.CatmullRom.Scale(dst, dst.Bounds(), square, square.Bounds(), draw.Src, nil)

		if w == Sizes[0] && averageBrightness(dst) < 70 {
			out.Warnings = append(out.Warnings, WarnDark)
		}

		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
			return nil, err
		}
		out.Files[w] = buf.Bytes()
	}
	return out, nil
}

// cropSquare returns the largest centred square of img.
func cropSquare(img image.Image) image.Image {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	rect := image.Rect(0, 0, side, side)
	dst := image.NewRGBA(rect)
	draw.Draw(dst, rect, img, image.Pt(x0, y0), draw.Src)
	return dst
}

// averageBrightness returns the mean luma (0–255), sampling every 4th pixel.
func averageBrightness(img *image.RGBA) float64 {
	b := img.Bounds()
	var sum float64
	var n int
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			i := img.PixOffset(x, y)
			r, g, bl := float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2])
			sum += 0.299*r + 0.587*g + 0.114*bl
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
