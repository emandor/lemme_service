package ocr

import "github.com/otiai10/gosseract/v2"

// currently we dont use this, we use openai vision
// ExtractText from image at path using Tesseract OCR with specified language
func ExtractText(path, lang string) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()
	client.SetLanguage(lang)
	client.SetImage(path)
	return client.Text()
}
