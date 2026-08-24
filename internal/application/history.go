package application

import (
	"fmt"

	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
)

type CaseHistory struct {
	Case           domain.PowerPermitCase `json:"case"`
	CurrentPlan    *domain.ElectricalPlan `json:"currentPlan,omitempty"`
	FindingSummary domain.FindingSummary  `json:"findingSummary"`
	Readiness      Readiness              `json:"readiness"`
	Timeline       []audit.Event          `json:"timeline"`
	TimelineDigest audit.TimelineDigest   `json:"timelineDigest"`
	IntegrityOK    bool                   `json:"integrityOK"`
}

func (s *Service) History(caseID string) (CaseHistory, error) {
	aggregate, err := s.repo.Get(caseID)
	if err != nil {
		return CaseHistory{}, err
	}
	readiness, err := s.Readiness(caseID)
	if err != nil {
		return CaseHistory{}, err
	}
	events, digest, err := s.PersistentTimeline(caseID)
	if err != nil {
		return CaseHistory{}, err
	}
	if err := audit.ValidateActionSequence(events); err != nil {
		return CaseHistory{}, fmt.Errorf("审计状态链校验失败: %w", err)
	}
	status, version, err := audit.LastState(events)
	if err != nil {
		return CaseHistory{}, err
	}
	result := CaseHistory{Case: aggregate.Case, FindingSummary: domain.SummarizeFindings(aggregate.Findings, s.now()), Readiness: readiness, Timeline: events, TimelineDigest: digest, IntegrityOK: status == aggregate.Case.Status && version == aggregate.Case.Version}
	if aggregate.Case.CurrentPlanID != "" {
		plan, planErr := domain.CurrentPlan(&aggregate)
		if planErr != nil {
			return CaseHistory{}, planErr
		}
		copyPlan := *plan
		result.CurrentPlan = &copyPlan
	}
	if !result.IntegrityOK {
		return CaseHistory{}, fmt.Errorf("事件时间线与案件投影不一致")
	}
	return result, nil
}
