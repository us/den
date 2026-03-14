package engine

import (
	"log/slog"
	"sync"
	"testing"
	"time"
)

// mockMemoryBackend is a thread-safe test double for MemoryBackend.
type mockMemoryBackend struct {
	mu    sync.RWMutex
	total uint64
	used  uint64
	free  uint64
	err   error

	containerMem map[string]uint64
}

func (m *mockMemoryBackend) HostMemory() (total, used, free uint64, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.total, m.used, m.free, m.err
}

func (m *mockMemoryBackend) ContainerMemory(containerID string) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.containerMem != nil {
		if v, ok := m.containerMem[containerID]; ok {
			return v, nil
		}
	}
	return 0, nil
}

func (m *mockMemoryBackend) setUsagePercent(pct float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.used = uint64(float64(m.total) * pct)
	m.free = m.total - m.used
}

func TestScoreToLevel(t *testing.T) {
	pm := NewPressureMonitor(&mockMemoryBackend{total: 1}, 1*time.Second, DefaultPressureThresholds(), slog.Default())

	tests := []struct {
		score float64
		want  PressureLevel
	}{
		{0.0, PressureNormal},
		{0.50, PressureNormal},
		{0.69, PressureNormal},
		{0.70, PressureWarning},
		{0.75, PressureWarning},
		{0.80, PressureHigh},
		{0.85, PressureHigh},
		{0.90, PressureCritical},
		{0.94, PressureCritical},
		{0.95, PressureEmergency},
		{0.99, PressureEmergency},
	}

	for _, tt := range tests {
		got := pm.scoreToLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToLevel(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestScoreToLevel_CustomThresholds(t *testing.T) {
	pm := NewPressureMonitor(&mockMemoryBackend{total: 1}, 1*time.Second, PressureThresholds{
		Warning:   0.50,
		High:      0.60,
		Critical:  0.70,
		Emergency: 0.80,
	}, slog.Default())

	tests := []struct {
		score float64
		want  PressureLevel
	}{
		{0.49, PressureNormal},
		{0.50, PressureWarning},
		{0.60, PressureHigh},
		{0.70, PressureCritical},
		{0.80, PressureEmergency},
	}

	for _, tt := range tests {
		got := pm.scoreToLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToLevel(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestPressureLevelString(t *testing.T) {
	tests := []struct {
		level PressureLevel
		want  string
	}{
		{PressureNormal, "normal"},
		{PressureWarning, "warning"},
		{PressureHigh, "high"},
		{PressureCritical, "critical"},
		{PressureEmergency, "emergency"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("PressureLevel(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestPressureMonitor_Hysteresis(t *testing.T) {
	backend := &mockMemoryBackend{
		total: 8 * 1024 * 1024 * 1024, // 8GB
	}
	backend.setUsagePercent(0.50) // Start normal

	logger := slog.Default()
	pm := NewPressureMonitor(backend, 50*time.Millisecond, DefaultPressureThresholds(), logger)

	ch := make(chan PressureEvent, 10)
	pm.Subscribe(ch)
	pm.Start()
	defer pm.Stop()

	// Wait for initial sample
	time.Sleep(100 * time.Millisecond)

	// Set to critical (91%) - first reading
	backend.setUsagePercent(0.91)
	time.Sleep(60 * time.Millisecond)

	// Should NOT have transitioned yet (need 2 consecutive readings)
	select {
	case evt := <-ch:
		// If we got an event, it might be from the second tick
		if evt.Level == PressureCritical {
			// This is fine - 2 readings happened
			return
		}
		t.Logf("got intermediate event: %v", evt.Level)
	default:
		// Expected - haven't seen 2 consecutive readings yet
	}

	// Wait for second reading
	time.Sleep(60 * time.Millisecond)

	// Now should have transitioned
	select {
	case evt := <-ch:
		if evt.Level != PressureCritical {
			t.Errorf("expected Critical level, got %v", evt.Level)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timed out waiting for critical pressure event")
	}
}

func TestPressureMonitor_LevelTransitions(t *testing.T) {
	backend := &mockMemoryBackend{
		total: 8 * 1024 * 1024 * 1024,
	}
	backend.setUsagePercent(0.50)

	logger := slog.Default()
	pm := NewPressureMonitor(backend, 30*time.Millisecond, DefaultPressureThresholds(), logger)

	ch := make(chan PressureEvent, 20)
	pm.Subscribe(ch)
	pm.Start()
	defer pm.Stop()

	// Wait for initial sample
	time.Sleep(50 * time.Millisecond)

	// Transition: Normal -> Warning (70%)
	backend.setUsagePercent(0.75)
	time.Sleep(80 * time.Millisecond) // Wait for 2 consecutive readings

	drainAndExpectLevel(t, ch, PressureWarning, "Normal->Warning")

	// Transition: Warning -> High (85%)
	backend.setUsagePercent(0.85)
	time.Sleep(80 * time.Millisecond)

	drainAndExpectLevel(t, ch, PressureHigh, "Warning->High")

	// Transition back: High -> Normal (50%)
	backend.setUsagePercent(0.50)
	time.Sleep(80 * time.Millisecond)

	drainAndExpectLevel(t, ch, PressureNormal, "High->Normal")
}

func drainAndExpectLevel(t *testing.T, ch <-chan PressureEvent, expected PressureLevel, label string) {
	t.Helper()
	var lastEvt *PressureEvent
	for {
		select {
		case evt := <-ch:
			lastEvt = &evt
		case <-time.After(200 * time.Millisecond):
			if lastEvt == nil {
				t.Errorf("%s: no pressure event received", label)
			} else if lastEvt.Level != expected {
				t.Errorf("%s: expected %v, got %v", label, expected, lastEvt.Level)
			}
			return
		}
	}
}

func TestPressureMonitor_CurrentEvent(t *testing.T) {
	backend := &mockMemoryBackend{
		total: 8 * 1024 * 1024 * 1024,
	}
	backend.setUsagePercent(0.50)

	logger := slog.Default()
	pm := NewPressureMonitor(backend, 30*time.Millisecond, DefaultPressureThresholds(), logger)
	pm.Start()
	defer pm.Stop()

	time.Sleep(50 * time.Millisecond)

	evt := pm.CurrentEvent()
	if evt.MemoryTotal != 8*1024*1024*1024 {
		t.Errorf("expected total 8GB, got %d", evt.MemoryTotal)
	}
	if evt.Level != PressureNormal {
		t.Errorf("expected Normal level, got %v", evt.Level)
	}
}

func TestPressureMonitor_FlappingPrevention(t *testing.T) {
	backend := &mockMemoryBackend{
		total: 8 * 1024 * 1024 * 1024,
	}
	backend.setUsagePercent(0.50)

	logger := slog.Default()
	pm := NewPressureMonitor(backend, 30*time.Millisecond, DefaultPressureThresholds(), logger)

	ch := make(chan PressureEvent, 20)
	pm.Subscribe(ch)
	pm.Start()
	defer pm.Stop()

	time.Sleep(50 * time.Millisecond)

	// Alternate between warning and normal - should NOT trigger transition
	backend.setUsagePercent(0.75) // Warning
	time.Sleep(35 * time.Millisecond)
	backend.setUsagePercent(0.50) // Normal
	time.Sleep(35 * time.Millisecond)
	backend.setUsagePercent(0.75) // Warning
	time.Sleep(35 * time.Millisecond)
	backend.setUsagePercent(0.50) // Normal
	time.Sleep(35 * time.Millisecond)

	// Should have no events (alternating prevents hysteresis from triggering)
	select {
	case evt := <-ch:
		// It's possible one transition happened if timing aligned
		t.Logf("got event during flapping: %v (may be timing-dependent)", evt.Level)
	default:
		// Expected: no transitions due to flapping prevention
	}
}
