package engine

import (
	"log/slog"
	"sync"
	"time"
)

// PressureLevel represents the current host memory pressure.
type PressureLevel int

const (
	PressureNormal    PressureLevel = iota // < 70%
	PressureWarning                        // 70-80%
	PressureHigh                           // 80-90%
	PressureCritical                       // 90-95%
	PressureEmergency                      // > 95%
)

func (p PressureLevel) String() string {
	switch p {
	case PressureNormal:
		return "normal"
	case PressureWarning:
		return "warning"
	case PressureHigh:
		return "high"
	case PressureCritical:
		return "critical"
	case PressureEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// PressureEvent is emitted when the pressure level changes.
type PressureEvent struct {
	Level       PressureLevel
	MemoryUsed  uint64
	MemoryTotal uint64
	Score       float64
	Timestamp   time.Time
}

// PressureThresholds defines the memory usage thresholds for each pressure level.
type PressureThresholds struct {
	Warning   float64 // default: 0.70
	High      float64 // default: 0.80
	Critical  float64 // default: 0.90
	Emergency float64 // default: 0.95
}

// DefaultPressureThresholds returns the default thresholds.
func DefaultPressureThresholds() PressureThresholds {
	return PressureThresholds{
		Warning:   0.70,
		High:      0.80,
		Critical:  0.90,
		Emergency: 0.95,
	}
}

// pressureDebounceSamples is the number of consecutive readings at a new level
// required before transitioning (hysteresis / flapping prevention).
const pressureDebounceSamples = 2

// PressureMonitor observes host memory pressure and notifies listeners.
// It implements hysteresis: a level change requires 2 consecutive matching readings.
type PressureMonitor struct {
	backend    MemoryBackend
	interval   time.Duration
	thresholds PressureThresholds
	listeners  []chan<- PressureEvent
	mu         sync.RWMutex
	stopCh     chan struct{}
	doneCh     chan struct{}
	logger     *slog.Logger

	// Current state
	currentLevel PressureLevel
	pendingLevel PressureLevel
	pendingCount int
	lastEvent    PressureEvent
	stopOnce     sync.Once
	startOnce    sync.Once
}

// NewPressureMonitor creates a new PressureMonitor.
func NewPressureMonitor(backend MemoryBackend, interval time.Duration, thresholds PressureThresholds, logger *slog.Logger) *PressureMonitor {
	return &PressureMonitor{
		backend:      backend,
		interval:     interval,
		thresholds:   thresholds,
		logger:       logger,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		currentLevel: PressureNormal,
	}
}

// Subscribe registers a channel to receive pressure events.
// The channel should be buffered to avoid blocking the monitor.
func (pm *PressureMonitor) Subscribe(ch chan<- PressureEvent) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.listeners = append(pm.listeners, ch)
}

// CurrentEvent returns the most recent pressure event.
func (pm *PressureMonitor) CurrentEvent() PressureEvent {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.lastEvent
}

// Start begins the monitoring loop in a goroutine. Safe to call multiple times.
func (pm *PressureMonitor) Start() {
	pm.startOnce.Do(func() {
		go pm.run()
	})
}

// Stop signals the monitor to stop and waits for the goroutine to finish.
// Safe to call multiple times.
func (pm *PressureMonitor) Stop() {
	pm.stopOnce.Do(func() {
		close(pm.stopCh)
	})
	<-pm.doneCh
}

func (pm *PressureMonitor) run() {
	defer close(pm.doneCh)
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	// Take an initial reading immediately (with panic recovery)
	pm.safeSample()

	for {
		select {
		case <-ticker.C:
			pm.safeSample()
		case <-pm.stopCh:
			return
		}
	}
}

func (pm *PressureMonitor) safeSample() {
	defer func() {
		if r := recover(); r != nil {
			pm.logger.Error("panic in pressure monitor, recovered", "panic", r)
		}
	}()
	pm.sample()
}

func (pm *PressureMonitor) sample() {
	total, used, _, err := pm.backend.HostMemory()
	if err != nil {
		pm.logger.Warn("failed to read host memory", "error", err)
		return
	}

	if total == 0 {
		return
	}

	score := float64(used) / float64(total)
	newLevel := pm.scoreToLevel(score)

	event := PressureEvent{
		Level:       newLevel,
		MemoryUsed:  used,
		MemoryTotal: total,
		Score:       score,
		Timestamp:   time.Now(),
	}

	// Determine transition under lock, notify outside lock
	var shouldNotify bool
	var oldLevel PressureLevel
	var listeners []chan<- PressureEvent

	pm.mu.Lock()

	// Hysteresis: require 2 consecutive readings at the same level to transition
	if newLevel != pm.currentLevel {
		if newLevel == pm.pendingLevel {
			pm.pendingCount++
		} else {
			pm.pendingLevel = newLevel
			pm.pendingCount = 1
		}

		if pm.pendingCount >= pressureDebounceSamples {
			oldLevel = pm.currentLevel
			pm.currentLevel = newLevel
			pm.pendingLevel = newLevel
			pm.pendingCount = 0
			shouldNotify = true

			listeners = make([]chan<- PressureEvent, len(pm.listeners))
			copy(listeners, pm.listeners)
		}
	} else {
		// Level matches current; reset pending
		pm.pendingLevel = newLevel
		pm.pendingCount = 0
	}
	// Store event with confirmed (hysteresis-filtered) level for external readers
	event.Level = pm.currentLevel
	pm.lastEvent = event
	pm.mu.Unlock()

	if shouldNotify {
		pm.logger.Info("pressure level changed",
			"from", oldLevel.String(),
			"to", newLevel.String(),
			"score", score,
		)

		// Notify listeners (non-blocking)
		for _, ch := range listeners {
			select {
			case ch <- event:
			default:
				pm.logger.Warn("pressure listener channel full, dropping event")
			}
		}
	}
}

func (pm *PressureMonitor) scoreToLevel(score float64) PressureLevel {
	switch {
	case score >= pm.thresholds.Emergency:
		return PressureEmergency
	case score >= pm.thresholds.Critical:
		return PressureCritical
	case score >= pm.thresholds.High:
		return PressureHigh
	case score >= pm.thresholds.Warning:
		return PressureWarning
	default:
		return PressureNormal
	}
}
