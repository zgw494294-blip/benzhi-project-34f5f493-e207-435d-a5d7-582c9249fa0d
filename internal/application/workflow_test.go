package application

import (
	"errors"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
	"testing"
	"time"
)

func TestRectificationWorkflowAndConcurrency(t *testing.T) {
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := New(repo, audit.New())
	now := time.Now().UTC()
	created, err := service.CreateCase(CreateCaseCommand{Meta: Meta{Actor: "申请人", IdempotencyKey: "create"}, ActivityName: "展会", Venue: "展馆", StartAt: now.Add(time.Hour), EndAt: now.Add(12 * time.Hour), Contact: domain.Contact{Name: "李工", Phone: "13800000000"}, RiskLevel: domain.RiskMedium})
	if err != nil {
		t.Fatal(err)
	}
	id, version := created.Case.Case.ID, created.Case.Case.Version
	planned, err := service.SubmitPlan(id, SubmitPlanCommand{Meta: Meta{Actor: "工程师", IdempotencyKey: "plan", ExpectedVersion: version}, DesignCapacityKVA: 50, Circuits: []domain.Circuit{{ID: "C1", Name: "主回路", Equipment: "展台", PowerKW: 8, VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartInspection(id, StartInspectionCommand{Meta: Meta{Actor: "检查员", IdempotencyKey: "stale", ExpectedVersion: version}}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected stale conflict, got %v", err)
	}
	started, err := service.StartInspection(id, StartInspectionCommand{Meta: Meta{Actor: "检查员", IdempotencyKey: "start", ExpectedVersion: planned.Case.Case.Version}})
	if err != nil {
		t.Fatal(err)
	}
	due := now.Add(48 * time.Hour)
	items := make([]InspectionItem, 0, len(domain.Checklist))
	for _, code := range domain.ChecklistCodes() {
		item := InspectionItem{ItemCode: code, MeasuredValue: "符合", Result: domain.FindingPass}
		if code == "GROUNDING" {
			item.Result, item.FindingText, item.Assignee, item.DueAt = domain.FindingFail, "接地缺失", "整改人", &due
		}
		items = append(items, item)
	}
	inspected, err := service.RecordInspection(id, RecordInspectionCommand{Meta: Meta{Actor: "检查员", IdempotencyKey: "inspect", ExpectedVersion: started.Case.Case.Version}, Items: items})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Case.Case.Status != domain.StatusRectifying {
		t.Fatal("case should require rectification")
	}
	failed := domain.OpenFindings(inspected.Case)[0]
	evidenced, err := service.SubmitEvidence(id, failed.ID, EvidenceCommand{Meta: Meta{Actor: "整改人", IdempotencyKey: "evidence", ExpectedVersion: inspected.Case.Case.Version}, Note: "已安装接地线并附照片"})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := service.StartInspection(id, StartInspectionCommand{Meta: Meta{Actor: "检查员", IdempotencyKey: "restart", ExpectedVersion: evidenced.Case.Case.Version}})
	if err != nil {
		t.Fatal(err)
	}
	rechecked, err := service.RecordInspection(id, RecordInspectionCommand{Meta: Meta{Actor: "检查员", IdempotencyKey: "recheck", ExpectedVersion: restarted.Case.Case.Version}, Items: []InspectionItem{{ItemCode: "GROUNDING", MeasuredValue: "接地电阻 2.0Ω", Result: domain.FindingPass, ResolvesFindingID: failed.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := service.Review(id, ReviewCommand{Meta: Meta{Actor: "负责人", IdempotencyKey: "review", ExpectedVersion: rechecked.Case.Case.Version}, Passed: true})
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue(id, IssueCommand{Meta: Meta{Actor: "负责人", IdempotencyKey: "issue", ExpectedVersion: reviewed.Case.Case.Version}})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Case.Permit == nil || !domain.VerifyPermit(issued.Case) {
		t.Fatal("permit missing or invalid")
	}
	duplicate, err := service.Issue(id, IssueCommand{Meta: Meta{Actor: "负责人", IdempotencyKey: "issue", ExpectedVersion: reviewed.Case.Case.Version}})
	if err != nil || !duplicate.Duplicate {
		t.Fatalf("idempotency failed: %+v %v", duplicate, err)
	}
	history, err := service.History(id)
	if err != nil || !history.IntegrityOK || history.TimelineDigest.EventCount != 9 {
		t.Fatalf("bad history: %+v %v", history, err)
	}
}
