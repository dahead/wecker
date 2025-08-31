package alarm

import (
	"sync"
	"time"
	"wecker/config"
)

// AlarmState represents the current state of an alarm
type AlarmState int

const (
	StateOff AlarmState = iota
	StateActive
	StateSnoozed
	StateTriggered
)

// ActiveAlarm represents an alarm that is currently running
type ActiveAlarm struct {
	Alarm       *config.Alarm
	State       AlarmState
	StartTime   time.Time
	SnoozeUntil time.Time
	Duration    time.Duration
}

// Manager manages the alarm system
type Manager struct {
	config       *config.Config
	activeAlarms map[int]*ActiveAlarm
	mutex        sync.RWMutex
	callbacks    AlarmCallbacks
}

// AlarmCallbacks defines callback functions for alarm events
type AlarmCallbacks struct {
	OnAlarmTriggered func(alarmID int, alarm *config.Alarm)
	OnAlarmSnoozed   func(alarmID int, duration time.Duration)
	OnAlarmStopped   func(alarmID int)
}

// NewManager creates a new alarm manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:       cfg,
		activeAlarms: make(map[int]*ActiveAlarm),
	}
}

// SetCallbacks sets the callback functions for alarm events
func (m *Manager) SetCallbacks(callbacks AlarmCallbacks) {
	m.callbacks = callbacks
}

// Start begins the alarm monitoring loop
func (m *Manager) Start() {
	go m.monitorLoop()
}

// monitorLoop continuously checks for alarm triggers
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			m.checkAlarms(now)
			m.updateActiveAlarms(now)
		}
	}
}

// checkAlarms checks if any alarms should be triggered
func (m *Manager) checkAlarms(now time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check Alarm 1
	if m.config.Alarm1.IsAlarmActive(now) {
		if _, exists := m.activeAlarms[1]; !exists {
			m.triggerAlarm(1, &m.config.Alarm1, now)
		}
	}

	// Check Alarm 2
	if m.config.Alarm2.IsAlarmActive(now) {
		if _, exists := m.activeAlarms[2]; !exists {
			m.triggerAlarm(2, &m.config.Alarm2, now)
		}
	}
}

// triggerAlarm triggers a specific alarm
func (m *Manager) triggerAlarm(alarmID int, alarm *config.Alarm, now time.Time) {
	activeAlarm := &ActiveAlarm{
		Alarm:     alarm,
		State:     StateTriggered,
		StartTime: now,
		Duration:  0,
	}

	m.activeAlarms[alarmID] = activeAlarm

	// Call trigger callback
	if m.callbacks.OnAlarmTriggered != nil {
		go m.callbacks.OnAlarmTriggered(alarmID, alarm)
	}
}

// updateActiveAlarms updates the state of currently active alarms
func (m *Manager) updateActiveAlarms(now time.Time) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for alarmID, activeAlarm := range m.activeAlarms {
		switch activeAlarm.State {
		case StateTriggered:
			// Check if alarm has been running for 60 minutes
			if now.Sub(activeAlarm.StartTime) >= 60*time.Minute {
				m.stopAlarmInternal(alarmID)
			}

		case StateSnoozed:
			// Check if snooze time has elapsed
			if now.After(activeAlarm.SnoozeUntil) {
				// Re-trigger the alarm
				activeAlarm.State = StateTriggered
				if m.callbacks.OnAlarmTriggered != nil {
					go m.callbacks.OnAlarmTriggered(alarmID, activeAlarm.Alarm)
				}
			}
		}
	}
}

// SnoozeAlarm snoozes an active alarm
func (m *Manager) SnoozeAlarm(alarmID int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	activeAlarm, exists := m.activeAlarms[alarmID]
	if !exists || activeAlarm.State != StateTriggered {
		return false
	}

	// Set snooze duration
	snoozeDuration := time.Duration(m.config.SnoozeMinutes) * time.Minute
	activeAlarm.State = StateSnoozed
	activeAlarm.SnoozeUntil = time.Now().Add(snoozeDuration)

	// Call snooze callback
	if m.callbacks.OnAlarmSnoozed != nil {
		go m.callbacks.OnAlarmSnoozed(alarmID, snoozeDuration)
	}

	return true
}

// StopAlarm stops an active alarm
func (m *Manager) StopAlarm(alarmID int) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.stopAlarmInternal(alarmID)
}

// stopAlarmInternal stops an alarm (internal, assumes mutex is held)
func (m *Manager) stopAlarmInternal(alarmID int) bool {
	_, exists := m.activeAlarms[alarmID]
	if !exists {
		return false
	}

	delete(m.activeAlarms, alarmID)

	// Call stop callback
	if m.callbacks.OnAlarmStopped != nil {
		go m.callbacks.OnAlarmStopped(alarmID)
	}

	return true
}

// GetActiveAlarms returns a copy of currently active alarms
func (m *Manager) GetActiveAlarms() map[int]*ActiveAlarm {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[int]*ActiveAlarm)
	for id, alarm := range m.activeAlarms {
		// Create a copy
		alarmCopy := *alarm
		result[id] = &alarmCopy
	}

	return result
}

// IsAlarmActive checks if a specific alarm is currently active
func (m *Manager) IsAlarmActive(alarmID int) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.activeAlarms[alarmID]
	return exists
}

// GetAlarmState returns the current state of an alarm
func (m *Manager) GetAlarmState(alarmID int) AlarmState {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if activeAlarm, exists := m.activeAlarms[alarmID]; exists {
		return activeAlarm.State
	}

	return StateOff
}

// SetSnoozeTime updates the snooze duration
func (m *Manager) SetSnoozeTime(minutes int) {
	validMinutes := []int{5, 7, 10, 15, 30, 45, 60, 90, 120}
	for _, valid := range validMinutes {
		if minutes == valid {
			m.config.SnoozeMinutes = minutes
			return
		}
	}
}

// GetSnoozeTimeRemaining returns remaining snooze time for an alarm
func (m *Manager) GetSnoozeTimeRemaining(alarmID int) time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if activeAlarm, exists := m.activeAlarms[alarmID]; exists && activeAlarm.State == StateSnoozed {
		remaining := time.Until(activeAlarm.SnoozeUntil)
		if remaining > 0 {
			return remaining
		}
	}

	return 0
}

// UpdateConfig updates the configuration reference
func (m *Manager) UpdateConfig(cfg *config.Config) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.config = cfg
}
