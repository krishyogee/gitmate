package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/krishyogee/gitmate/internal/tui"
)

func jsonUnmarshalLenient(raw string, v any) error {
	raw = strings.TrimSpace(raw)
	return json.Unmarshal([]byte(raw), v)
}

func scoreLabel(score float64) string {
	label := fmt.Sprintf("%.2f", score)
	switch {
	case score >= 0.8:
		return tui.OK.Render(label)
	case score >= 0.4:
		return tui.Warn.Render(label)
	default:
		return tui.Err.Render(label)
	}
}
