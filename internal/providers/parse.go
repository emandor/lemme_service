package providers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TryParseAnswer attempts various strategies to normalize LLM answers.
// Priorities: JSON -> code fence JSON -> first JSON object -> "Answer:"/"Jawaban:" pattern -> letter/number/boolean -> fallback raw.
func TryParseAnswer(content string) (Answer, error) {
	ans := Answer{Raw: strings.TrimSpace(content)}

	if tryJSON(content, &ans) {
		normalize(&ans)
		return ans, nil
	}

	if s := extractCodeFenceJSON(content); s != "" && tryJSON(s, &ans) {
		normalize(&ans)
		return ans, nil
	}

	if s := extractFirstJSONObject(content); s != "" && tryJSON(s, &ans) {
		normalize(&ans)
		return ans, nil
	}

	if a, r := parseColonStyle(content); a != "" {
		ans.Answer, ans.Reason = a, r
		normalize(&ans)
		return ans, nil
	}

	if a := parseSimpleFinal(content); a != "" {
		ans.Answer = a
		normalize(&ans)
		return ans, nil
	}

	ans.Answer = truncateSingleLine(ans.Raw, 500)
	normalize(&ans)
	return ans, nil
}

func tryJSON(s string, out *Answer) bool {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) == nil {
		if v, ok := m["answer"]; ok {
			out.Answer = str(v)
		}
		if v, ok := m["reason"]; ok {
			out.Reason = str(v)
		}
		if v, ok := m["options"]; ok {
			switch vv := v.(type) {
			case []any:
				for _, it := range vv {
					out.Options = append(out.Options, str(it))
				}
			case []string:
				out.Options = vv
			}
		}
		if v, ok := m["confidence"]; ok {
			out.Confidence = toFloat(v)
		}
		return out.Answer != ""
	}
	return json.Unmarshal([]byte(s), out) == nil && out.Answer != ""
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strings.TrimSuffix(strings.TrimSuffix(formatFloat(t), ".0"), ".00")
		}
		return formatFloat(t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
func formatFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}
func toFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f
		}
	}
	return 0
}

var rxFence = regexp.MustCompile("(?is)```json\\s*(\\{[\\s\\S]*?\\})\\s*```")

func extractCodeFenceJSON(s string) string {
	m := rxFence.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

// find the first JSON object by simple brace balancing
func extractFirstJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	level := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			level++
		case '}':
			level--
			if level == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

var rxAnsColon = regexp.MustCompile(`(?im)^(?:answer|jawaban)\s*[:：]\s*(.+)$`)
var rxReaColon = regexp.MustCompile(`(?im)^(?:reason|penjelasan)\s*[:：]\s*(.+)$`)

func parseColonStyle(s string) (answer, reason string) {
	if m := rxAnsColon.FindStringSubmatch(s); len(m) > 1 {
		answer = strings.TrimSpace(m[1])
	}
	if m := rxReaColon.FindStringSubmatch(s); len(m) > 1 {
		reason = strings.TrimSpace(m[1])
	}
	return
}

var (
	rxLetter = regexp.MustCompile(`(?i)\b([A-Z])\b`)       // A..Z
	rxNum    = regexp.MustCompile(`(?i)\b([1-9][0-9]?)\b`) // 1..99
	rxBool   = regexp.MustCompile(`(?i)\b(true|false|ya|tidak|benar|salah)\b`)
	rxFinal  = regexp.MustCompile(`(?i)\b(final|jawaban|answer)\b[:：]?\s*([A-Z]|[1-9][0-9]?|true|false|ya|tidak|benar|salah)\b`)
)

func parseSimpleFinal(s string) string {
	if m := rxFinal.FindStringSubmatch(s); len(m) > 2 {
		return normalizeToken(m[2])
	}
	// fallback: get the first token that looks valid
	if m := rxBool.FindStringSubmatch(s); len(m) > 1 {
		return normalizeToken(m[1])
	}
	if m := rxLetter.FindStringSubmatch(s); len(m) > 1 {
		return strings.ToUpper(m[1])
	}
	if m := rxNum.FindStringSubmatch(s); len(m) > 1 {
		return m[1]
	}
	return ""
}

func normalizeToken(t string) string {
	t = strings.TrimSpace(strings.ToLower(t))
	switch t {
	case "true", "ya", "benar":
		return "True"
	case "false", "tidak", "salah":
		return "False"
	default:
		// A..Z or numbers
		if len(t) == 1 && t[0] >= 'a' && t[0] <= 'z' {
			return strings.ToUpper(t)
		}
		return t
	}
}

func truncateSingleLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func normalize(a *Answer) {
	// clean up answer casing (A..Z), or boolean, or leave free text
	a.Answer = strings.TrimSpace(a.Answer)
	// delete wrapping quotes leftover from JSON in Raw
	a.Raw = strings.TrimSpace(a.Raw)
}
