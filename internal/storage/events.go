package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func (r *Repository) Events(caseID string) ([]Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	file, err := os.Open(r.eventPath)
	if os.IsNotExist(err) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(file))
	result := []Event{}
	for {
		var event Event
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if caseID == "" || event.CaseID == caseID {
			result = append(result, event)
		}
	}
	if !ValidateEventChain(result, 0, "") {
		return nil, errors.New("事件日志校验链不连续")
	}
	return result, nil
}

func (r *Repository) Verify() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	events, err := r.readEventsAfter(0)
	if err != nil {
		return err
	}
	if !ValidateEventChain(events, 0, "") {
		return errors.New("事件日志校验失败")
	}
	return nil
}
