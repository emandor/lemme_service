package providers

import (
	"strings"
)

// instruction to make all LLMs reply in single-line JSON, no code fence.
const JSON_INSTRUCTION = `Return ONLY a single-line JSON object with keys:
"answer": string (may be "A".."Z", number like "2", boolean "True"/"False", or free text),
"reason": string (optional, brief).
No Markdown, no code fences, no extra text. please use Indonesian if the question is in Indonesian.`

// BuildPromptWithChoices: user OCR text + optional choices to build prompt.
// if choices is empty or nil, it will be ignored.
func BuildPromptWithChoices(ocr string, choices []string) string {
	var b strings.Builder
	b.WriteString(JSON_INSTRUCTION)
	b.WriteString("\n\nSolve this quiz based on the OCR text below.\n")
	if len(choices) > 0 {
		b.WriteString("If appropriate, select ONLY ONE from these options:\n")
		for i, c := range choices {
			// show as A if letter, else just show the text
			b.WriteString("- ")
			// if there is "A/B/C..." just show as is
			b.WriteString(c)
			if i < len(choices)-1 {
				b.WriteByte('\n')
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\nOCR:\n")
	b.WriteString(ocr)
	b.WriteString("\n")
	return b.String()
}

// BuildPrompt: user OCR text only, no choices.
func BuildPrompt(ocr string) string {
	return BuildPromptWithChoices(ocr, nil)
}
