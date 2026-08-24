package application

import (
	"powerpermit/internal/domain"
	"time"
)

type Meta struct {
	Actor           string `json:"actor"`
	IdempotencyKey  string `json:"idempotencyKey"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

type CreateCaseCommand struct {
	Meta
	ActivityName string           `json:"activityName"`
	Venue        string           `json:"venue"`
	StartAt      time.Time        `json:"startAt"`
	EndAt        time.Time        `json:"endAt"`
	Contact      domain.Contact   `json:"contact"`
	RiskLevel    domain.RiskLevel `json:"riskLevel"`
}

type SubmitPlanCommand struct {
	Meta
	Circuits          []domain.Circuit `json:"circuits"`
	DesignCapacityKVA float64          `json:"designCapacityKVA"`
}

type StartInspectionCommand struct{ Meta }

type RecordInspectionCommand struct {
	Meta
	Items []InspectionItem `json:"items"`
}

type InspectionItem struct {
	ItemCode          string               `json:"itemCode"`
	MeasuredValue     string               `json:"measuredValue"`
	PhotoNote         string               `json:"photoNote"`
	Result            domain.FindingResult `json:"result"`
	FindingText       string               `json:"findingText"`
	Assignee          string               `json:"assignee"`
	DueAt             *time.Time           `json:"dueAt"`
	ResolvesFindingID string               `json:"resolvesFindingID,omitempty"`
}

type EvidenceCommand struct {
	Meta
	Note string `json:"note"`
}

type ReviewCommand struct {
	Meta
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

type IssueCommand struct{ Meta }

type CommandResponse struct {
	Case      domain.Aggregate `json:"case"`
	Duplicate bool             `json:"duplicate"`
}
