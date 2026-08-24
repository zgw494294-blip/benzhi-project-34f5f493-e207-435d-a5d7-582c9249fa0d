package domain

import (
	"sort"
	"time"
)

type FindingSummary struct {
	Total        int `json:"total"`
	Passed       int `json:"passed"`
	Failed       int `json:"failed"`
	Open         int `json:"open"`
	WithEvidence int `json:"withEvidence"`
	Overdue      int `json:"overdue"`
	Rounds       int `json:"rounds"`
}

func ChecklistCodes() []string {
	codes := make([]string, 0, len(Checklist))
	for code := range Checklist {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func ValidateFirstInspection(items []InspectionFinding) error {
	if len(items) != len(Checklist) {
		return invalid("items", "首次检查必须完整覆盖现场检查清单")
	}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item.ItemCode] {
			return invalid("items", "同一检查轮次不能重复检查项")
		}
		seen[item.ItemCode] = true
	}
	for code := range Checklist {
		if !seen[code] {
			return invalid("items", "首次检查缺少检查项 "+code)
		}
	}
	return nil
}

func ValidateReinspectionReferences(open []InspectionFinding, referenceIDs []string) error {
	if len(open) == 0 {
		return invalid("items", "没有需要复检的开放整改项")
	}
	seen := map[string]bool{}
	for _, id := range referenceIDs {
		if id == "" {
			return invalid("resolvesFindingID", "复检必须关联原整改项")
		}
		if seen[id] {
			return invalid("resolvesFindingID", "不能重复关联同一整改项")
		}
		seen[id] = true
	}
	for _, finding := range open {
		if !seen[finding.ID] {
			return invalid("items", "复检必须覆盖全部开放整改项")
		}
	}
	if len(seen) != len(open) {
		return invalid("resolvesFindingID", "复检引用了非开放整改项")
	}
	return nil
}

func SummarizeFindings(findings []InspectionFinding, now time.Time) FindingSummary {
	result := FindingSummary{Total: len(findings)}
	for _, finding := range findings {
		if finding.Round > result.Rounds {
			result.Rounds = finding.Round
		}
		if finding.Result == FindingPass {
			result.Passed++
		} else {
			result.Failed++
		}
		if finding.EvidenceNote != "" {
			result.WithEvidence++
		}
		if finding.Result == FindingFail && finding.ResolvedAt == nil {
			result.Open++
			if finding.DueAt != nil && finding.DueAt.Before(now) {
				result.Overdue++
			}
		}
	}
	return result
}

func EvidenceReady(findings []InspectionFinding) bool {
	foundOpen := false
	for _, finding := range findings {
		if finding.Result != FindingFail || finding.ResolvedAt != nil {
			continue
		}
		foundOpen = true
		if finding.EvidenceNote == "" {
			return false
		}
	}
	return foundOpen
}

func LatestRound(findings []InspectionFinding) []InspectionFinding {
	round := 0
	for _, finding := range findings {
		if finding.Round > round {
			round = finding.Round
		}
	}
	result := []InspectionFinding{}
	for _, finding := range findings {
		if finding.Round == round {
			result = append(result, finding)
		}
	}
	return result
}
