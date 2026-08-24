package storage

import (
	"errors"
	"powerpermit/internal/domain"
	"testing"
	"time"
)

func TestRepositoryCommitRestartAndIdempotency(t *testing.T) {
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	caseItem, err := domain.NewCase("case-1", "活动", "场地", now.Add(time.Hour), now.Add(2*time.Hour), domain.Contact{Name: "联系人", Phone: "10086"}, domain.RiskLow, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.Aggregate{Case: caseItem, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
	first, duplicate, err := repo.Commit(Mutation{CaseID: "case-1", ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "申请人", IdempotencyKey: "key-1", Aggregate: aggregate})
	if err != nil || duplicate || first.Version != 1 {
		t.Fatalf("commit result: %+v %v %v", first, duplicate, err)
	}
	_, duplicate, err = repo.Commit(Mutation{CaseID: "case-1", ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "申请人", IdempotencyKey: "key-1", Aggregate: aggregate})
	if err != nil || !duplicate {
		t.Fatalf("duplicate not recognized: %v %v", duplicate, err)
	}
	_, _, err = repo.Commit(Mutation{CaseID: "case-1", ExpectedVersion: 0, Kind: "BAD", Actor: "申请人", IdempotencyKey: "key-2", Aggregate: aggregate})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := repo.Verify(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get("case-1")
	if err != nil || restored.Case.ActivityName != "活动" {
		t.Fatalf("restore failed: %+v %v", restored, err)
	}
	if reopened.SnapshotInfo().Sequence != 1 {
		t.Fatal("unexpected snapshot sequence")
	}
}
