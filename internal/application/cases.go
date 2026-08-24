package application

import (
	"fmt"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

func (s *Service) CreateCase(command CreateCaseCommand) (CommandResponse, error) {
	if err := validateMeta(command.Meta, true); err != nil {
		return CommandResponse{}, err
	}
	if response, duplicate, err := s.verifyKey(command.IdempotencyKey, ""); duplicate {
		return response, err
	}
	now := s.now()
	caseID := s.id()
	item, err := domain.NewCase(caseID, command.ActivityName, command.Venue, command.StartAt, command.EndAt, command.Contact, command.RiskLevel, now)
	if err != nil {
		return CommandResponse{}, err
	}
	aggregate := domain.Aggregate{Case: item, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
	_, duplicate, err := s.repo.Commit(storage.Mutation{CaseID: caseID, ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: command.Actor, IdempotencyKey: command.IdempotencyKey, Aggregate: aggregate, Response: aggregate})
	if err != nil {
		return CommandResponse{}, err
	}
	if duplicate {
		response, _, getErr := s.verifyKey(command.IdempotencyKey, "")
		return response, getErr
	}
	s.audit.Record(newAudit(s, aggregate, command.Meta, "CASE_CREATED", "建立临时用电档案"))
	return CommandResponse{Case: aggregate}, nil
}

func newAudit(s *Service, aggregate domain.Aggregate, meta Meta, action, detail string) audit.Event {
	return audit.Event{ID: s.id(), CaseID: aggregate.Case.ID, Action: action, Actor: meta.Actor, RequestKey: meta.IdempotencyKey, FromVersion: aggregate.Case.Version - 1, ToVersion: aggregate.Case.Version, FromStatus: "", ToStatus: string(aggregate.Case.Status), At: s.now(), Detail: detail}
}

func (s *Service) AssertHealthy() error {
	if err := s.repo.Verify(); err != nil {
		return fmt.Errorf("存储校验失败: %w", err)
	}
	return nil
}
