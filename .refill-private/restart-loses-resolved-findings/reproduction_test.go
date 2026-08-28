package restart_loses_resolved_findings_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"powerpermit/internal/application"
	"powerpermit/internal/audit"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
)

func TestRestartRecoveryRetainsResolvedFindings(t *testing.T) {
	dir := t.TempDir()
	repo, err := storage.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.New(repo, audit.New())
	now := time.Now().UTC()

	created, err := service.CreateCase(application.CreateCaseCommand{
		Meta:         application.Meta{Actor: "申请人", IdempotencyKey: "recovery-create"},
		ActivityName: "重启恢复测试活动",
		Venue:        "临时展馆",
		StartAt:      now.Add(time.Hour),
		EndAt:        now.Add(24 * time.Hour),
		Contact:      domain.Contact{Name: "李工", Phone: "13800000000"},
		RiskLevel:    domain.RiskMedium,
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := created.Case.Case.ID

	planned, err := service.SubmitPlan(caseID, application.SubmitPlanCommand{
		Meta:              application.Meta{Actor: "方案工程师", IdempotencyKey: "recovery-plan", ExpectedVersion: created.Case.Case.Version},
		DesignCapacityKVA: 50,
		Circuits: []domain.Circuit{{
			ID: "C1", Name: "舞台主回路", Equipment: "舞台设备", PowerKW: 8,
			VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	started, err := service.StartInspection(caseID, application.StartInspectionCommand{Meta: application.Meta{
		Actor: "检查员", IdempotencyKey: "recovery-start", ExpectedVersion: planned.Case.Case.Version,
	}})
	if err != nil {
		t.Fatal(err)
	}

	dueAt := now.Add(12 * time.Hour)
	items := make([]application.InspectionItem, 0, len(domain.Checklist))
	for _, code := range domain.ChecklistCodes() {
		item := application.InspectionItem{ItemCode: code, MeasuredValue: "现场检查通过", Result: domain.FindingPass}
		if code == "GROUNDING" {
			item.Result = domain.FindingFail
			item.FindingText = "接地连接缺失"
			item.Assignee = "整改负责人"
			item.DueAt = &dueAt
		}
		items = append(items, item)
	}
	inspected, err := service.RecordInspection(caseID, application.RecordInspectionCommand{
		Meta:  application.Meta{Actor: "检查员", IdempotencyKey: "recovery-inspect", ExpectedVersion: started.Case.Case.Version},
		Items: items,
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := domain.OpenFindings(inspected.Case)[0]
	evidenced, err := service.SubmitEvidence(caseID, failed.ID, application.EvidenceCommand{
		Meta: application.Meta{Actor: "整改负责人", IdempotencyKey: "recovery-evidence", ExpectedVersion: inspected.Case.Case.Version},
		Note: "已补装接地连接并留存照片",
	})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := service.StartInspection(caseID, application.StartInspectionCommand{Meta: application.Meta{
		Actor: "检查员", IdempotencyKey: "recovery-restart", ExpectedVersion: evidenced.Case.Case.Version,
	}})
	if err != nil {
		t.Fatal(err)
	}
	rechecked, err := service.RecordInspection(caseID, application.RecordInspectionCommand{
		Meta: application.Meta{Actor: "检查员", IdempotencyKey: "recovery-recheck", ExpectedVersion: restarted.Case.Case.Version},
		Items: []application.InspectionItem{{
			ItemCode: "GROUNDING", MeasuredValue: "接地电阻 2.0Ω", Result: domain.FindingPass, ResolvesFindingID: failed.ID,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rechecked.Case.Findings) != len(domain.Checklist)+1 {
		t.Fatalf("测试前置状态不完整: findings=%d", len(rechecked.Case.Findings))
	}

	snapshotPath := filepath.Join(dir, "snapshot.json")
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	recoveredRepo, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("重启恢复失败: %v", err)
	}
	recovered, err := recoveredRepo.Get(caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.Findings) != len(rechecked.Case.Findings) {
		t.Fatalf("TestRestartRecoveryRetainsResolvedFindings: 重启前有 %d 条不可变检查记录，事件重放后仅剩 %d 条", len(rechecked.Case.Findings), len(recovered.Findings))
	}
	for _, finding := range recovered.Findings {
		if finding.ID == failed.ID && finding.ResolvedAt != nil && finding.EvidenceNote != "" {
			return
		}
	}
	t.Fatalf("TestRestartRecoveryRetainsResolvedFindings: 已关闭的原检查记录 %s 在事件重放后丢失", failed.ID)
}
