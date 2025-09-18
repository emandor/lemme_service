package img

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"

	"github.com/disintegration/imaging"
)

type Prepared struct {
	Bytes []byte
	MIME  string
}

// PrepareForOCR: resize → grayscale (optional) → low quality JPEG (token saving)
func PrepareForOCR(path string, maxW, quality int, grayscale bool) (Prepared, error) {
	src, err := imaging.Open(path, imaging.AutoOrientation(true))
	if err != nil {
		return Prepared{}, err
	}

	// resize proportional
	if src.Bounds().Dx() > maxW && maxW > 0 {
		src = imaging.Resize(src, maxW, 0, imaging.Lanczos)
	}

	if grayscale {
		src = imaging.AdjustSaturation(src, -100) // grayscale quickly
		// or: src = imaging.Grayscale(src)
	}

	// we can try some blur to reduce noise
	// src = imaging.Blur(src, 0.6)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, forceOpaque(src), &jpeg.Options{Quality: clamp(quality, 40, 85)}); err != nil {
		return Prepared{}, err
	}
	return Prepared{Bytes: buf.Bytes(), MIME: "image/jpeg"}, nil
}

// convert alpha to white (avoid unnecessary alpha cost)
func forceOpaque(img image.Image) image.Image {
	b := img.Bounds()
	dst := image.NewRGBA(b)
	bg := color.White
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a == 0 {
				dst.Set(x, y, bg)
			} else {
				dst.SetRGBA(x, y, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 0xff})
			}
		}
	}
	return dst
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// helper if you need []byte from path (e.g. fallback)
func MustRead(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}
