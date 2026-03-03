package store

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var (
	sandboxBucket  = []byte("sandboxes")
	snapshotBucket = []byte("snapshots")
)

// BoltStore implements Store using BoltDB.
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore opens or creates a BoltDB database at the given path.
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("opening bolt db at %s: %w", path, err)
	}

	// Create buckets
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(sandboxBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(snapshotBucket); err != nil {
			return err
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating buckets: %w", err)
	}

	return &BoltStore{db: db}, nil
}

func (s *BoltStore) SaveSandbox(record *SandboxRecord) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sandboxBucket)
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshalling sandbox: %w", err)
		}
		return b.Put([]byte(record.ID), data)
	})
}

func (s *BoltStore) GetSandbox(id string) (*SandboxRecord, error) {
	var record SandboxRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sandboxBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("sandbox %s: %w", id, ErrNotFound)
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *BoltStore) ListSandboxes() ([]*SandboxRecord, error) {
	var records []*SandboxRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sandboxBucket)
		return b.ForEach(func(k, v []byte) error {
			var record SandboxRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			records = append(records, &record)
			return nil
		})
	})
	return records, err
}

func (s *BoltStore) DeleteSandbox(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(sandboxBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("sandbox %s: %w", id, ErrNotFound)
		}
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) SaveSnapshot(record *SnapshotRecord) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(snapshotBucket)
		data, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshalling snapshot: %w", err)
		}
		return b.Put([]byte(record.ID), data)
	})
}

func (s *BoltStore) GetSnapshot(id string) (*SnapshotRecord, error) {
	var record SnapshotRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(snapshotBucket)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("snapshot %s: %w", id, ErrNotFound)
		}
		return json.Unmarshal(data, &record)
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *BoltStore) ListSnapshots(sandboxID string) ([]*SnapshotRecord, error) {
	var records []*SnapshotRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(snapshotBucket)
		return b.ForEach(func(k, v []byte) error {
			var record SnapshotRecord
			if err := json.Unmarshal(v, &record); err != nil {
				return err
			}
			if sandboxID == "" || record.SandboxID == sandboxID {
				records = append(records, &record)
			}
			return nil
		})
	})
	return records, err
}

func (s *BoltStore) DeleteSnapshot(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(snapshotBucket)
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("snapshot %s: %w", id, ErrNotFound)
		}
		return b.Delete([]byte(id))
	})
}

func (s *BoltStore) Close() error {
	return s.db.Close()
}
