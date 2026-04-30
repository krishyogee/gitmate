package observability

import (
	"bufio"
	"encoding/json"
	"os"
)

type Metrics struct {
	TotalCalls         int                `json:"total_calls"`
	SuccessRate        float64            `json:"success_rate"`
	ApprovalRate       float64            `json:"approval_rate"`
	EditRate           float64            `json:"edit_rate"`
	FallbackRate       float64            `json:"fallback_rate"`
	AvgLatencyMs       float64            `json:"avg_latency_ms"`
	AvgScore           float64            `json:"avg_score"`
	ScoreByAction      map[string]float64 `json:"score_by_action"`
	ApprovalByAction   map[string]float64 `json:"approval_by_action"`
}

func ComputeMetrics(logPath string) (*Metrics, error) {
	m := &Metrics{ScoreByAction: map[string]float64{}, ApprovalByAction: map[string]float64{}}
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	defer f.Close()
	scoreSum := map[string]float64{}
	scoreCount := map[string]int{}
	approvalSum := map[string]int{}
	approvalCount := map[string]int{}

	totalSuccess := 0
	totalApproved := 0
	totalApproval := 0
	totalEdited := 0
	totalLatency := int64(0)
	totalLatencyCount := 0
	totalScoreSum := 0.0
	totalScoreCount := 0
	fallbacks := 0

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var e LogEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		m.TotalCalls++
		if e.Success {
			totalSuccess++
		}
		if e.LatencyMs > 0 {
			totalLatency += e.LatencyMs
			totalLatencyCount++
		}
		if e.Score > 0 {
			totalScoreSum += e.Score
			totalScoreCount++
			scoreSum[e.Action] += e.Score
			scoreCount[e.Action]++
		}
		if e.UserAction != "" {
			totalApproval++
			approvalCount[e.Action]++
			if e.UserAction == "approved" || e.UserAction == "session" {
				totalApproved++
				approvalSum[e.Action]++
			}
			if e.UserAction == "edited" {
				totalEdited++
			}
		}
		if e.Note == "fallback" {
			fallbacks++
		}
	}

	if m.TotalCalls > 0 {
		m.SuccessRate = float64(totalSuccess) / float64(m.TotalCalls)
		m.FallbackRate = float64(fallbacks) / float64(m.TotalCalls)
	}
	if totalApproval > 0 {
		m.ApprovalRate = float64(totalApproved) / float64(totalApproval)
		m.EditRate = float64(totalEdited) / float64(totalApproval)
	}
	if totalLatencyCount > 0 {
		m.AvgLatencyMs = float64(totalLatency) / float64(totalLatencyCount)
	}
	if totalScoreCount > 0 {
		m.AvgScore = totalScoreSum / float64(totalScoreCount)
	}
	for k, v := range scoreSum {
		m.ScoreByAction[k] = v / float64(scoreCount[k])
	}
	for k, v := range approvalSum {
		if approvalCount[k] > 0 {
			m.ApprovalByAction[k] = float64(v) / float64(approvalCount[k])
		}
	}
	return m, nil
}
