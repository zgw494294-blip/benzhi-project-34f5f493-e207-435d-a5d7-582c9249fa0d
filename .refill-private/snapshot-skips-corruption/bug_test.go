package snapshot_skips_corruption

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

func TestOpenRejectsCorruptionBeforeSnapshotSequence(t *testing.T) {
	dir := t.TempDir()
	repo, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	caseItem, err := domain.NewCase("case-1", "活动", "场地", now.Add(time.Hour), now.Add(2*time.Hour), domain.Contact{Name: "联系人", Phone: "10086"}, domain.RiskLow, now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate := domain.Aggregate{Case: caseItem, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
	if _, _, err := repo.Commit(storage.Mutation{CaseID: "case-1", ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "申请人", IdempotencyKey: "create", Aggregate: aggregate}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var event storage.Event
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatal(err)
	}
	event.Actor = "被篡改的操作者"
	corrupted, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	corrupted = append(corrupted, '\n')
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), corrupted, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := repo.Verify(); err == nil {
		t.Fatal("完整事件链校验意外通过")
	}
	if _, err := storage.Open(dir); err == nil {
		t.Fatal("TestOpenRejectsCorruptionBeforeSnapshotSequence: startup accepted corrupted history covered by snapshot")
	}
}
