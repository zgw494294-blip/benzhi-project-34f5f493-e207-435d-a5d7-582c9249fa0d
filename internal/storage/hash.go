package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func eventHash(event Event) (string, error) {
	copyEvent := event
	copyEvent.Hash = ""
	payload, err := json.Marshal(copyEvent)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func snapshotChecksum(snapshot Snapshot) (string, error) {
	copySnapshot := snapshot
	copySnapshot.Checksum = ""
	payload, err := json.Marshal(copySnapshot)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateEventChain(events []Event, initialSequence int64, initialHash string) bool {
	sequence := initialSequence
	previous := initialHash
	for _, event := range events {
		if event.Sequence != sequence+1 || event.PreviousHash != previous {
			return false
		}
		hash, err := eventHash(event)
		if err != nil || hash != event.Hash {
			return false
		}
		sequence = event.Sequence
		previous = event.Hash
	}
	return true
}
