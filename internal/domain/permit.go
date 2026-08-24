package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func ReviewSafety(a *Aggregate, reviewer string, passed bool, reason string, now time.Time) (Review, error) {
	if a == nil || a.Case.Status != StatusReviewing {
		return Review{}, ErrInvalidState
	}
	if strings.TrimSpace(reviewer) == "" {
		return Review{}, invalid("reviewer", "复核负责人不能为空")
	}
	if !passed && strings.TrimSpace(reason) == "" {
		return Review{}, invalid("reason", "退回整改必须填写理由")
	}
	if passed && len(OpenFindings(*a)) > 0 {
		return Review{}, invalid("findings", "仍有未关闭整改项，不能通过复核")
	}
	return Review{Reviewer: strings.TrimSpace(reviewer), Passed: passed, Reason: strings.TrimSpace(reason), ReviewedAt: now}, nil
}

func FreezePlan(plan *ElectricalPlan, now time.Time) error {
	if plan == nil || plan.FrozenAt != nil {
		return ErrInvalidState
	}
	if plan.CalculationResult != "PASS" {
		return invalid("plan", "核算未通过的方案不能冻结")
	}
	copyPlan := *plan
	copyPlan.ContentHash = ""
	copyPlan.FrozenAt = nil
	payload, err := json.Marshal(copyPlan)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	plan.ContentHash = hex.EncodeToString(sum[:])
	plan.FrozenAt = &now
	return nil
}

func IssuePermit(id string, a Aggregate, approvedBy string, now time.Time) (EnergizationPermit, error) {
	if a.Case.Status != StatusFrozen || a.Permit != nil {
		return EnergizationPermit{}, ErrInvalidState
	}
	plan, err := CurrentPlan(&a)
	if err != nil {
		return EnergizationPermit{}, err
	}
	if plan.FrozenAt == nil || plan.ContentHash == "" {
		return EnergizationPermit{}, invalid("plan", "方案尚未冻结")
	}
	if strings.TrimSpace(approvedBy) == "" {
		return EnergizationPermit{}, invalid("approvedBy", "签发人不能为空")
	}
	permit := EnergizationPermit{ID: id, CaseID: a.Case.ID, PlanID: plan.ID, PermitNumber: fmt.Sprintf("TMP-%s-%s", now.Format("20060102"), strings.ToUpper(shortID(id))), ApprovedBy: strings.TrimSpace(approvedBy), ApprovedAt: now, ValidUntil: a.Case.EndAt, Status: "VALID"}
	material := struct {
		Permit   EnergizationPermit `json:"permit"`
		PlanHash string             `json:"planHash"`
	}{Permit: permit, PlanHash: plan.ContentHash}
	payload, err := json.Marshal(material)
	if err != nil {
		return EnergizationPermit{}, err
	}
	sum := sha256.Sum256(payload)
	permit.ContentHash = hex.EncodeToString(sum[:])
	return permit, nil
}

func VerifyPermit(a Aggregate) bool {
	if a.Permit == nil {
		return false
	}
	plan, err := CurrentPlan(&a)
	if err != nil || plan.ContentHash == "" {
		return false
	}
	copyPermit := *a.Permit
	expected := copyPermit.ContentHash
	copyPermit.ContentHash = ""
	material := struct {
		Permit   EnergizationPermit `json:"permit"`
		PlanHash string             `json:"planHash"`
	}{Permit: copyPermit, PlanHash: plan.ContentHash}
	payload, err := json.Marshal(material)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(payload)
	return expected == hex.EncodeToString(sum[:])
}

func shortID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) > 8 {
		return clean[:8]
	}
	return clean
}
