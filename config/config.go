package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AlarmSource represents different types of alarm sounds
type AlarmSource string

const (
	SourceBuzzer  AlarmSource = "buzzer"
	SourceSoother AlarmSource = "soother"
	SourceMP3     AlarmSource = "mp3"
	SourceRadio   AlarmSource = "radio"
)

// Alarm represents a single alarm configuration
type Alarm struct {
	ID               int         `json:"id"`
	Enabled          bool        `json:"enabled"`
	Time             string      `json:"time"` // HH:MM:SS format
	Days             []bool      `json:"days"` // 7 days, Sunday=0
	Source           AlarmSource `json:"source"`
	Volume           int         `json:"volume"`             // 1-100
	AlarmSourceValue string      `json:"alarm_source_value"` // file path for .tone/.mp3 files or directory/playlist path
	VolumeRamp       bool        `json:"volume_ramp"`        // Progressive volume increase
}

// SleepTimer represents a sleep timer configuration
type SleepTimer struct {
	Duration         int         `json:"duration"`           // Duration in minutes: 0 (disabled) or 5-120 (active)
	Source           AlarmSource `json:"source"`             // Sound source: soother, mp3, radio
	Volume           int         `json:"volume"`             // 1-100
	AlarmSourceValue string      `json:"alarm_source_value"` // file path for .tone/.mp3 files or directory/playlist path
}

// Config represents the application configuration
type Config struct {
	// Display settings
	Hour24Format bool   `json:"hour_24_format"`
	ShowSeconds  bool   `json:"show_seconds"` // Show seconds in time display
	FontName     string `json:"font_name"`    // Font name for ASCII art display
	Brightness   int    `json:"brightness"`   // 1-10
	Backlight    int    `json:"backlight"`    // 1-10

	// Alarms
	Alarm1 Alarm `json:"alarm1"`
	Alarm2 Alarm `json:"alarm2"`

	// Sleep Timer
	SleepTimer SleepTimer `json:"sleep_timer"`

	// Timers
	SnoozeMinutes int `json:"snooze_minutes"` // 5, 10, 15, 30

	// Audio settings
	PlayerCommand string `json:"player_command"` // e.g., "mpv"
	LastRadioURL  string `json:"last_radio_url"`
	LastMP3Path   string `json:"last_mp3_path"`

	// Sound directories
	BuzzerDir  string `json:"buzzer_dir"`  // Directory containing buzzer .tone files
	SootherDir string `json:"soother_dir"` // Directory containing soother .tone files

	// UI enable/disable
	ShowNavigationBar bool `json:"show_navigation_bar"`
	ShowSettingsBar   bool `json:"show_settings_bar"`   // show/hide [SETTINGS] [ALARM...] [SLEEP] bar
	ShowSleepTimer    bool `json:"show_sleep_timer"`    // show/hide just [SLEEP]
	ShowInactiveItems bool `json:"show_inactive_items"` // show/hide alarm1/2 indicator, sleep indicator if disabled
	ShowAlarm2        bool `json:"show_alarm_2"`        // show/hide [ALARM 2]
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Hour24Format: true,
		ShowSeconds:  true,
		FontName:     "big",
		Brightness:   5,
		Backlight:    5,
		Alarm1: Alarm{
			ID:         1,
			Enabled:    false,
			Time:       "07:00:00",
			Days:       []bool{false, true, true, true, true, true, false}, // Mon-Fri
			Source:     SourceBuzzer,
			Volume:     50,
			VolumeRamp: true,
		},
		Alarm2: Alarm{
			ID:         2,
			Enabled:    false,
			Time:       "07:30:00",
			Days:       []bool{false, true, true, true, true, true, false}, // Mon-Fri
			Source:     SourceBuzzer,
			Volume:     50,
			VolumeRamp: true,
		},
		SleepTimer: SleepTimer{
			Duration: 60, // Default 60 minutes
			Source:   SourceSoother,
			Volume:   30, // Lower volume for sleep timer
		},
		SnoozeMinutes:     5,
		PlayerCommand:     "mpv",
		BuzzerDir:         "include/sounds/buzzer",
		SootherDir:        "include/sounds/soother",
		ShowNavigationBar: true,
		ShowSettingsBar:   true,
		ShowSleepTimer:    true,
		ShowInactiveItems: true,
		ShowAlarm2:        true,
	}
}

// Load loads configuration from config.json
func Load() (*Config, error) {
	configPath := getConfigPath()

	// If config file doesn't exist, create it with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if saveErr := cfg.Save(); saveErr != nil {
			return cfg, fmt.Errorf("failed to save default config: %v", saveErr)
		}
		return cfg, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Always reset sleep timer duration to 0 on app start
	cfg.SleepTimer.Duration = 0

	return &cfg, nil
}

// Save saves the configuration to config.json
func (c *Config) Save() error {
	configPath := getConfigPath()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// IsAlarmActive checks if an alarm should be active for the given time
func (a *Alarm) IsAlarmActive(t time.Time) bool {
	if !a.Enabled {
		return false
	}

	// Check if today is enabled
	weekday := int(t.Weekday()) // Sunday = 0
	if !a.Days[weekday] {
		return false
	}

	// Check if current time matches alarm time (within 1 minute)
	currentTimeStr := t.Format("15:04:05")
	alarmTime := a.Time

	// Handle both HH:MM:SS and HH:MM formats for backwards compatibility
	if len(alarmTime) == 5 { // HH:MM format
		alarmTime += ":00"
	}

	AlarmTriggered := currentTimeStr == alarmTime

	// log.Println("Alarm triggered: ", AlarmTriggered)

	return AlarmTriggered
}

// FormatTime formats time according to 12/24 hour setting
func (c *Config) FormatTime(t time.Time) string {
	if c.Hour24Format {
		if c.ShowSeconds {
			return t.Format("15:04:05")
		}
		return t.Format("15:04")
	}
	if c.ShowSeconds {
		return t.Format("3:04:05 PM")
	}
	return t.Format("3:04 PM")
}

// getConfigPath returns the path to the config file
func getConfigPath() string {
	return "config.json"
}
