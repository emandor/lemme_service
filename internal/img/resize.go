package img

import (
	"crypto/sha256"
	"encoding/hex"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

type SaveResult struct {
	Path          string
	Hash          string
	Width, Height int
}

func SaveResizedJPEG(srcPath, dstDir string, maxW int) (SaveResult, error) {
	im, err := imaging.Open(srcPath)
	if err != nil {
		return SaveResult{}, err
	}
	w := im.Bounds().Dx()
	var out image.Image
	if w > maxW {
		out = imaging.Resize(im, maxW, 0, imaging.Lanczos)
	} else {
		out = im
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return SaveResult{}, err
	}
	tmp := filepath.Join(dstDir, filepath.Base(srcPath)+".jpg")
	if err := imaging.Save(out, tmp, imaging.JPEGQuality(85)); err != nil {
		return SaveResult{}, err
	}

	b, _ := os.ReadFile(tmp)
	h := sha256.Sum256(b)
	return SaveResult{Path: tmp, Hash: hex.EncodeToString(h[:]), Width: out.Bounds().Dx(), Height: out.Bounds().Dy()}, nil
}
