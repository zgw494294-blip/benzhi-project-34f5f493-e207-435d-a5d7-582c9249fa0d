package commit_failure_publishes_state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

func TestFailedEventAppendDoesNotPublishState(t *testing.T) {
	dir := t.TempDir()
	repo, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "events.jsonl"), 0o750); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	caseItem, err := domain.NewCase("case-1", "活动", "场地", now.Add(time.Hour), now.Add(2*time.Hour), domain.Contact{Name: "联系人", Phone: "10086"}, domain.RiskLow, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.Aggregate{Case: caseItem, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
	_, _, commitErr := repo.Commit(storage.Mutation{CaseID: caseItem.ID, ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "申请人", IdempotencyKey: "create-1", Aggregate: aggregate})
	if commitErr == nil {
		t.Fatal("expected event append failure")
	}

	if _, err := repo.Get(caseItem.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("failed commit published case state: %v", err)
	}
	if _, ok := repo.IdempotentResult("create-1"); ok {
		t.Fatal("failed commit published idempotency result")
	}
	if info := repo.SnapshotInfo(); info.Sequence != 0 || info.CaseCount != 0 || info.IdempotencyCount != 0 {
		t.Fatalf("failed commit advanced snapshot: %+v", info)
	}
}
