package concurrentquerybuffer

import (
	"fmt"
	"powerpermit/internal/domain"
	"powerpermit/internal/storage"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentQueriesDoNotShareBuffers(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(previousProcs)
	repo, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		name := fmt.Sprintf("alpha-%02d", i)
		if i%2 == 0 {
			name = fmt.Sprintf("beta-%02d", i)
		}
		item, createErr := domain.NewCase(fmt.Sprintf("case-%02d", i), name, "场地", when.Add(time.Hour), when.Add(2*time.Hour), domain.Contact{Name: "联系人", Phone: "10086"}, domain.RiskLow, when)
		if createErr != nil {
			t.Fatal(createErr)
		}
		aggregate := domain.Aggregate{Case: item, Plans: []domain.ElectricalPlan{}, Findings: []domain.InspectionFinding{}, Reviews: []domain.Review{}}
		if _, _, commitErr := repo.Commit(storage.Mutation{CaseID: item.ID, ExpectedVersion: 0, Kind: "CASE_CREATED", Actor: "测试", IdempotencyKey: item.ID, Aggregate: aggregate}); commitErr != nil {
			t.Fatal(commitErr)
		}
	}

	const workers = 12
	const rounds = 120
	start := make(chan struct{})
	errors := make(chan error, workers*rounds)
	var group sync.WaitGroup
	group.Add(workers * 2)
	for i := 0; i < workers; i++ {
		go func() {
			defer group.Done()
			<-start
			for round := 0; round < rounds; round++ {
				items, listErr := repo.List("")
				if listErr != nil {
					errors <- listErr
					return
				}
				if len(items) != 24 {
					errors <- fmt.Errorf("列表结果被并发查询污染：得到 %d 条", len(items))
					return
				}
			}
		}()
		go func() {
			defer group.Done()
			<-start
			for round := 0; round < rounds; round++ {
				items, searchErr := repo.Search("alpha", "")
				if searchErr != nil {
					errors <- searchErr
					return
				}
				for _, item := range items {
					if !strings.Contains(item.Case.ActivityName, "alpha") {
						errors <- fmt.Errorf("搜索结果混入非 alpha 案件：%s", item.Case.ActivityName)
						return
					}
				}
			}
		}()
	}
	close(start)
	group.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
}
