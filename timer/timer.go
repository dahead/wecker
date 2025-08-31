package timer

import (
	"fmt"
	"sync"
	"time"
)

// TimerType represents different types of timers
type TimerType int

const (
	TypeSleep TimerType = iota
	TypeSnooze
)

// Timer represents an active timer
type Timer struct {
	Type      TimerType
	StartTime time.Time
	Duration  time.Duration
	EndTime   time.Time
	IsActive  bool
}

// Manager manages sleep and snooze timers
type Manager struct {
	activeTimers map[TimerType]*Timer
	mutex        sync.RWMutex
	callbacks    TimerCallbacks
}

// TimerCallbacks defines callback functions for timer events
type TimerCallbacks struct {
	OnSleepTimerExpired  func()
	OnSnoozeTimerExpired func()
	OnTimerStarted       func(timerType TimerType, duration time.Duration)
	OnTimerStopped       func(timerType TimerType)
}

// NewManager creates a new timer manager
func NewManager() *Manager {
	return &Manager{
		activeTimers: make(map[TimerType]*Timer),
	}
}

// SetCallbacks sets the callback functions for timer events
func (m *Manager) SetCallbacks(callbacks TimerCallbacks) {
	m.callbacks = callbacks
}

// Start begins the timer monitoring loop
func (m *Manager) Start() {
	go m.monitorLoop()
}

// monitorLoop continuously checks for timer expiration
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			m.checkTimers(now)
		}
	}
}

// checkTimers checks if any timers have expired
func (m *Manager) checkTimers(now time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for timerType, timer := range m.activeTimers {
		if timer.IsActive && now.After(timer.EndTime) {
			timer.IsActive = false

			// Call appropriate callback
			switch timerType {
			case TypeSleep:
				if m.callbacks.OnSleepTimerExpired != nil {
					go m.callbacks.OnSleepTimerExpired()
				}
			case TypeSnooze:
				if m.callbacks.OnSnoozeTimerExpired != nil {
					go m.callbacks.OnSnoozeTimerExpired()
				}
			}

			// Remove expired timer
			delete(m.activeTimers, timerType)
		}
	}
}

// StartSleepTimer starts a sleep timer with specified minutes
func (m *Manager) StartSleepTimer(minutes int) bool {
	// Accept any value between 5-120 minutes for slider control
	if minutes < 5 || minutes > 120 {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	duration := time.Duration(minutes) * time.Minute
	now := time.Now()

	timer := &Timer{
		Type:      TypeSleep,
		StartTime: now,
		Duration:  duration,
		EndTime:   now.Add(duration),
		IsActive:  true,
	}

	m.activeTimers[TypeSleep] = timer

	// Call start callback
	if m.callbacks.OnTimerStarted != nil {
		go m.callbacks.OnTimerStarted(TypeSleep, duration)
	}

	return true
}

// StartSnoozeTimer starts a snooze timer with specified minutes
func (m *Manager) StartSnoozeTimer(minutes int) bool {
	validMinutes := []int{5, 10, 15, 30}
	isValid := false
	for _, valid := range validMinutes {
		if minutes == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		return false
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	duration := time.Duration(minutes) * time.Minute
	now := time.Now()

	timer := &Timer{
		Type:      TypeSnooze,
		StartTime: now,
		Duration:  duration,
		EndTime:   now.Add(duration),
		IsActive:  true,
	}

	m.activeTimers[TypeSnooze] = timer

	// Call start callback
	if m.callbacks.OnTimerStarted != nil {
		go m.callbacks.OnTimerStarted(TypeSnooze, duration)
	}

	return true
}

// StopTimer stops a specific timer
func (m *Manager) StopTimer(timerType TimerType) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	timer, exists := m.activeTimers[timerType]
	if !exists || !timer.IsActive {
		return false
	}

	timer.IsActive = false
	delete(m.activeTimers, timerType)

	// Call stop callback
	if m.callbacks.OnTimerStopped != nil {
		go m.callbacks.OnTimerStopped(timerType)
	}

	return true
}

// GetTimeRemaining returns the remaining time for a specific timer
func (m *Manager) GetTimeRemaining(timerType TimerType) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	timer, exists := m.activeTimers[timerType]
	if !exists || !timer.IsActive {
		return 0
	}

	remaining := time.Until(timer.EndTime)
	if remaining < 0 {
		return 0
	}

	return remaining
}

// IsTimerActive checks if a specific timer is currently active
func (m *Manager) IsTimerActive(timerType TimerType) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	timer, exists := m.activeTimers[timerType]
	return exists && timer.IsActive
}

// GetSleepTimerOptions returns valid sleep timer duration options in minutes
func GetSleepTimerOptions() []int {
	return []int{15, 30, 45, 60, 90, 120}
}

// GetSnoozeTimerOptions returns valid snooze timer duration options in minutes
func GetSnoozeTimerOptions() []int {
	return []int{5, 10, 15, 30}
}

// CycleSleepTimer cycles through sleep timer options and returns the next option
func CycleSleepTimer(currentMinutes int) int {
	options := GetSleepTimerOptions()

	// Find current option index
	currentIndex := -1
	for i, option := range options {
		if option == currentMinutes {
			currentIndex = i
			break
		}
	}

	// Move to next option (cycle back to start if at end)
	nextIndex := (currentIndex + 1) % len(options)
	return options[nextIndex]
}

// CycleSnoozeTimer cycles through snooze timer options and returns the next option
func CycleSnoozeTimer(currentMinutes int) int {
	options := GetSnoozeTimerOptions()

	// Find current option index
	currentIndex := -1
	for i, option := range options {
		if option == currentMinutes {
			currentIndex = i
			break
		}
	}

	// Move to next option (cycle back to start if at end)
	nextIndex := (currentIndex + 1) % len(options)
	return options[nextIndex]
}

// FormatTimeRemaining formats remaining time as MM:SS
func FormatTimeRemaining(duration time.Duration) string {
	totalSeconds := int(duration.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
