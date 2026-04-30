package conflict

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/krishyogee/gitmate/internal/ai"
)

type Explanation struct {
	OursIntent          string  `json:"ours_intent"`
	TheirsIntent        string  `json:"theirs_intent"`
	ConflictType        string  `json:"conflict_type"`
	ResolutionStrategy  string  `json:"resolution_strategy"`
	ResolutionRationale string  `json:"resolution_rationale"`
	CandidatePatch      string  `json:"candidate_patch"`
	Confidence          float64 `json:"confidence"`
	RiskNotes           string  `json:"risk_notes"`
}

func Explain(ctx context.Context, client *ai.Client, b Block) (*Explanation, error) {
	if !client.HasProvider() {
		return nil, fmt.Errorf("no AI provider configured (set ANTHROPIC_API_KEY)")
	}
	user := fmt.Sprintf(`Conflict block:
<ours>
%s
</ours>

<theirs>
%s
</theirs>

Surrounding context:
%s

File: %s
Language: %s`,
		strings.Join(b.OursLines, "\n"),
		strings.Join(b.TheirsLines, "\n"),
		b.SurroundingContext,
		b.FilePath,
		b.Language,
	)
	user = ai.RedactSecrets(user)

	out, err := client.Complete(ctx, ai.ConflictExplainerPrompt, user, "conflict_analysis")
	if err != nil {
		return nil, err
	}
	out = stripJSONFence(out)

	var ex Explanation
	if err := json.Unmarshal([]byte(out), &ex); err != nil {
		return nil, fmt.Errorf("decode explanation: %w (raw=%s)", err, out)
	}
	return &ex, nil
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx > 0 {
			s = s[idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
