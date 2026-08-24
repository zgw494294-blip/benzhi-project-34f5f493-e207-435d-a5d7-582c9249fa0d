package application

import (
	"encoding/json"
	"fmt"
	"time"

	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

type Dashboard struct {
	Statistics  storage.Statistics `json:"statistics"`
	Recent      []domain.Aggregate `json:"recent"`
	GeneratedAt time.Time          `json:"generatedAt"`
}

type Readiness struct {
	CaseID         string   `json:"caseID"`
	CanInspect     bool     `json:"canInspect"`
	CanReview      bool     `json:"canReview"`
	CanIssue       bool     `json:"canIssue"`
	PlanFrozen     bool     `json:"planFrozen"`
	PermitVerified bool     `json:"permitVerified"`
	OpenFindingIDs []string `json:"openFindingIDs"`
	Reasons        []string `json:"reasons"`
}

func (s *Service) Dashboard() (Dashboard, error) {
	recent, err := s.repo.List("")
	if err != nil {
		return Dashboard{}, err
	}
	if len(recent) > 8 {
		recent = recent[:8]
	}
	return Dashboard{Statistics: s.repo.Statistics(s.now()), Recent: recent, GeneratedAt: s.now()}, nil
}

func (s *Service) Search(query string, status domain.CaseStatus) ([]domain.Aggregate, error) {
	return s.repo.Search(query, status)
}

func (s *Service) Readiness(caseID string) (Readiness, error) {
	aggregate, err := s.repo.Get(caseID)
	if err != nil {
		return Readiness{}, err
	}
	result := Readiness{CaseID: caseID}
	plan, planErr := domain.CurrentPlan(&aggregate)
	if planErr == nil {
		result.CanInspect = aggregate.Case.Status == domain.StatusPlanned || aggregate.Case.Status == domain.StatusRectifying
		result.PlanFrozen = plan.FrozenAt != nil
	}
	for _, finding := range domain.OpenFindings(aggregate) {
		result.OpenFindingIDs = append(result.OpenFindingIDs, finding.ID)
	}
	result.CanReview = aggregate.Case.Status == domain.StatusReviewing && len(result.OpenFindingIDs) == 0
	result.CanIssue = aggregate.Case.Status == domain.StatusFrozen && result.PlanFrozen && aggregate.Permit == nil
	if aggregate.Permit != nil {
		result.PermitVerified = domain.VerifyPermit(aggregate)
	}
	if planErr != nil {
		result.Reasons = append(result.Reasons, "尚未形成当前方案")
	}
	if len(result.OpenFindingIDs) > 0 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("仍有 %d 项整改未关闭", len(result.OpenFindingIDs)))
	}
	if aggregate.Case.Status == domain.StatusPermitted && !result.PermitVerified {
		result.Reasons = append(result.Reasons, "许可内容摘要校验失败")
	}
	return result, nil
}

func (s *Service) PersistentTimeline(caseID string) ([]audit.Event, audit.TimelineDigest, error) {
	if _, err := s.repo.Get(caseID); err != nil {
		return nil, audit.TimelineDigest{}, err
	}
	events, err := s.repo.Events(caseID)
	if err != nil {
		return nil, audit.TimelineDigest{}, err
	}
	result := make([]audit.Event, 0, len(events))
	previousStatus := ""
	for _, stored := range events {
		var aggregate domain.Aggregate
		if err := json.Unmarshal(stored.Payload, &aggregate); err != nil {
			return nil, audit.TimelineDigest{}, err
		}
		result = append(result, audit.Event{ID: stored.Hash, CaseID: stored.CaseID, Action: stored.Kind, Actor: stored.Actor, RequestKey: stored.IdempotencyKey, FromVersion: stored.Version - 1, ToVersion: stored.Version, FromStatus: previousStatus, ToStatus: string(aggregate.Case.Status), At: stored.OccurredAt})
		previousStatus = string(aggregate.Case.Status)
	}
	digest, err := audit.DigestTimeline(caseID, result)
	return result, digest, err
}

func (s *Service) FindPermit(number string) (domain.Aggregate, bool, error) {
	aggregate, err := s.repo.FindPermit(number)
	if err != nil {
		return domain.Aggregate{}, false, err
	}
	return aggregate, domain.VerifyPermit(aggregate), nil
}
