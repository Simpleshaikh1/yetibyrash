package media

import (
	"encoding/binary"
	"image"
)

// exifOrientation reads the EXIF Orientation tag (1–8) from JPEG bytes.
// Phones save photos sideways and record the rotation here; 1 means "already upright".
func exifOrientation(jpg []byte) int {
	// Walk JPEG segments looking for APP1 "Exif".
	if len(jpg) < 4 || jpg[0] != 0xFF || jpg[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(jpg) {
		if jpg[i] != 0xFF {
			return 1
		}
		marker := jpg[i+1]
		size := int(binary.BigEndian.Uint16(jpg[i+2:]))
		if marker == 0xDA || size < 2 || i+2+size > len(jpg) { // start of image data, or broken
			return 1
		}
		seg := jpg[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return orientationFromTIFF(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func orientationFromTIFF(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[ifd:]))
	for k := 0; k < n; k++ {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 { // Orientation
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation rotates/flips img so it displays upright.
func applyOrientation(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := o >= 5 // orientations 5–8 turn the image on its side
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2: // mirrored
				dx, dy = w-1-x, y
			case 3: // upside down
				dx, dy = w-1-x, h-1-y
			case 4: // upside down, mirrored
				dx, dy = x, h-1-y
			case 5: // mirrored, rotated 90° anticlockwise
				dx, dy = y, x
			case 6: // rotated 90° clockwise to view
				dx, dy = h-1-y, x
			case 7: // mirrored, rotated 90° clockwise
				dx, dy = h-1-y, w-1-x
			case 8: // rotated 90° anticlockwise to view
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
