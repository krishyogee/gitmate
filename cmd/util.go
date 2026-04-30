package cmd

import (
	"encoding/json"
	"strings"
)

func jsonUnmarshalLenient(raw string, v any) error {
	raw = strings.TrimSpace(raw)
	return json.Unmarshal([]byte(raw), v)
}
