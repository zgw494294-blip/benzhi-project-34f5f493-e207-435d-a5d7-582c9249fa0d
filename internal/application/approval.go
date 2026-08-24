package application

import "powerpermit/internal/domain"

func (s *Service) Review(caseID string, command ReviewCommand) (CommandResponse, error) {
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
	review, err := domain.ReviewSafety(&aggregate, command.Actor, command.Passed, command.Reason, s.now())
	if err != nil {
		return CommandResponse{}, err
	}
	before := aggregate
	aggregate.Reviews = append(aggregate.Reviews, review)
	if command.Passed {
		plan, planErr := domain.CurrentPlan(&aggregate)
		if planErr != nil {
			return CommandResponse{}, planErr
		}
		if freezeErr := domain.FreezePlan(plan, s.now()); freezeErr != nil {
			return CommandResponse{}, freezeErr
		}
		if transitionErr := domain.Transition(&aggregate.Case, domain.StatusFrozen, s.now()); transitionErr != nil {
			return CommandResponse{}, transitionErr
		}
		return s.commit(command.Meta, before, aggregate, "SAFETY_REVIEW_PASSED", "安全复核通过并冻结当前方案")
	}
	if err := domain.Transition(&aggregate.Case, domain.StatusRectifying, s.now()); err != nil {
		return CommandResponse{}, err
	}
	return s.commit(command.Meta, before, aggregate, "SAFETY_REVIEW_RETURNED", command.Reason)
}

func (s *Service) Issue(caseID string, command IssueCommand) (CommandResponse, error) {
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
	before := aggregate
	permit, err := domain.IssuePermit(s.id(), aggregate, command.Actor, s.now())
	if err != nil {
		return CommandResponse{}, err
	}
	aggregate.Permit = &permit
	if err := domain.Transition(&aggregate.Case, domain.StatusPermitted, s.now()); err != nil {
		return CommandResponse{}, err
	}
	return s.commit(command.Meta, before, aggregate, "PERMIT_ISSUED", "签发不可变送电许可 "+permit.PermitNumber)
}

func (s *Service) VerifyPermit(caseID string) (bool, error) {
	aggregate, err := s.repo.Get(caseID)
	if err != nil {
		return false, err
	}
	return domain.VerifyPermit(aggregate), nil
}
