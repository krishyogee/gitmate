package ai

func TaskTypeFor(action string) string {
	switch action {
	case "generate_commit", "refine_commit", "create_pr":
		return "commit_draft"
	case "explain_conflict", "resolve_conflict":
		return "conflict_analysis"
	case "explain_diff":
		return "explain"
	default:
		return "planning"
	}
}
