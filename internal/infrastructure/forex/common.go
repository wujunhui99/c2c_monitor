package forex

import (
	"fmt"
	"io"
	"strings"
)

const maxForexResponseBytes int64 = 1 << 20

func readResponseBody(body io.Reader) ([]byte, error) {
	limited := &io.LimitedReader{R: body, N: maxForexResponseBytes + 1}
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > maxForexResponseBytes {
		return nil, fmt.Errorf("forex response exceeds %d bytes", maxForexResponseBytes)
	}
	return payload, nil
}

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
