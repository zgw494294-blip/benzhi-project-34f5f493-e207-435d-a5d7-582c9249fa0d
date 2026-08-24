package application

import (
	"fmt"
	"powerpermit/internal/domain"
)

func (s *Service) SubmitPlan(caseID string, command SubmitPlanCommand) (CommandResponse, error) {
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
	if aggregate.Case.Status != domain.StatusDraft {
		return CommandResponse{}, domain.ErrInvalidState
	}
	revision := len(aggregate.Plans) + 1
	plan, err := domain.BuildPlan(s.id(), caseID, revision, command.Circuits, command.DesignCapacityKVA, s.now())
	if err != nil {
		return CommandResponse{}, err
	}
	if plan.CalculationResult != "PASS" {
		return CommandResponse{}, &domain.ValidationError{Field: "plan", Message: "容量或保护参数核算未通过"}
	}
	before := aggregate
	aggregate.Plans = append(aggregate.Plans, plan)
	aggregate.Case.CurrentPlanID = plan.ID
	if err := domain.Transition(&aggregate.Case, domain.StatusPlanned, s.now()); err != nil {
		return CommandResponse{}, err
	}
	return s.commit(command.Meta, before, aggregate, "PLAN_SUBMITTED", fmt.Sprintf("提交方案第 %d 版，总负荷 %.2fkW", revision, plan.TotalLoadKW))
}

func (s *Service) StartInspection(caseID string, command StartInspectionCommand) (CommandResponse, error) {
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
	if aggregate.Case.Status != domain.StatusPlanned && aggregate.Case.Status != domain.StatusRectifying {
		return CommandResponse{}, domain.ErrInvalidState
	}
	if aggregate.Case.Status == domain.StatusRectifying && !domain.EvidenceReady(aggregate.Findings) {
		return CommandResponse{}, &domain.ValidationError{Field: "findings", Message: "全部开放整改项提交证据后才能发起复检"}
	}
	before := aggregate
	if err := domain.Transition(&aggregate.Case, domain.StatusInspecting, s.now()); err != nil {
		return CommandResponse{}, err
	}
	return s.commit(command.Meta, before, aggregate, "INSPECTION_STARTED", "方案进入现场检查")
}
