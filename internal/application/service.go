package application

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

type Service struct {
	repo  *storage.Repository
	audit *audit.Logger
	now   func() time.Time
	id    func() string
}

func New(repo *storage.Repository, logger *audit.Logger) *Service {
	return &Service{repo: repo, audit: logger, now: func() time.Time { return time.Now().UTC() }, id: newID}
}

func newID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buffer)
}

func validateMeta(meta Meta, create bool) error {
	if strings.TrimSpace(meta.Actor) == "" {
		return &domain.ValidationError{Field: "actor", Message: "操作者不能为空"}
	}
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return &domain.ValidationError{Field: "idempotencyKey", Message: "幂等请求键不能为空"}
	}
	if create && meta.ExpectedVersion != 0 {
		return domain.ErrConflict
	}
	if !create && meta.ExpectedVersion < 1 {
		return &domain.ValidationError{Field: "expectedVersion", Message: "必须大于零"}
	}
	return nil
}

func (s *Service) Get(caseID string) (domain.Aggregate, error) { return s.repo.Get(caseID) }

func (s *Service) List(status domain.CaseStatus) ([]domain.Aggregate, error) {
	return s.repo.List(status)
}

func (s *Service) Timeline(caseID string) ([]audit.Event, error) {
	events, _, err := s.PersistentTimeline(caseID)
	return events, err
}

func (s *Service) verifyKey(key, caseID string) (CommandResponse, bool, error) {
	result, ok := s.repo.IdempotentResult(key)
	if !ok {
		return CommandResponse{}, false, nil
	}
	if caseID != "" && result.CaseID != caseID {
		return CommandResponse{}, true, domain.ErrConflict
	}
	aggregate, err := snapshotFromResult(result)
	if err != nil {
		return CommandResponse{}, true, err
	}
	return CommandResponse{Case: aggregate, Duplicate: true}, true, nil
}

// snapshotFromResult reconstructs the aggregate captured at the first successful
// commit for an idempotency key. Returning this stored snapshot—instead of the
// live case state—ensures duplicate retries observe the original case version,
// status and plan content even after later transitions (for example into field
// inspection) have advanced the case.
func snapshotFromResult(result storage.CommandResult) (domain.Aggregate, error) {
	if len(result.Response) == 0 {
		return domain.Aggregate{}, domain.ErrNotFound
	}
	var aggregate domain.Aggregate
	if err := json.Unmarshal(result.Response, &aggregate); err != nil {
		return domain.Aggregate{}, err
	}
	return aggregate, nil
}

func (s *Service) commit(meta Meta, before domain.Aggregate, after domain.Aggregate, kind, detail string) (CommandResponse, error) {
	if response, duplicate, err := s.verifyKey(meta.IdempotencyKey, after.Case.ID); duplicate {
		return response, err
	}
	committed, duplicate, err := s.repo.Commit(storage.Mutation{CaseID: after.Case.ID, ExpectedVersion: meta.ExpectedVersion, Kind: kind, Actor: meta.Actor, IdempotencyKey: meta.IdempotencyKey, Aggregate: after, Response: after})
	if err != nil {
		return CommandResponse{}, err
	}
	if duplicate {
		aggregate, snapErr := snapshotFromResult(committed)
		return CommandResponse{Case: aggregate, Duplicate: true}, snapErr
	}
	s.audit.Record(audit.Event{ID: s.id(), CaseID: after.Case.ID, Action: kind, Actor: meta.Actor, RequestKey: meta.IdempotencyKey, FromVersion: before.Case.Version, ToVersion: after.Case.Version, FromStatus: string(before.Case.Status), ToStatus: string(after.Case.Status), At: s.now(), Detail: detail})
	return CommandResponse{Case: after}, nil
}

func touch(c *domain.PowerPermitCase, now time.Time) { c.Version++; c.UpdatedAt = now }

func is(err, target error) bool { return errors.Is(err, target) }
