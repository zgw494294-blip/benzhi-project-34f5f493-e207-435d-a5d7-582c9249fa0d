package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"powerpermit/internal/domain"
)

func PlanHash(plan domain.ElectricalPlan) (string, error) {
	plan.ContentHash = ""
	plan.FrozenAt = nil
	payload, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func PermitHash(permit domain.EnergizationPermit, planHash string) (string, error) {
	permit.ContentHash = ""
	payload, err := json.Marshal(struct {
		Permit   domain.EnergizationPermit `json:"permit"`
		PlanHash string                    `json:"planHash"`
	}{permit, planHash})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
