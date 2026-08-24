package application

import (
	"fmt"
	"powerpermit/internal/domain"
)

func (s *Service) RecordInspection(caseID string, command RecordInspectionCommand) (CommandResponse, error) {
	if err := validateMeta(command.Meta, false); err != nil {
		return CommandResponse{}, err
	}
	if len(command.Items) == 0 {
		return CommandResponse{}, &domain.ValidationError{Field: "items", Message: "至少提交一个检查项"}
	}
	if response, duplicate, err := s.verifyKey(command.IdempotencyKey, caseID); duplicate {
		return response, err
	}
	aggregate, err := s.repo.Get(caseID)
	if err != nil {
		return CommandResponse{}, err
	}
	if aggregate.Case.Status != domain.StatusInspecting {
		return CommandResponse{}, domain.ErrInvalidState
	}
	plan, err := domain.CurrentPlan(&aggregate)
	if err != nil {
		return CommandResponse{}, err
	}
	before := aggregate
	round := domain.NextInspectionRound(aggregate.Findings)
	openBefore := domain.OpenFindings(aggregate)
	failed := false
	created := make([]domain.InspectionFinding, 0, len(command.Items))
	references := make([]string, 0, len(command.Items))
	for _, item := range command.Items {
		finding, createErr := domain.NewFinding(s.id(), caseID, plan.ID, round, item.ItemCode, item.MeasuredValue, item.PhotoNote, item.Result, item.FindingText, item.Assignee, item.DueAt, command.Actor, s.now())
		if createErr != nil {
			return CommandResponse{}, createErr
		}
		if item.ResolvesFindingID != "" {
			references = append(references, item.ResolvesFindingID)
			index := findingIndex(aggregate.Findings, item.ResolvesFindingID)
			if index < 0 {
				return CommandResponse{}, &domain.ValidationError{Field: "resolvesFindingID", Message: "原整改项不存在"}
			}
			if item.Result != domain.FindingPass {
				failed = true
			} else if err := domain.ResolveFinding(&aggregate.Findings[index], s.now()); err != nil {
				return CommandResponse{}, err
			}
		}
		if item.Result == domain.FindingFail {
			failed = true
		}
		created = append(created, finding)
	}
	if round == 1 {
		if err := domain.ValidateFirstInspection(created); err != nil {
			return CommandResponse{}, err
		}
	} else if err := domain.ValidateReinspectionReferences(openBefore, references); err != nil {
		return CommandResponse{}, err
	}
	aggregate.Findings = append(aggregate.Findings, created...)
	target := domain.StatusReviewing
	if failed || len(domain.OpenFindings(aggregate)) > 0 {
		target = domain.StatusRectifying
	}
	if err := domain.Transition(&aggregate.Case, target, s.now()); err != nil {
		return CommandResponse{}, err
	}
	return s.commit(command.Meta, before, aggregate, "INSPECTION_RECORDED", fmt.Sprintf("保存第 %d 轮现场检查，共 %d 项", round, len(command.Items)))
}

func (s *Service) SubmitEvidence(caseID, findingID string, command EvidenceCommand) (CommandResponse, error) {
	if err := validateMeta(command.Meta, false); err != nil {
		return CommandResponse{}, err
	}
	if response, duplicate, err := s.verifyKey(command.IdempotencyKey, caseID); duplicate {
		return response, err
	}
	aggregate, err := s.repo.Get(caseID)
	if err != nil {
		return CommandResponse{}, err
	}
	if aggregate.Case.Status != domain.StatusRectifying {
		return CommandResponse{}, domain.ErrInvalidState
	}
	index := findingIndex(aggregate.Findings, findingID)
	if index < 0 {
		return CommandResponse{}, domain.ErrNotFound
	}
	before := aggregate
	if err := domain.SubmitEvidence(&aggregate.Findings[index], command.Actor, command.Note); err != nil {
		return CommandResponse{}, err
	}
	touch(&aggregate.Case, s.now())
	return s.commit(command.Meta, before, aggregate, "EVIDENCE_SUBMITTED", "整改责任人提交证据")
}

func findingIndex(findings []domain.InspectionFinding, id string) int {
	for i := range findings {
		if findings[i].ID == id {
			return i
		}
	}
	return -1
}
