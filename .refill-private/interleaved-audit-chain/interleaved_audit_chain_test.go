package interleavedauditchain

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
	"powerpermit/internal/web"
)

func TestAuditQueryAcceptsInterleavedCaseEvents(t *testing.T) {
	dir := t.TempDir()
	repo, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first, err := domain.NewCase("case-a", "活动 A", "场地 A", now.Add(time.Hour), now.Add(2*time.Hour), domain.Contact{Name: "甲", Phone: "10001"}, domain.RiskLow, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := domain.NewCase("case-b", "活动 B", "场地 B", now.Add(time.Hour), now.Add(2*time.Hour), domain.Contact{Name: "乙", Phone: "10002"}, domain.RiskLow, now)
	if err != nil {
		t.Fatal(err)
	}
	empty := func(item domain.PowerPermitCase) domain.Aggregate {
		return domain.Aggregate{Case: item, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
	}
	firstAggregate := empty(first)
	if _, _, err := repo.Commit(storage.Mutation{CaseID: first.ID, ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "甲", IdempotencyKey: "a-1", Aggregate: firstAggregate}); err != nil {
		t.Fatal(err)
	}
	secondAggregate := empty(second)
	if _, _, err := repo.Commit(storage.Mutation{CaseID: second.ID, ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "乙", IdempotencyKey: "b-1", Aggregate: secondAggregate}); err != nil {
		t.Fatal(err)
	}
	plan, err := domain.BuildPlan("plan-a", first.ID, 1, []domain.Circuit{{ID: "c1", Name: "主回路", Equipment: "展台", PowerKW: 8, VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4}}, 50, now)
	if err != nil {
		t.Fatal(err)
	}
	firstAggregate.Plans = append(firstAggregate.Plans, plan)
	firstAggregate.Case.CurrentPlanID = plan.ID
	if err := domain.Transition(&firstAggregate.Case, domain.StatusPlanned, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.Commit(storage.Mutation{CaseID: first.ID, ExpectedVersion: 1, Kind: "PLAN_SUBMITTED", Actor: "甲", IdempotencyKey: "a-2", Aggregate: firstAggregate}); err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, audit.New())
	handler := web.New(service).Handler()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/cases/"+first.ID+"/audit", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("交错事件不应阻断单案件审计查询: HTTP %d, body=%s", recorder.Code, recorder.Body.String())
	}
	data, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("PLAN_SUBMITTED"), []byte("PLAN_CORRUPTED"), 1)
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), data, 0o640); err != nil {
		t.Fatal(err)
	}
	corruptedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(corruptedRecorder, httptest.NewRequest(http.MethodGet, "/api/cases/"+first.ID+"/audit", nil))
	if corruptedRecorder.Code == http.StatusOK {
		t.Fatalf("篡改后的事件链不应继续返回成功: body=%s", corruptedRecorder.Body.String())
	}
}
