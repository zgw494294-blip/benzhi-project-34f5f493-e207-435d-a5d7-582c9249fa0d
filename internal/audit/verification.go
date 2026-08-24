package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"powerpermit/internal/domain"
)

type TimelineDigest struct {
	CaseID     string    `json:"caseID"`
	EventCount int       `json:"eventCount"`
	FirstAt    time.Time `json:"firstAt"`
	LastAt     time.Time `json:"lastAt"`
	Hash       string    `json:"hash"`
}

func DigestTimeline(caseID string, events []Event) (TimelineDigest, error) {
	if strings.TrimSpace(caseID) == "" {
		return TimelineDigest{}, errors.New("案件编号不能为空")
	}
	ordered := append([]Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].At.Equal(ordered[j].At) {
			return ordered[i].ToVersion < ordered[j].ToVersion
		}
		return ordered[i].At.Before(ordered[j].At)
	})
	previousVersion := int64(0)
	for _, event := range ordered {
		if event.CaseID != caseID {
			return TimelineDigest{}, errors.New("审计事件属于其他案件")
		}
		if event.FromVersion != previousVersion || event.ToVersion != previousVersion+1 {
			return TimelineDigest{}, errors.New("审计版本链不连续")
		}
		if event.Actor == "" || event.RequestKey == "" || event.Action == "" {
			return TimelineDigest{}, errors.New("审计事件缺少操作者、请求键或动作")
		}
		previousVersion = event.ToVersion
	}
	payload, err := json.Marshal(ordered)
	if err != nil {
		return TimelineDigest{}, err
	}
	sum := sha256.Sum256(payload)
	result := TimelineDigest{CaseID: caseID, EventCount: len(ordered), Hash: hex.EncodeToString(sum[:])}
	if len(ordered) > 0 {
		result.FirstAt, result.LastAt = ordered[0].At, ordered[len(ordered)-1].At
	}
	return result, nil
}

func AggregateDigest(aggregate domain.Aggregate) (string, error) {
	copyAggregate := aggregate
	if copyAggregate.Permit != nil {
		copyPermit := *copyAggregate.Permit
		copyPermit.ContentHash = ""
		copyAggregate.Permit = &copyPermit
	}
	for i := range copyAggregate.Plans {
		copyAggregate.Plans[i].ContentHash = ""
	}
	payload, err := json.Marshal(copyAggregate)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func VerifyFrozenPlan(plan domain.ElectricalPlan) bool {
	if plan.FrozenAt == nil || plan.ContentHash == "" {
		return false
	}
	expected := plan.ContentHash
	actual, err := PlanHash(plan)
	return err == nil && expected == actual
}

func VerifyPermit(permit domain.EnergizationPermit, plan domain.ElectricalPlan) bool {
	if !VerifyFrozenPlan(plan) || permit.PlanID != plan.ID {
		return false
	}
	expected := permit.ContentHash
	actual, err := PermitHash(permit, plan.ContentHash)
	return err == nil && expected == actual
}
