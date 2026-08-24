package domain

import (
	"strings"
	"time"
)

func NewCase(id, activity, venue string, start, end time.Time, contact Contact, risk RiskLevel, now time.Time) (PowerPermitCase, error) {
	if strings.TrimSpace(id) == "" {
		return PowerPermitCase{}, invalid("id", "不能为空")
	}
	if strings.TrimSpace(activity) == "" {
		return PowerPermitCase{}, invalid("activityName", "请填写活动名称")
	}
	if strings.TrimSpace(venue) == "" {
		return PowerPermitCase{}, invalid("venue", "请填写活动场地")
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return PowerPermitCase{}, invalid("endAt", "结束时间必须晚于开始时间")
	}
	if strings.TrimSpace(contact.Name) == "" || strings.TrimSpace(contact.Phone) == "" {
		return PowerPermitCase{}, invalid("contact", "请填写联系人和电话")
	}
	if risk != RiskLow && risk != RiskMedium && risk != RiskHigh {
		return PowerPermitCase{}, invalid("riskLevel", "风险等级必须为 LOW、MEDIUM 或 HIGH")
	}
	return PowerPermitCase{ID: id, ActivityName: strings.TrimSpace(activity), Venue: strings.TrimSpace(venue), StartAt: start, EndAt: end, Contact: contact, RiskLevel: risk, Status: StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func CanTransition(from, to CaseStatus) bool {
	allowed := map[CaseStatus][]CaseStatus{
		StatusDraft:      {StatusPlanned},
		StatusPlanned:    {StatusInspecting},
		StatusInspecting: {StatusRectifying, StatusReviewing},
		StatusRectifying: {StatusInspecting},
		StatusReviewing:  {StatusRectifying, StatusFrozen},
		StatusFrozen:     {StatusPermitted},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func Transition(c *PowerPermitCase, to CaseStatus, now time.Time) error {
	if c == nil {
		return invalid("case", "案件不能为空")
	}
	if !CanTransition(c.Status, to) {
		return ErrInvalidState
	}
	c.Status = to
	c.Version++
	c.UpdatedAt = now
	return nil
}

func CurrentPlan(a *Aggregate) (*ElectricalPlan, error) {
	for i := range a.Plans {
		if a.Plans[i].ID == a.Case.CurrentPlanID {
			return &a.Plans[i], nil
		}
	}
	return nil, invalid("plan", "当前方案不存在")
}

func OpenFindings(a Aggregate) []InspectionFinding {
	var result []InspectionFinding
	for _, finding := range a.Findings {
		if finding.Result == FindingFail && finding.ResolvedAt == nil {
			result = append(result, finding)
		}
	}
	return result
}
