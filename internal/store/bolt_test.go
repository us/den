package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *BoltStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, err := NewBoltStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBoltStore_SandboxCRUD(t *testing.T) {
	s := newTestStore(t)

	record := &SandboxRecord{
		ID:        "sbx-001",
		Image:     "ubuntu:24.04",
		Status:    "running",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute).Truncate(time.Millisecond),
		Labels:    map[string]string{"env": "test"},
	}

	// Save
	err := s.SaveSandbox(record)
	require.NoError(t, err)

	// Get
	got, err := s.GetSandbox("sbx-001")
	require.NoError(t, err)
	assert.Equal(t, record.ID, got.ID)
	assert.Equal(t, record.Image, got.Image)
	assert.Equal(t, record.Status, got.Status)
	assert.Equal(t, record.Labels["env"], got.Labels["env"])

	// List
	list, err := s.ListSandboxes()
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update
	record.Status = "stopped"
	err = s.SaveSandbox(record)
	require.NoError(t, err)
	got, err = s.GetSandbox("sbx-001")
	require.NoError(t, err)
	assert.Equal(t, "stopped", got.Status)

	// Delete
	err = s.DeleteSandbox("sbx-001")
	require.NoError(t, err)

	// Verify deleted
	_, err = s.GetSandbox("sbx-001")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBoltStore_SandboxNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSandbox("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBoltStore_DeleteNonexistent(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteSandbox("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBoltStore_MultipleSandboxes(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 5; i++ {
		err := s.SaveSandbox(&SandboxRecord{
			ID:        "sbx-" + string(rune('a'+i)),
			Image:     "ubuntu:24.04",
			Status:    "running",
			CreatedAt: time.Now().UTC(),
		})
		require.NoError(t, err)
	}

	list, err := s.ListSandboxes()
	require.NoError(t, err)
	assert.Len(t, list, 5)
}

func TestBoltStore_SnapshotCRUD(t *testing.T) {
	s := newTestStore(t)

	snap := &SnapshotRecord{
		ID:        "snap-001",
		SandboxID: "sbx-001",
		Name:      "checkpoint-1",
		ImageID:   "sha256:abc123",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}

	// Save
	err := s.SaveSnapshot(snap)
	require.NoError(t, err)

	// Get
	got, err := s.GetSnapshot("snap-001")
	require.NoError(t, err)
	assert.Equal(t, snap.ID, got.ID)
	assert.Equal(t, snap.Name, got.Name)
	assert.Equal(t, snap.SandboxID, got.SandboxID)

	// List by sandbox ID
	snaps, err := s.ListSnapshots("sbx-001")
	require.NoError(t, err)
	assert.Len(t, snaps, 1)

	// List all
	snaps, err = s.ListSnapshots("")
	require.NoError(t, err)
	assert.Len(t, snaps, 1)

	// List for different sandbox
	snaps, err = s.ListSnapshots("sbx-other")
	require.NoError(t, err)
	assert.Len(t, snaps, 0)

	// Delete
	err = s.DeleteSnapshot("snap-001")
	require.NoError(t, err)

	_, err = s.GetSnapshot("snap-001")
	assert.Error(t, err)
}

func TestBoltStore_SnapshotNotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetSnapshot("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBoltStore_InvalidPath(t *testing.T) {
	_, err := NewBoltStore(filepath.Join(os.DevNull, "impossible", "path.db"))
	assert.Error(t, err)
}
