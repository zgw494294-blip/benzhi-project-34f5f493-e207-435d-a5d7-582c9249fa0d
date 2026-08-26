package concurrentauditaccess

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"powerpermit/internal/audit"
)

func TestConcurrentAuditRecordingAndTimeline(t *testing.T) {
	previousProcs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previousProcs)

	logger := audit.New()
	logger.Record(audit.Event{
		ID:         "initial",
		CaseID:     "case-controlled",
		Action:     "CASE_CREATED",
		Actor:      "申请人",
		RequestKey: "initial-key",
		ToVersion:  1,
		At:         time.Unix(1, 0).UTC(),
	})

	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(2)
	done.Add(2)

	go func() {
		defer done.Done()
		ready.Done()
		<-start
		for index := 0; index < 64; index++ {
			logger.Record(audit.Event{
				ID:          "recorded",
				CaseID:      "case-controlled",
				Action:      "EVIDENCE_SUBMITTED",
				Actor:       "责任人",
				RequestKey:  "record-key",
				FromVersion: int64(index + 1),
				ToVersion:   int64(index + 2),
				At:          time.Unix(int64(index+2), 0).UTC(),
			})
		}
	}()

	go func() {
		defer done.Done()
		ready.Done()
		<-start
		for index := 0; index < 64; index++ {
			_ = logger.TimelineSorted("case-controlled")
			_ = logger.All()
		}
	}()

	ready.Wait()
	close(start)
	done.Wait()
}
