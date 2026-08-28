package readiness_cache_stale_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
	"powerpermit/internal/web"
)

func TestReadinessCacheTracksCaseVersion(t *testing.T) {
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, audit.New())
	now := time.Now().UTC()
	created, err := service.CreateCase(application.CreateCaseCommand{
		Meta:         application.Meta{Actor: "申请人", IdempotencyKey: "cache-create"},
		ActivityName: "缓存失效复现活动",
		Venue:        "临时展馆",
		StartAt:      now.Add(time.Hour),
		EndAt:        now.Add(12 * time.Hour),
		Contact:      domain.Contact{Name: "李工", Phone: "13800000000"},
		RiskLevel:    domain.RiskMedium,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := created.Case.Case.ID
	planned, err := service.SubmitPlan(caseID, application.SubmitPlanCommand{
		Meta:              application.Meta{Actor: "工程师", IdempotencyKey: "cache-plan", ExpectedVersion: created.Case.Case.Version},
		DesignCapacityKVA: 50,
		Circuits: []domain.Circuit{{
			ID: "C1", Name: "主回路", Equipment: "展台", PowerKW: 8,
			VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := web.New(service).Handler()
	before := requestReadiness(t, handler, caseID)
	if !before.CanInspect || before.CanReview {
		t.Fatalf("unexpected planned readiness: %+v", before)
	}

	started, err := service.StartInspection(caseID, application.StartInspectionCommand{Meta: application.Meta{
		Actor: "检查员", IdempotencyKey: "cache-start", ExpectedVersion: planned.Case.Case.Version,
	}})
	if err != nil {
		t.Fatal(err)
	}
	items := make([]application.InspectionItem, 0, len(domain.Checklist))
	for _, code := range domain.ChecklistCodes() {
		items = append(items, application.InspectionItem{
			ItemCode: code, MeasuredValue: "符合", Result: domain.FindingPass,
		})
	}
	inspected, err := service.RecordInspection(caseID, application.RecordInspectionCommand{
		Meta:  application.Meta{Actor: "检查员", IdempotencyKey: "cache-inspect", ExpectedVersion: started.Case.Case.Version},
		Items: items,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Case.Case.Status != domain.StatusReviewing {
		t.Fatalf("inspection did not reach reviewing: %s", inspected.Case.Case.Status)
	}

	after := requestReadiness(t, handler, caseID)
	if after.CanInspect || !after.CanReview {
		t.Fatalf("TestReadinessCacheTracksCaseVersion: stale readiness after version %d: %+v", inspected.Case.Case.Version, after)
	}
}

func requestReadiness(t *testing.T, handler http.Handler, caseID string) application.Readiness {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/cases/"+caseID+"/readiness", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var result application.Readiness
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
