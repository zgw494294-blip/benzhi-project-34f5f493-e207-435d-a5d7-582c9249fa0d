package storage

import (
	"errors"
	"sort"
	"strings"
	"time"

	"powerpermit/internal/domain"
)

type Statistics struct {
	Total           int                       `json:"total"`
	ByStatus        map[domain.CaseStatus]int `json:"byStatus"`
	OpenFindings    int                       `json:"openFindings"`
	OverdueFindings int                       `json:"overdueFindings"`
	ValidPermits    int                       `json:"validPermits"`
	TotalLoadKW     float64                   `json:"totalLoadKW"`
}

type SnapshotInfo struct {
	Sequence         int64  `json:"sequence"`
	LastHash         string `json:"lastHash"`
	CaseCount        int    `json:"caseCount"`
	IdempotencyCount int    `json:"idempotencyCount"`
}

func (r *Repository) Statistics(now time.Time) Statistics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := Statistics{ByStatus: map[domain.CaseStatus]int{}}
	for _, aggregate := range r.snapshot.Cases {
		result.Total++
		result.ByStatus[aggregate.Case.Status]++
		if aggregate.Permit != nil && aggregate.Permit.Status == "VALID" && aggregate.Permit.ValidUntil.After(now) {
			result.ValidPermits++
		}
		for _, plan := range aggregate.Plans {
			if plan.ID == aggregate.Case.CurrentPlanID {
				result.TotalLoadKW += plan.TotalLoadKW
			}
		}
		for _, finding := range aggregate.Findings {
			if finding.Result == domain.FindingFail && finding.ResolvedAt == nil {
				result.OpenFindings++
				if finding.DueAt != nil && finding.DueAt.Before(now) {
					result.OverdueFindings++
				}
			}
		}
	}
	return result
}

func (r *Repository) SnapshotInfo() SnapshotInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return SnapshotInfo{Sequence: r.snapshot.Sequence, LastHash: r.snapshot.LastHash, CaseCount: len(r.snapshot.Cases), IdempotencyCount: len(r.snapshot.Idempotency)}
}

func (r *Repository) Search(query string, status domain.CaseStatus) ([]domain.Aggregate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	needle := strings.ToLower(strings.TrimSpace(query))
	items := make([]domain.Aggregate, 0, len(r.snapshot.Cases))
	for _, aggregate := range r.snapshot.Cases {
		if status != "" && aggregate.Case.Status != status {
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{aggregate.Case.ID, aggregate.Case.ActivityName, aggregate.Case.Venue, aggregate.Case.Contact.Name, aggregate.Case.Contact.Phone}, " "))
		if needle != "" && !strings.Contains(haystack, needle) {
			continue
		}
		copyItem, err := cloneAggregate(aggregate)
		if err != nil {
			return nil, err
		}
		items = append(items, copyItem)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Case.UpdatedAt.After(items[j].Case.UpdatedAt) })
	return items, nil
}

func (r *Repository) EventPage(caseID string, after int64, limit int) ([]Event, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	events, err := r.Events(caseID)
	if err != nil {
		return nil, err
	}
	page := make([]Event, 0, limit)
	for _, event := range events {
		if event.Sequence <= after {
			continue
		}
		page = append(page, event)
		if len(page) == limit {
			break
		}
	}
	return page, nil
}

func (r *Repository) FindPermit(number string) (domain.Aggregate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, aggregate := range r.snapshot.Cases {
		if aggregate.Permit != nil && aggregate.Permit.PermitNumber == number {
			return cloneAggregate(aggregate)
		}
	}
	return domain.Aggregate{}, domain.ErrNotFound
}

func ValidateAggregate(aggregate domain.Aggregate) error {
	if aggregate.Case.ID == "" || aggregate.Case.Version < 1 {
		return errors.New("案件基础字段无效")
	}
	plans := map[string]bool{}
	for _, plan := range aggregate.Plans {
		if plan.CaseID != aggregate.Case.ID || plans[plan.ID] {
			return errors.New("方案引用无效或编号重复")
		}
		plans[plan.ID] = true
	}
	if aggregate.Case.CurrentPlanID != "" && !plans[aggregate.Case.CurrentPlanID] {
		return errors.New("当前方案引用不存在")
	}
	findings := map[string]bool{}
	for _, finding := range aggregate.Findings {
		if finding.CaseID != aggregate.Case.ID || !plans[finding.PlanID] || findings[finding.ID] {
			return errors.New("检查记录引用无效或编号重复")
		}
		findings[finding.ID] = true
	}
	if aggregate.Permit != nil {
		if aggregate.Permit.CaseID != aggregate.Case.ID || !plans[aggregate.Permit.PlanID] {
			return errors.New("许可引用无效")
		}
		if aggregate.Case.Status != domain.StatusPermitted {
			return errors.New("存在许可但案件未处于已许可状态")
		}
	}
	return nil
}
