package domain

import (
	"errors"
	"testing"
	"time"
)

func TestBuildPlanAndFreezePermit(t *testing.T) {
	now := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	plan, err := BuildPlan("plan-1", "case-1", 1, []Circuit{{ID: "C1", Name: "舞台", Equipment: "灯光", PowerKW: 8, VoltageV: 380, Phases: 3, BreakerA: 20, RCDMilliA: 30, CableMM2: 4}}, 50, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CalculationResult != "PASS" || plan.TotalLoadKW != 8 {
		t.Fatalf("unexpected calculation: %+v", plan)
	}
	if err := FreezePlan(&plan, now); err != nil {
		t.Fatal(err)
	}
	aggregate := Aggregate{Case: PowerPermitCase{ID: "case-1", Status: StatusFrozen, CurrentPlanID: plan.ID, EndAt: now.Add(8 * time.Hour)}, Plans: []ElectricalPlan{plan}}
	permit, err := IssuePermit("permit-1", aggregate, "负责人", now)
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Permit = &permit
	if !VerifyPermit(aggregate) {
		t.Fatal("permit hash should verify")
	}
	aggregate.Permit.ApprovedBy = "篡改者"
	if VerifyPermit(aggregate) {
		t.Fatal("tampered permit must fail verification")
	}
}

func TestProtectionFailureAndTransitions(t *testing.T) {
	now := time.Now().UTC()
	plan, err := BuildPlan("p", "c", 1, []Circuit{{ID: "C", Name: "回路", Equipment: "设备", PowerKW: 10, VoltageV: 220, Phases: 1, BreakerA: 10, RCDMilliA: 100, CableMM2: 1.5}}, 5, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CalculationResult != "FAIL" {
		t.Fatal("unsafe circuit should fail")
	}
	item := PowerPermitCase{Status: StatusDraft, Version: 1}
	if err := Transition(&item, StatusPlanned, now); err != nil {
		t.Fatal(err)
	}
	if err := Transition(&item, StatusFrozen, now); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expected invalid state, got %v", err)
	}
}

func TestInspectionRules(t *testing.T) {
	now := time.Now().UTC()
	items := make([]InspectionFinding, 0, len(Checklist))
	for _, code := range ChecklistCodes() {
		finding, err := NewFinding(code, "case", "plan", 1, code, "符合", "", FindingPass, "", "", nil, "检查员", now)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, finding)
	}
	if err := ValidateFirstInspection(items); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFirstInspection(items[:len(items)-1]); err == nil {
		t.Fatal("incomplete first inspection should fail")
	}
	due := now.Add(time.Hour)
	finding, err := NewFinding("f", "case", "plan", 1, "GROUNDING", "未连接", "", FindingFail, "补做接地", "责任人", &due, "检查员", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := SubmitEvidence(&finding, "其他人", "照片"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if err := SubmitEvidence(&finding, "责任人", "接地完成照片"); err != nil {
		t.Fatal(err)
	}
	if err := ResolveFinding(&finding, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}
