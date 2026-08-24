package audit

import "sort"

func (l *Logger) TimelineSorted(caseID string) []Event {
	events := l.Timeline(caseID)
	sort.SliceStable(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	return events
}
