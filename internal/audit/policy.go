package audit

import (
	"errors"
	"fmt"
	"sort"

	"powerpermit/internal/domain"
)

type ActionPolicy struct {
	Action        string              `json:"action"`
	AllowedFrom   []domain.CaseStatus `json:"allowedFrom"`
	ResultStatus  domain.CaseStatus   `json:"resultStatus"`
	ChangesStatus bool                `json:"changesStatus"`
}

var commandPolicies = map[string]ActionPolicy{
	"CASE_CREATED":           {Action: "CASE_CREATED", ResultStatus: domain.StatusDraft, ChangesStatus: true},
	"PLAN_SUBMITTED":         {Action: "PLAN_SUBMITTED", AllowedFrom: []domain.CaseStatus{domain.StatusDraft}, ResultStatus: domain.StatusPlanned, ChangesStatus: true},
	"INSPECTION_STARTED":     {Action: "INSPECTION_STARTED", AllowedFrom: []domain.CaseStatus{domain.StatusPlanned, domain.StatusRectifying}, ResultStatus: domain.StatusInspecting, ChangesStatus: true},
	"INSPECTION_RECORDED":    {Action: "INSPECTION_RECORDED", AllowedFrom: []domain.CaseStatus{domain.StatusInspecting}, ChangesStatus: true},
	"EVIDENCE_SUBMITTED":     {Action: "EVIDENCE_SUBMITTED", AllowedFrom: []domain.CaseStatus{domain.StatusRectifying}, ResultStatus: domain.StatusRectifying},
	"SAFETY_REVIEW_PASSED":   {Action: "SAFETY_REVIEW_PASSED", AllowedFrom: []domain.CaseStatus{domain.StatusReviewing}, ResultStatus: domain.StatusFrozen, ChangesStatus: true},
	"SAFETY_REVIEW_RETURNED": {Action: "SAFETY_REVIEW_RETURNED", AllowedFrom: []domain.CaseStatus{domain.StatusReviewing}, ResultStatus: domain.StatusRectifying, ChangesStatus: true},
	"PERMIT_ISSUED":          {Action: "PERMIT_ISSUED", AllowedFrom: []domain.CaseStatus{domain.StatusFrozen}, ResultStatus: domain.StatusPermitted, ChangesStatus: true},
}

func Policies() []ActionPolicy {
	result := make([]ActionPolicy, 0, len(commandPolicies))
	for _, policy := range commandPolicies {
		result = append(result, policy)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Action < result[j].Action })
	return result
}

func ValidateActionSequence(events []Event) error {
	if len(events) == 0 {
		return nil
	}
	current := domain.CaseStatus("")
	for index, event := range events {
		policy, ok := commandPolicies[event.Action]
		if !ok {
			return fmt.Errorf("第 %d 条审计事件包含未知动作 %s", index+1, event.Action)
		}
		from := domain.CaseStatus(event.FromStatus)
		to := domain.CaseStatus(event.ToStatus)
		if index == 0 {
			if event.Action != "CASE_CREATED" || from != "" || to != domain.StatusDraft {
				return errors.New("审计时间线必须从草案建档开始")
			}
			current = to
			continue
		}
		if from != current {
			return fmt.Errorf("第 %d 条事件的起始状态与上一事件不一致", index+1)
		}
		if !containsStatus(policy.AllowedFrom, from) {
			return fmt.Errorf("动作 %s 不允许从状态 %s 执行", event.Action, from)
		}
		if event.Action == "INSPECTION_RECORDED" {
			if to != domain.StatusRectifying && to != domain.StatusReviewing {
				return errors.New("检查结果只能进入整改或复核状态")
			}
		} else if policy.ChangesStatus && to != policy.ResultStatus {
			return fmt.Errorf("动作 %s 的结果状态应为 %s", event.Action, policy.ResultStatus)
		} else if !policy.ChangesStatus && to != from {
			return fmt.Errorf("动作 %s 不应改变案件状态", event.Action)
		}
		current = to
	}
	return nil
}

func containsStatus(values []domain.CaseStatus, target domain.CaseStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func LastState(events []Event) (domain.CaseStatus, int64, error) {
	if err := ValidateActionSequence(events); err != nil {
		return "", 0, err
	}
	if len(events) == 0 {
		return "", 0, nil
	}
	last := events[len(events)-1]
	return domain.CaseStatus(last.ToStatus), last.ToVersion, nil
}
