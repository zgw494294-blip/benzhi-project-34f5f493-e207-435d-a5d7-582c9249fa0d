package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"powerpermit/internal/domain"
)

type Repository struct {
	mu           sync.RWMutex
	dir          string
	eventPath    string
	snapshotPath string
	snapshot     Snapshot
	now          func() time.Time
}

func Open(dir string) (*Repository, error) {
	if dir == "" {
		return nil, errors.New("存储目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建存储目录: %w", err)
	}
	r := &Repository{dir: dir, eventPath: filepath.Join(dir, "events.jsonl"), snapshotPath: filepath.Join(dir, "snapshot.json"), now: time.Now}
	r.snapshot = Snapshot{Cases: map[string]domain.Aggregate{}, Idempotency: map[string]CommandResult{}}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Repository) load() error {
	data, err := os.ReadFile(r.snapshotPath)
	if err == nil {
		var snapshot Snapshot
		if json.Unmarshal(data, &snapshot) == nil {
			expected, checksumErr := snapshotChecksum(snapshot)
			if checksumErr == nil && expected == snapshot.Checksum {
				r.snapshot = snapshot
				if r.snapshot.Cases == nil {
					r.snapshot.Cases = map[string]domain.Aggregate{}
				}
				if r.snapshot.Idempotency == nil {
					r.snapshot.Idempotency = map[string]CommandResult{}
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取快照: %w", err)
	}
	events, err := r.readEventsAfter(r.snapshot.Sequence)
	if err != nil {
		return err
	}
	if !ValidateEventChain(events, r.snapshot.Sequence, r.snapshot.LastHash) {
		return errors.New("事件日志校验链不连续")
	}
	for _, event := range events {
		var aggregate domain.Aggregate
		if err := json.Unmarshal(event.Payload, &aggregate); err != nil {
			return fmt.Errorf("重放事件 %d: %w", event.Sequence, err)
		}
		r.snapshot.Cases[event.CaseID] = aggregate
		r.snapshot.Idempotency[event.IdempotencyKey] = CommandResult{CaseID: event.CaseID, Version: event.Version, EventKind: event.Kind}
		r.snapshot.Sequence = event.Sequence
		r.snapshot.LastHash = event.Hash
	}
	if len(events) > 0 {
		return r.writeSnapshotLocked()
	}
	return nil
}

func (r *Repository) readEventsAfter(sequence int64) ([]Event, error) {
	file, err := os.Open(r.eventPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("打开事件日志: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(file))
	var events []Event
	for {
		var event Event
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("解析事件日志: %w", err)
		}
		if event.Sequence > sequence {
			events = append(events, event)
		}
	}
	return events, nil
}

func (r *Repository) Get(caseID string) (domain.Aggregate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	aggregate, ok := r.snapshot.Cases[caseID]
	if !ok {
		return domain.Aggregate{}, domain.ErrNotFound
	}
	return cloneAggregate(aggregate)
}

func (r *Repository) List(status domain.CaseStatus) ([]domain.Aggregate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.Aggregate, 0, len(r.snapshot.Cases))
	for _, aggregate := range r.snapshot.Cases {
		if status != "" && aggregate.Case.Status != status {
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

func (r *Repository) IdempotentResult(key string) (CommandResult, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result, ok := r.snapshot.Idempotency[key]
	return result, ok
}

func (r *Repository) Commit(m Mutation) (CommandResult, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m.IdempotencyKey == "" {
		return CommandResult{}, false, errors.New("idempotencyKey 不能为空")
	}
	if existing, ok := r.snapshot.Idempotency[m.IdempotencyKey]; ok {
		return existing, true, nil
	}
	current, exists := r.snapshot.Cases[m.CaseID]
	if !exists {
		if m.ExpectedVersion != 0 {
			return CommandResult{}, false, domain.ErrConflict
		}
	} else if current.Case.Version != m.ExpectedVersion {
		return CommandResult{}, false, domain.ErrConflict
	}
	if exists && m.Aggregate.Case.Version != m.ExpectedVersion+1 {
		return CommandResult{}, false, errors.New("提交后的案件版本必须递增一次")
	}
	if !exists && m.Aggregate.Case.Version != 1 {
		return CommandResult{}, false, errors.New("新案件版本必须为 1")
	}
	if err := ValidateAggregate(m.Aggregate); err != nil {
		return CommandResult{}, false, fmt.Errorf("案件投影校验失败: %w", err)
	}
	// The aggregate stored in the snapshot must be fully isolated from the
	// caller's object, so that mutating fields or nested slices of the value
	// returned by a successful command cannot leak into Get/List/History
	// results. cloneAggregate performs a deep copy via JSON round-trip.
	stored, err := cloneAggregate(m.Aggregate)
	if err != nil {
		return CommandResult{}, false, err
	}
	payload, err := json.Marshal(m.Aggregate)
	if err != nil {
		return CommandResult{}, false, err
	}
	response, err := json.Marshal(m.Response)
	if err != nil {
		return CommandResult{}, false, err
	}
	event := Event{Sequence: r.snapshot.Sequence + 1, CaseID: m.CaseID, Kind: m.Kind, Actor: m.Actor, IdempotencyKey: m.IdempotencyKey, Version: m.Aggregate.Case.Version, OccurredAt: r.now().UTC(), Payload: payload, PreviousHash: r.snapshot.LastHash}
	event.Hash, err = eventHash(event)
	if err != nil {
		return CommandResult{}, false, err
	}
	if err := r.appendEventLocked(event); err != nil {
		return CommandResult{}, false, err
	}
	result := CommandResult{CaseID: m.CaseID, Version: m.Aggregate.Case.Version, EventKind: m.Kind, Response: response}
	r.snapshot.Sequence = event.Sequence
	r.snapshot.LastHash = event.Hash
	r.snapshot.Cases[m.CaseID] = stored
	r.snapshot.Idempotency[m.IdempotencyKey] = result
	if err := r.writeSnapshotLocked(); err != nil {
		return CommandResult{}, false, err
	}
	return result, false, nil
}

func (r *Repository) appendEventLocked(event Event) error {
	file, err := os.OpenFile(r.eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("打开事件日志: %w", err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(event); err != nil {
		file.Close()
		return fmt.Errorf("追加事件: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("同步事件: %w", err)
	}
	return file.Close()
}

func (r *Repository) writeSnapshotLocked() error {
	checksum, err := snapshotChecksum(r.snapshot)
	if err != nil {
		return err
	}
	r.snapshot.Checksum = checksum
	data, err := json.MarshalIndent(r.snapshot, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(r.dir, "snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err = io.Copy(temp, bytes.NewReader(data)); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, r.snapshotPath); err != nil {
		return err
	}
	dir, err := os.Open(r.dir)
	if err == nil {
		err = dir.Sync()
		dir.Close()
	}
	return err
}

func cloneAggregate(input domain.Aggregate) (domain.Aggregate, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return domain.Aggregate{}, err
	}
	var result domain.Aggregate
	err = json.Unmarshal(data, &result)
	return result, err
}
