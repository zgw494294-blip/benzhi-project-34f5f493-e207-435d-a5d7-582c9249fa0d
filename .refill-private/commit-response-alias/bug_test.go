package commit_response_alias

import (
	"testing"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

func TestCommandResponseMutationDoesNotPolluteRepository(t *testing.T) {
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, audit.New())
	now := time.Now().UTC()
	created, err := service.CreateCase(application.CreateCaseCommand{
		Meta: application.Meta{Actor: "申请人", IdempotencyKey: "create"}, ActivityName: "活动", Venue: "场地",
		StartAt: now.Add(time.Hour), EndAt: now.Add(2 * time.Hour), Contact: domain.Contact{Name: "联系人", Phone: "10086"}, RiskLevel: domain.RiskLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	planned, err := service.SubmitPlan(created.Case.Case.ID, application.SubmitPlanCommand{
		Meta: application.Meta{Actor: "工程师", IdempotencyKey: "plan", ExpectedVersion: created.Case.Case.Version},
		DesignCapacityKVA: 50, Circuits: []domain.Circuit{{ID: "c1", Name: "主回路", Equipment: "原设备", PowerKW: 8, VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4}},
	})
	if err != nil {
		t.Fatal(err)
	}
	planned.Case.Plans[0].Circuits[0].Equipment = "调用方篡改"
	stored, err := service.Get(created.Case.Case.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Plans[0].Circuits[0].Equipment != "原设备" {
		t.Fatalf("TestCommandResponseMutationDoesNotPolluteRepository: response alias changed stored projection to %q without a commit", stored.Plans[0].Circuits[0].Equipment)
	}
}
