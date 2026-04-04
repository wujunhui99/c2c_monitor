package forex

import "strings"

func responseSnippet(body []byte) string {
	snippet := strings.TrimSpace(string(body))
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\t", " ")
	if len(snippet) > 180 {
		return snippet[:180] + "..."
	}
	if snippet == "" {
		return "empty response"
	}
	return snippet
}
