package domain

import "time"

type CaseStatus string

const (
	StatusDraft      CaseStatus = "DRAFT"
	StatusPlanned    CaseStatus = "PLANNED"
	StatusInspecting CaseStatus = "INSPECTING"
	StatusRectifying CaseStatus = "RECTIFYING"
	StatusReviewing  CaseStatus = "REVIEWING"
	StatusFrozen     CaseStatus = "FROZEN"
	StatusPermitted  CaseStatus = "PERMITTED"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "LOW"
	RiskMedium RiskLevel = "MEDIUM"
	RiskHigh   RiskLevel = "HIGH"
)

type Contact struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type PowerPermitCase struct {
	ID            string     `json:"id"`
	ActivityName  string     `json:"activityName"`
	Venue         string     `json:"venue"`
	StartAt       time.Time  `json:"startAt"`
	EndAt         time.Time  `json:"endAt"`
	Contact       Contact    `json:"contact"`
	RiskLevel     RiskLevel  `json:"riskLevel"`
	Status        CaseStatus `json:"status"`
	CurrentPlanID string     `json:"currentPlanID,omitempty"`
	Version       int64      `json:"version"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type Circuit struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Equipment string  `json:"equipment"`
	PowerKW   float64 `json:"powerKW"`
	VoltageV  float64 `json:"voltageV"`
	Phases    int     `json:"phases"`
	BreakerA  float64 `json:"breakerA"`
	RCDMilliA int     `json:"rcdMilliA"`
	CableMM2  float64 `json:"cableMM2"`
}

type ProtectionCheck struct {
	CircuitID          string   `json:"circuitID"`
	CalculatedCurrentA float64  `json:"calculatedCurrentA"`
	BreakerAdequate    bool     `json:"breakerAdequate"`
	CableAdequate      bool     `json:"cableAdequate"`
	RCDCompliant       bool     `json:"rcdCompliant"`
	Messages           []string `json:"messages"`
}

type ElectricalPlan struct {
	ID                string            `json:"id"`
	CaseID            string            `json:"caseID"`
	Revision          int               `json:"revision"`
	Circuits          []Circuit         `json:"circuits"`
	TotalLoadKW       float64           `json:"totalLoadKW"`
	DesignCapacityKVA float64           `json:"designCapacityKVA"`
	ProtectionChecks  []ProtectionCheck `json:"protectionChecks"`
	CalculationResult string            `json:"calculationResult"`
	SubmittedAt       time.Time         `json:"submittedAt"`
	FrozenAt          *time.Time        `json:"frozenAt,omitempty"`
	ContentHash       string            `json:"contentHash,omitempty"`
}

type FindingResult string

const (
	FindingPass FindingResult = "PASS"
	FindingFail FindingResult = "FAIL"
)

type InspectionFinding struct {
	ID            string        `json:"id"`
	CaseID        string        `json:"caseID"`
	PlanID        string        `json:"planID"`
	Round         int           `json:"round"`
	ItemCode      string        `json:"itemCode"`
	MeasuredValue string        `json:"measuredValue"`
	PhotoNote     string        `json:"photoNote,omitempty"`
	Result        FindingResult `json:"result"`
	FindingText   string        `json:"findingText,omitempty"`
	Assignee      string        `json:"assignee,omitempty"`
	DueAt         *time.Time    `json:"dueAt,omitempty"`
	EvidenceNote  string        `json:"evidenceNote,omitempty"`
	ResolvedAt    *time.Time    `json:"resolvedAt,omitempty"`
	Inspector     string        `json:"inspector"`
	InspectedAt   time.Time     `json:"inspectedAt"`
}

type Review struct {
	Reviewer   string    `json:"reviewer"`
	Passed     bool      `json:"passed"`
	Reason     string    `json:"reason,omitempty"`
	ReviewedAt time.Time `json:"reviewedAt"`
}

type EnergizationPermit struct {
	ID           string    `json:"id"`
	CaseID       string    `json:"caseID"`
	PlanID       string    `json:"planID"`
	PermitNumber string    `json:"permitNumber"`
	ApprovedBy   string    `json:"approvedBy"`
	ApprovedAt   time.Time `json:"approvedAt"`
	ValidUntil   time.Time `json:"validUntil"`
	ContentHash  string    `json:"contentHash"`
	Status       string    `json:"status"`
}

type Aggregate struct {
	Case     PowerPermitCase     `json:"case"`
	Plans    []ElectricalPlan    `json:"plans"`
	Findings []InspectionFinding `json:"findings"`
	Reviews  []Review            `json:"reviews"`
	Permit   *EnergizationPermit `json:"permit,omitempty"`
}
