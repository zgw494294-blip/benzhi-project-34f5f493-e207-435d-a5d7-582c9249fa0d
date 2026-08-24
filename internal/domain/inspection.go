package domain

import (
	"strings"
	"time"
)

var Checklist = map[string]string{
	"GROUNDING":   "配电系统接地与等电位连接",
	"RCD_TEST":    "剩余电流保护器动作试验",
	"CABLE_ROUTE": "电缆敷设、防护与跨越措施",
	"PANEL_LOCK":  "配电箱门锁、标识和防雨措施",
	"EMERGENCY":   "应急断电装置和操作通道",
}

func NewFinding(id, caseID, planID string, round int, itemCode, measured, photo string, result FindingResult, text, assignee string, due *time.Time, inspector string, now time.Time) (InspectionFinding, error) {
	if id == "" || caseID == "" || planID == "" {
		return InspectionFinding{}, invalid("id", "检查记录标识不能为空")
	}
	if round < 1 {
		return InspectionFinding{}, invalid("round", "检查轮次必须大于零")
	}
	if _, ok := Checklist[itemCode]; !ok {
		return InspectionFinding{}, invalid("itemCode", "未知检查项")
	}
	if strings.TrimSpace(measured) == "" {
		return InspectionFinding{}, invalid("measuredValue", "请填写实测值或现场观察")
	}
	if result != FindingPass && result != FindingFail {
		return InspectionFinding{}, invalid("result", "结论必须为 PASS 或 FAIL")
	}
	if strings.TrimSpace(inspector) == "" {
		return InspectionFinding{}, invalid("inspector", "检查员不能为空")
	}
	if result == FindingFail {
		if strings.TrimSpace(text) == "" || strings.TrimSpace(assignee) == "" || due == nil {
			return InspectionFinding{}, invalid("finding", "不通过项必须填写问题、责任人和截止时间")
		}
		if !due.After(now) {
			return InspectionFinding{}, invalid("dueAt", "整改截止时间必须晚于检查时间")
		}
	}
	return InspectionFinding{ID: id, CaseID: caseID, PlanID: planID, Round: round, ItemCode: itemCode, MeasuredValue: strings.TrimSpace(measured), PhotoNote: strings.TrimSpace(photo), Result: result, FindingText: strings.TrimSpace(text), Assignee: strings.TrimSpace(assignee), DueAt: due, Inspector: strings.TrimSpace(inspector), InspectedAt: now}, nil
}

func SubmitEvidence(finding *InspectionFinding, actor, note string) error {
	if finding == nil || finding.Result != FindingFail || finding.ResolvedAt != nil {
		return ErrInvalidState
	}
	if strings.TrimSpace(actor) != finding.Assignee {
		return ErrForbidden
	}
	if strings.TrimSpace(note) == "" {
		return invalid("evidenceNote", "整改证据说明不能为空")
	}
	finding.EvidenceNote = strings.TrimSpace(note)
	return nil
}

func ResolveFinding(finding *InspectionFinding, now time.Time) error {
	if finding == nil || finding.Result != FindingFail || finding.EvidenceNote == "" || finding.ResolvedAt != nil {
		return ErrInvalidState
	}
	finding.ResolvedAt = &now
	return nil
}

func NextInspectionRound(findings []InspectionFinding) int {
	max := 0
	for _, finding := range findings {
		if finding.Round > max {
			max = finding.Round
		}
	}
	return max + 1
}
