package display

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"wecker/alarm"
	"wecker/audio"
	"wecker/config"
	"wecker/timer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

// AppState represents the current state of the application
type AppState int

const (
	StateMainClock AppState = iota
	StateSettings
	StateAlarmEdit
	StateSleepEdit
	StateTimeInput
	StateAlarmDays
	StateAlarmSource
	StateAlarmVolume
	StateAlarmToneSelect
	StateAlarmCustomPath
	StateSleepDuration
	StateSleepVolume
	StateSleepSoundSelect
	StateSleepCustomPath
)

// App holds the main TUI application
type App struct {
	program      *tea.Program
	config       *config.Config
	alarmManager *alarm.Manager
	timerManager *timer.Manager
	audioPlayer  *audio.Player

	// UI state
	state           AppState
	selectedMenu    int
	editingAlarm    int // 1 or 2
	timeInput       string
	customPathInput string
	availableTones  []string
	availableFonts  []string

	// Styles with modern hacker colors
	titleStyle       lipgloss.Style
	menuStyle        lipgloss.Style
	selectedStyle    lipgloss.Style
	timeStyle        lipgloss.Style
	errorStyle       lipgloss.Style
	instructionStyle lipgloss.Style
}

// Model represents the bubbletea model
type Model struct {
	app *App
}

// TickMsg is sent every second to update the clock
type TickMsg time.Time

// NewApp creates a new display application with modern styling
func NewApp(cfg *config.Config, alarmMgr *alarm.Manager, timerMgr *timer.Manager, audioPlayer *audio.Player) *App {
	app := &App{
		config:       cfg,
		alarmManager: alarmMgr,
		timerManager: timerMgr,
		audioPlayer:  audioPlayer,
		state:        StateMainClock,
		selectedMenu: 0,
		// fontName is now stored in config
		availableTones: discoverToneFiles(),
		availableFonts: []string{"big", "small", "3d", "3x5", "5lineoblique", "alphabet", "banner", "doh", "isometric1", "letters", "alligator"},

		// Modern hacker-style color scheme
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true).
			Align(lipgloss.Center),

		menuStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#333333")).
			Padding(0, 1),

		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#00FF00")).
			Padding(0, 1).
			Bold(true),

		timeStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Align(lipgloss.Center),

		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true),

		instructionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center),
	}

	model := Model{app: app}
	app.program = tea.NewProgram(model, tea.WithAltScreen())
	return app
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})

	case tea.KeyMsg:
		// Handle sleep timer stop on any key press when active
		if m.app.timerManager.IsTimerActive(timer.TypeSleep) && m.app.state == StateMainClock {
			// Stop sleep timer on any button press when active
			m.app.timerManager.StopTimer(timer.TypeSleep)
			return m, nil
		}

		// Handle active alarms - snooze with S or SPACE
		activeAlarms := m.app.alarmManager.GetActiveAlarms()
		if len(activeAlarms) > 0 && m.app.state == StateMainClock {
			// Handle snooze with S or SPACE keys
			if msg.String() == "s" || msg.String() == " " {
				for alarmID := range activeAlarms {
					m.app.alarmManager.SnoozeAlarm(alarmID)
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "ctrl+c", "q":
			// IMPORTANT: Save config before quitting to fix alarm settings saving issue
			if err := m.app.config.Save(); err != nil {
				// Log error but don't prevent quit
			}
			return m, tea.Quit

		case "esc":
			switch m.app.state {
			case StateSettings:
				m.app.state = StateMainClock
				m.app.selectedMenu = 0
			case StateAlarmEdit:
				m.app.state = StateMainClock
				m.app.selectedMenu = m.app.editingAlarm - 1
			case StateSleepEdit:
				// Start sleep timer when leaving settings if enabled
				if m.app.config.SleepTimer.Enabled {
					m.app.timerManager.StartSleepTimer(m.app.config.SleepTimer.Duration)
				}
				m.app.state = StateMainClock
				m.app.selectedMenu = 3 // Sleep menu index
			case StateTimeInput, StateAlarmDays, StateAlarmVolume, StateAlarmToneSelect, StateAlarmCustomPath:
				m.app.state = StateAlarmEdit
				m.app.selectedMenu = 0
				m.app.customPathInput = "" // Clear input on cancel
			case StateSleepDuration, StateSleepVolume, StateSleepSoundSelect, StateSleepCustomPath:
				// Start sleep timer when leaving duration settings if enabled
				if m.app.state == StateSleepDuration && m.app.config.SleepTimer.Enabled {
					m.app.timerManager.StartSleepTimer(m.app.config.SleepTimer.Duration)
				}
				m.app.state = StateSleepEdit
				m.app.selectedMenu = 0
				m.app.customPathInput = "" // Clear input on cancel
			case StateMainClock:
				// Already at main clock, do nothing
			default:
				m.app.state = StateMainClock
				m.app.selectedMenu = 0
			}

		case "enter":
			return m.handleEnter()

		case "up", "k":
			return m.handleUp()

		case "down", "j":
			return m.handleDown()

		case "left", "h":
			return m.handleLeft()

		case "right", "l":
			return m.handleRight()

		case "t":
			// Simple time editing - press T to enter time input
			if m.app.state == StateAlarmEdit {
				m.app.state = StateTimeInput
				// Pre-fill with current alarm time
				var alarm *config.Alarm
				if m.app.editingAlarm == 1 {
					alarm = &m.app.config.Alarm1
				} else {
					alarm = &m.app.config.Alarm2
				}
				// Extract HH:MM from the time string (remove seconds if present)
				m.app.timeInput = alarm.Time[:5]
			}

		case "e":
			// Toggle alarm enabled/disabled
			if m.app.state == StateAlarmEdit {
				if m.app.editingAlarm == 1 {
					m.app.config.Alarm1.Enabled = !m.app.config.Alarm1.Enabled
				} else {
					m.app.config.Alarm2.Enabled = !m.app.config.Alarm2.Enabled
				}
				// Save immediately when toggling alarm
				m.app.config.Save()
			}

		default:
			if m.app.state == StateTimeInput {
				return m.handleTimeInput(msg.String())
			} else if m.app.state == StateAlarmCustomPath {
				return m.handleCustomPathInput(msg.String())
			}
		}
	}

	return m, nil
}

// View renders the UI
func (m Model) View() string {
	switch m.app.state {
	case StateMainClock:
		return m.renderMainClock()
	case StateSettings:
		return m.renderSettings()
	case StateAlarmEdit:
		return m.renderAlarmEdit()
	case StateSleepEdit:
		return m.renderSleepEdit()
	case StateTimeInput:
		return m.renderTimeInput()
	case StateAlarmDays:
		return m.renderAlarmDays()
	case StateAlarmVolume:
		return m.renderAlarmVolume()
	case StateAlarmToneSelect:
		return m.renderAlarmToneSelect()
	case StateAlarmCustomPath:
		return m.renderAlarmCustomPath()
	case StateSleepDuration:
		return m.renderSleepDuration()
	case StateSleepVolume:
		return m.renderSleepVolume()
	case StateSleepSoundSelect:
		return m.renderSleepSoundSelect()
	case StateSleepCustomPath:
		return m.renderSleepCustomPath()
	default:
		return m.renderMainClock()
	}
}

// Render main clock view with ASCII art using go-figure
func (m Model) renderMainClock() string {
	var content strings.Builder

	// Get current time
	now := time.Now()
	timeStr := m.app.config.FormatTime(now)

	// Create ASCII art time using go-figure with configurable font
	asciiTime := figure.NewFigure(timeStr, m.app.config.FontName, true).String()

	// Style the ASCII time with cyan color
	styledTime := m.app.timeStyle.Render(asciiTime)

	// Build centered layout
	content.WriteString("\n\n")
	content.WriteString(styledTime)
	content.WriteString("\n\n")

	// Add alarm status with colors
	content.WriteString(m.renderAlarmStatus())
	content.WriteString("\n\n")

	// Add simple bottom menu bar
	content.WriteString(m.renderBottomMenu())
	content.WriteString("\n\n")

	// Add navigation instructions
	instructions := "← → to navigate  •  ENTER to select  •  Q to quit"
	content.WriteString(m.app.instructionStyle.Render(instructions))

	return content.String()
}

// Render alarm status with modern colors
func (m Model) renderAlarmStatus() string {
	var status strings.Builder

	// Check for active alarms
	activeAlarms := m.app.alarmManager.GetActiveAlarms()

	// Regular alarm status display (enhanced to show active alarms)
	// Alarm 1 status
	alarm1Icon := "⏰"
	color1 := "#666666"
	_, isAlarm1Active := activeAlarms[1]

	if isAlarm1Active {
		alarm1Icon = "🔴"
		color1 = "#FF0000"
	} else if m.app.config.Alarm1.Enabled {
		alarm1Icon = "🔔"
		color1 = "#00FF00"
	}

	alarm1Text := fmt.Sprintf("%s ALARM 1: %s", alarm1Icon, m.app.config.Alarm1.Time[:5])
	if isAlarm1Active {
		alarm1Text += " [ACTIVE]"
	} else if m.app.config.Alarm1.Enabled {
		activeDays := m.getActiveDaysString(m.app.config.Alarm1.Days)
		alarm1Text += fmt.Sprintf(" [%s]", activeDays)
	} else {
		alarm1Text += " [OFF]"
	}

	status.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(color1)).
		Bold(isAlarm1Active).
		Render(alarm1Text))

	status.WriteString("    ")

	// Alarm 2 status
	alarm2Icon := "⏰"
	color2 := "#666666"
	_, isAlarm2Active := activeAlarms[2]

	if isAlarm2Active {
		alarm2Icon = "🔴"
		color2 = "#FF0000"
	} else if m.app.config.Alarm2.Enabled {
		alarm2Icon = "🔔"
		color2 = "#00FF00"
	}

	alarm2Text := fmt.Sprintf("%s ALARM 2: %s", alarm2Icon, m.app.config.Alarm2.Time[:5])
	if isAlarm2Active {
		alarm2Text += " [ACTIVE]"
	} else if m.app.config.Alarm2.Enabled {
		activeDays := m.getActiveDaysString(m.app.config.Alarm2.Days)
		alarm2Text += fmt.Sprintf(" [%s]", activeDays)
	} else {
		alarm2Text += " [OFF]"
	}

	status.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(color2)).
		Bold(isAlarm2Active).
		Render(alarm2Text))

	status.WriteString("    ")

	// Sleep Timer status (always shown)
	sleepIcon := "😴"
	colorSleep := "#666666"
	var sleepText string

	// Check if sleep timer is actually running
	if m.app.timerManager.IsTimerActive(timer.TypeSleep) {
		sleepIcon = "🌙"
		colorSleep = "#00FF00"
		remaining := m.app.timerManager.GetTimeRemaining(timer.TypeSleep)
		sleepText = fmt.Sprintf("%s SLEEP: %s", sleepIcon, timer.FormatTimeRemaining(remaining))
	} else if m.app.config.SleepTimer.Enabled {
		sleepIcon = "🌙"
		colorSleep = "#FFFF00" // Yellow for enabled but not running
		sleepText = fmt.Sprintf("%s SLEEP: %dm [READY]", sleepIcon, m.app.config.SleepTimer.Duration)
	} else {
		sleepText = fmt.Sprintf("%s SLEEP: %dm [OFF]", sleepIcon, m.app.config.SleepTimer.Duration)
	}

	status.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorSleep)).
		Render(sleepText))

	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(status.String())
}

// Get active days string
func (m Model) getActiveDaysString(days []bool) string {
	dayNames := []string{"S", "M", "T", "W", "T", "F", "S"}
	var active []string

	for i, enabled := range days {
		if enabled {
			active = append(active, dayNames[i])
		}
	}

	if len(active) == 0 {
		return "NONE"
	}
	return strings.Join(active, "")
}

// Render simple bottom menu (no complex arrows or styles)
func (m Model) renderBottomMenu() string {
	// Always show normal menu, even when alarms are active
	menuItems := []string{"SETTINGS", "ALARM 1", "ALARM 2", "SLEEP"}

	var rendered []string

	for i, item := range menuItems {
		style := m.app.menuStyle
		if i == m.app.selectedMenu && m.app.state == StateMainClock {
			style = m.app.selectedStyle
		}
		rendered = append(rendered, style.Render(fmt.Sprintf(" %s ", item)))
	}

	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, rendered...))
}

// Handle Enter key
func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.app.state {
	case StateMainClock:
		// Check if any alarms are active and handle stopping them
		activeAlarms := m.app.alarmManager.GetActiveAlarms()

		switch m.app.selectedMenu {
		case 0: // Settings
			m.app.state = StateSettings
			m.app.selectedMenu = 0
		case 1: // Alarm 1
			// If Alarm 1 is active, stop it; otherwise go to edit screen
			if _, isActive := activeAlarms[1]; isActive {
				m.app.alarmManager.StopAlarm(1)
			} else {
				m.app.state = StateAlarmEdit
				m.app.editingAlarm = 1
				m.app.selectedMenu = 0
			}
		case 2: // Alarm 2
			// If Alarm 2 is active, stop it; otherwise go to edit screen
			if _, isActive := activeAlarms[2]; isActive {
				m.app.alarmManager.StopAlarm(2)
			} else {
				m.app.state = StateAlarmEdit
				m.app.editingAlarm = 2
				m.app.selectedMenu = 0
			}
		case 3: // Sleep Timer
			m.app.state = StateSleepEdit
			m.app.selectedMenu = 0
		}
	case StateSettings:
		switch m.app.selectedMenu {
		case 0: // Font - RETURN key changes font as requested
			currentIndex := 0
			for i, font := range m.app.availableFonts {
				if font == m.app.config.FontName {
					currentIndex = i
					break
				}
			}
			m.app.config.FontName = m.app.availableFonts[(currentIndex+1)%len(m.app.availableFonts)]
			m.app.config.Save()
		case 1: // 24H Format
			m.app.config.Hour24Format = !m.app.config.Hour24Format
			m.app.config.Save()
		case 2: // Show Seconds
			m.app.config.ShowSeconds = !m.app.config.ShowSeconds
			m.app.config.Save()
		case 3: // Back
			m.app.state = StateMainClock
			m.app.selectedMenu = 0
		}
	case StateAlarmEdit:
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}

		maxOptions := 5 // Enabled, Time, Days, Volume, Source
		if alarm.Source == config.SourceBuzzer {
			maxOptions = 6 // Add Tone selection
		} else if alarm.Source == config.SourceMP3 || alarm.Source == config.SourceRadio {
			maxOptions = 6 // Add Custom path
		}

		switch m.app.selectedMenu {
		case 0: // Toggle enabled
			alarm.Enabled = !alarm.Enabled
			m.app.config.Save()
		case 1: // Edit time
			m.app.state = StateTimeInput
			// Pre-fill with current alarm time
			// Extract HH:MM from the time string (remove seconds if present)
			m.app.timeInput = alarm.Time[:5]
		case 2: // Edit days
			m.app.state = StateAlarmDays
			m.app.selectedMenu = 0
		case 3: // Edit volume
			m.app.state = StateAlarmVolume
		case 4: // Change source
			sources := []config.AlarmSource{config.SourceBuzzer, config.SourceMP3, config.SourceRadio}
			currentIndex := 0
			for i, source := range sources {
				if source == alarm.Source {
					currentIndex = i
					break
				}
			}
			alarm.Source = sources[(currentIndex+1)%len(sources)]
			// Reset source value when changing source
			alarm.AlarmSourceValue = ""
			m.app.config.Save()
		case 5: // Source-specific options
			if alarm.Source == config.SourceBuzzer {
				m.app.state = StateAlarmToneSelect
				m.app.selectedMenu = 0
			} else if alarm.Source == config.SourceMP3 || alarm.Source == config.SourceRadio {
				m.app.state = StateAlarmCustomPath
				m.app.customPathInput = alarm.AlarmSourceValue
			}
		default: // Back
			if m.app.selectedMenu >= maxOptions {
				m.app.state = StateMainClock
				m.app.selectedMenu = m.app.editingAlarm - 1
			}
		}
	case StateSleepEdit:
		sleepTimer := &m.app.config.SleepTimer
		maxOptions := 4 // Enabled, Duration, Volume, Source
		if sleepTimer.Source == config.SourceSoother {
			maxOptions = 5 // Add Sound selection
		} else if sleepTimer.Source == config.SourceMP3 || sleepTimer.Source == config.SourceRadio {
			maxOptions = 5 // Add Custom path
		}

		switch m.app.selectedMenu {
		case 0: // Toggle enabled
			sleepTimer.Enabled = !sleepTimer.Enabled
			m.app.config.Save()
		case 1: // Edit duration with slider
			m.app.state = StateSleepDuration
		case 2: // Volume
			m.app.state = StateSleepVolume
		case 3: // Change source
			sources := []config.AlarmSource{config.SourceSoother, config.SourceMP3, config.SourceRadio}
			currentIndex := 0
			for i, source := range sources {
				if source == sleepTimer.Source {
					currentIndex = i
					break
				}
			}
			sleepTimer.Source = sources[(currentIndex+1)%len(sources)]
			// Reset source value when changing source
			sleepTimer.AlarmSourceValue = ""
			m.app.config.Save()
		case 4: // Source-specific options
			if sleepTimer.Source == config.SourceSoother {
				m.app.state = StateSleepSoundSelect
				m.app.selectedMenu = 0
			} else if sleepTimer.Source == config.SourceMP3 || sleepTimer.Source == config.SourceRadio {
				m.app.state = StateSleepCustomPath
				m.app.customPathInput = sleepTimer.AlarmSourceValue
			}
		default: // Back
			if m.app.selectedMenu >= maxOptions {
				m.app.state = StateMainClock
				m.app.selectedMenu = 3 // Sleep menu index
			}
		}
	case StateTimeInput:
		// Process time input and save - ENTER leaves time set menu as requested
		if m.parseAndSetTime() {
			m.app.state = StateAlarmEdit
			m.app.config.Save()
		}
	case StateAlarmDays:
		// Toggle day selection
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}
		if m.app.selectedMenu < 7 {
			alarm.Days[m.app.selectedMenu] = !alarm.Days[m.app.selectedMenu]
			m.app.config.Save()
		}
	case StateAlarmVolume:
		// Volume handled by left/right keys
	case StateAlarmToneSelect:
		// Select tone file
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}
		if m.app.selectedMenu < len(m.app.availableTones) {
			alarm.AlarmSourceValue = "include/sounds/buzzer/" + m.app.availableTones[m.app.selectedMenu]
			m.app.config.Save()
			m.app.state = StateAlarmEdit
			m.app.selectedMenu = 5
		}
	case StateAlarmCustomPath:
		// Save custom path
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}
		alarm.AlarmSourceValue = m.app.customPathInput
		m.app.config.Save()
		m.app.state = StateAlarmEdit
		m.app.selectedMenu = 5
		m.app.customPathInput = ""
	case StateSleepDuration:
		// Duration handled by left/right keys
	case StateSleepVolume:
		// Volume handled by left/right keys
	case StateSleepSoundSelect:
		// Select soother sound
		availableSounds := getAvailableFiles(config.SourceSoother)
		if m.app.selectedMenu < len(availableSounds) {
			m.app.config.SleepTimer.AlarmSourceValue = "include/sounds/soother/" + availableSounds[m.app.selectedMenu]
			m.app.config.Save()
			m.app.state = StateSleepEdit
			m.app.selectedMenu = 4
		}
	case StateSleepCustomPath:
		// Save custom path for sleep timer
		m.app.config.SleepTimer.AlarmSourceValue = m.app.customPathInput
		m.app.config.Save()
		m.app.state = StateSleepEdit
		m.app.selectedMenu = 4
		m.app.customPathInput = ""
	}

	return m, nil
}

// Enhanced navigation handlers for all states
func (m Model) handleUp() (tea.Model, tea.Cmd) {
	switch m.app.state {
	case StateSettings:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateAlarmEdit:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateSleepEdit:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateAlarmDays:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateAlarmToneSelect:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateSleepSoundSelect:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	}
	return m, nil
}

func (m Model) handleDown() (tea.Model, tea.Cmd) {
	switch m.app.state {
	case StateMainClock:
		// No up/down navigation on main clock
	case StateSettings:
		maxItems := 4 // Font, 24H, Seconds, Back
		if m.app.selectedMenu < maxItems-1 {
			m.app.selectedMenu++
		}
	case StateAlarmEdit:
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}

		maxOptions := 6 // Enabled, Time, Days, Volume, Source, Back
		if alarm.Source == config.SourceBuzzer {
			maxOptions = 7 // Add Tone selection
		} else if alarm.Source == config.SourceMP3 || alarm.Source == config.SourceRadio {
			maxOptions = 7 // Add Custom path
		}

		if m.app.selectedMenu < maxOptions-1 {
			m.app.selectedMenu++
		}
	case StateSleepEdit:
		sleepTimer := &m.app.config.SleepTimer
		maxItems := 5 // Enabled, Duration, Volume, Source, Back
		if sleepTimer.Source == config.SourceSoother {
			maxItems = 6 // Add Sound selection
		} else if sleepTimer.Source == config.SourceMP3 || sleepTimer.Source == config.SourceRadio {
			maxItems = 6 // Add Custom path
		}
		if m.app.selectedMenu < maxItems-1 {
			m.app.selectedMenu++
		}
	case StateAlarmDays:
		if m.app.selectedMenu < 6 { // 7 days (0-6)
			m.app.selectedMenu++
		}
	case StateAlarmToneSelect:
		if m.app.selectedMenu < len(m.app.availableTones)-1 {
			m.app.selectedMenu++
		}
	case StateSleepSoundSelect:
		availableSounds := getAvailableFiles(config.SourceSoother)
		if m.app.selectedMenu < len(availableSounds)-1 {
			m.app.selectedMenu++
		}
	}
	return m, nil
}

func (m Model) handleLeft() (tea.Model, tea.Cmd) {
	switch m.app.state {
	case StateMainClock:
		if m.app.selectedMenu > 0 {
			m.app.selectedMenu--
		}
	case StateAlarmVolume:
		// Decrease volume
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}
		if alarm.Volume > 0 {
			alarm.Volume -= 5
			if alarm.Volume < 0 {
				alarm.Volume = 0
			}
			m.app.config.Save()
		}
	case StateSleepDuration:
		// Decrease sleep timer duration
		sleepTimer := &m.app.config.SleepTimer
		if sleepTimer.Duration > 5 {
			sleepTimer.Duration -= 5
			if sleepTimer.Duration < 5 {
				sleepTimer.Duration = 5
			}
			// Auto-enable when duration > 0, but don't start timer yet
			if sleepTimer.Duration > 0 {
				sleepTimer.Enabled = true
			} else {
				sleepTimer.Enabled = false
				m.app.timerManager.StopTimer(timer.TypeSleep)
			}
			m.app.config.Save()
		}
	case StateSleepVolume:
		// Decrease sleep timer volume
		sleepTimer := &m.app.config.SleepTimer
		if sleepTimer.Volume > 0 {
			sleepTimer.Volume -= 5
			if sleepTimer.Volume < 0 {
				sleepTimer.Volume = 0
			}
			m.app.config.Save()
		}
	}
	return m, nil
}

func (m Model) handleRight() (tea.Model, tea.Cmd) {
	switch m.app.state {
	case StateMainClock:
		if m.app.selectedMenu < 3 {
			m.app.selectedMenu++
		}
	case StateAlarmVolume:
		// Increase volume
		var alarm *config.Alarm
		if m.app.editingAlarm == 1 {
			alarm = &m.app.config.Alarm1
		} else {
			alarm = &m.app.config.Alarm2
		}
		if alarm.Volume < 100 {
			alarm.Volume += 5
			if alarm.Volume > 100 {
				alarm.Volume = 100
			}
			m.app.config.Save()
		}
	case StateSleepDuration:
		// Increase sleep timer duration
		sleepTimer := &m.app.config.SleepTimer
		if sleepTimer.Duration < 120 {
			sleepTimer.Duration += 5
			if sleepTimer.Duration > 120 {
				sleepTimer.Duration = 120
			}
			// Auto-enable when duration > 0, but don't start timer yet
			if sleepTimer.Duration > 0 {
				sleepTimer.Enabled = true
			} else {
				sleepTimer.Enabled = false
				m.app.timerManager.StopTimer(timer.TypeSleep)
			}
			m.app.config.Save()
		}
	case StateSleepVolume:
		// Increase sleep timer volume
		sleepTimer := &m.app.config.SleepTimer
		if sleepTimer.Volume < 100 {
			sleepTimer.Volume += 5
			if sleepTimer.Volume > 100 {
				sleepTimer.Volume = 100
			}
			m.app.config.Save()
		}
	}
	return m, nil
}

// Simple time input - just numbers, no complex scrolling
func (m Model) handleTimeInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "backspace":
		if len(m.app.timeInput) > 0 {
			m.app.timeInput = m.app.timeInput[:len(m.app.timeInput)-1]
		}
	default:
		// Only allow digits and colon for simple input
		if len(key) == 1 && (key >= "0" && key <= "9" || key == ":") {
			if len(m.app.timeInput) < 8 { // HH:MM:SS format
				m.app.timeInput += key
			}
		}
	}
	return m, nil
}

// Handle custom path input for MP3/Radio URLs
func (m Model) handleCustomPathInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "backspace":
		if len(m.app.customPathInput) > 0 {
			m.app.customPathInput = m.app.customPathInput[:len(m.app.customPathInput)-1]
		}
	default:
		// Allow most printable characters for paths and URLs
		if len(key) == 1 && key >= " " && key <= "~" {
			if len(m.app.customPathInput) < 256 { // Reasonable limit for paths
				m.app.customPathInput += key
			}
		}
	}
	return m, nil
}

// Parse and set time with validation
func (m Model) parseAndSetTime() bool {
	// Simple validation for HH:MM or HH:MM:SS format
	parts := strings.Split(m.app.timeInput, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	// Validate hours
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}

	// Validate minutes
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return false
	}

	// Default seconds to 00 if not provided
	second := 0
	if len(parts) == 3 {
		second, err = strconv.Atoi(parts[2])
		if err != nil || second < 0 || second > 59 {
			return false
		}
	}

	// Format time string
	timeStr := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)

	// Set the alarm time
	if m.app.editingAlarm == 1 {
		m.app.config.Alarm1.Time = timeStr
	} else {
		m.app.config.Alarm2.Time = timeStr
	}

	m.app.timeInput = ""
	return true
}

// Render settings menu (simple, no complex styling)
func (m Model) renderSettings() string {
	var content strings.Builder

	content.WriteString(m.app.titleStyle.Render("⚙️  SETTINGS"))
	content.WriteString("\n\n")

	settings := []string{
		fmt.Sprintf("Font: %s", m.app.config.FontName),
		fmt.Sprintf("24H Format: %s", getBoolText(m.app.config.Hour24Format)),
		fmt.Sprintf("Show Seconds: %s", getBoolText(m.app.config.ShowSeconds)),
		"Back",
	}

	for i, setting := range settings {
		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s ", setting)))
		} else {
			content.WriteString(fmt.Sprintf("   %s", setting))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  F to change font  •  ENTER to toggle  •  ESC to return"))

	return content.String()
}

// Render alarm edit menu (enhanced with all configuration options)
func (m Model) renderAlarmEdit() string {
	var content strings.Builder

	title := fmt.Sprintf("🔔 ALARM %d CONFIGURATION", m.app.editingAlarm)
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	var alarm *config.Alarm
	if m.app.editingAlarm == 1 {
		alarm = &m.app.config.Alarm1
	} else {
		alarm = &m.app.config.Alarm2
	}

	menuOptions := []string{
		fmt.Sprintf("Enabled: %s", getBoolText(alarm.Enabled)),
		fmt.Sprintf("Time: %s", alarm.Time[:5]),
		fmt.Sprintf("Days: %s", m.getActiveDaysString(alarm.Days)),
		fmt.Sprintf("Volume: %d%%", alarm.Volume),
		fmt.Sprintf("Source: %s", alarm.Source),
	}

	// Add source-specific options
	if alarm.Source == config.SourceBuzzer {
		toneFile := alarm.AlarmSourceValue
		if toneFile == "" {
			toneFile = "pattern1.tone"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("Tone: %s", toneFile))
	} else if alarm.Source == config.SourceMP3 {
		mp3Path := alarm.AlarmSourceValue
		if mp3Path == "" {
			mp3Path = "<not set>"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("MP3 Path: %s", mp3Path))
	} else if alarm.Source == config.SourceRadio {
		radioURL := alarm.AlarmSourceValue
		if radioURL == "" {
			radioURL = "<not set>"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("Radio URL: %s", radioURL))
	}

	menuOptions = append(menuOptions, "Back")

	for i, option := range menuOptions {
		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s ", option)))
		} else {
			content.WriteString(fmt.Sprintf("   %s", option))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  ENTER to edit  •  ESC to return"))

	return content.String()
}

// Render sleep timer edit menu
func (m Model) renderSleepEdit() string {
	var content strings.Builder

	title := "🌙 SLEEP TIMER CONFIGURATION"
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	sleepTimer := &m.app.config.SleepTimer

	menuOptions := []string{
		fmt.Sprintf("Enabled: %s", getBoolText(sleepTimer.Enabled)),
		fmt.Sprintf("Duration: %d minutes", sleepTimer.Duration),
		fmt.Sprintf("Volume: %d%%", sleepTimer.Volume),
		fmt.Sprintf("Source: %s", sleepTimer.Source),
	}

	// Add source-specific options
	if sleepTimer.Source == config.SourceSoother {
		soundFile := sleepTimer.AlarmSourceValue
		if soundFile == "" {
			soundFile = "noise1.tone"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("Sound: %s", soundFile))
	} else if sleepTimer.Source == config.SourceMP3 {
		mp3Path := sleepTimer.AlarmSourceValue
		if mp3Path == "" {
			mp3Path = "<not set>"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("MP3 Path: %s", mp3Path))
	} else if sleepTimer.Source == config.SourceRadio {
		radioURL := sleepTimer.AlarmSourceValue
		if radioURL == "" {
			radioURL = "<not set>"
		}
		menuOptions = append(menuOptions, fmt.Sprintf("Radio URL: %s", radioURL))
	}

	menuOptions = append(menuOptions, "Back")

	for i, option := range menuOptions {
		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s ", option)))
		} else {
			content.WriteString(fmt.Sprintf("   %s", option))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  ENTER to edit  •  ESC to return"))

	return content.String()
}

// Render simple time input screen
func (m Model) renderTimeInput() string {
	var content strings.Builder

	title := fmt.Sprintf("⏰ SET TIME FOR ALARM %d", m.app.editingAlarm)
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	content.WriteString("Enter time in HH:MM format (24-hour)\n")
	content.WriteString("Examples: 07:30, 14:15, 23:45\n\n")

	inputDisplay := m.app.timeInput
	if len(inputDisplay) == 0 {
		inputDisplay = "HH:MM"
	}

	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1).
		Render(fmt.Sprintf(" %s_ ", inputDisplay)))

	content.WriteString("\n\n")
	content.WriteString(m.app.instructionStyle.Render("Type numbers and :  •  ENTER to save  •  ESC to cancel"))

	return content.String()
}

// Render day selection screen
func (m Model) renderAlarmDays() string {
	var content strings.Builder

	title := fmt.Sprintf("📅 SELECT DAYS FOR ALARM %d", m.app.editingAlarm)
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	var alarm *config.Alarm
	if m.app.editingAlarm == 1 {
		alarm = &m.app.config.Alarm1
	} else {
		alarm = &m.app.config.Alarm2
	}

	days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	for i, day := range days {
		checkbox := "[ ]"
		if alarm.Days[i] {
			checkbox = "[X]"
		}

		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s %s ", checkbox, day)))
		} else {
			content.WriteString(fmt.Sprintf("   %s %s", checkbox, day))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  ENTER to toggle  •  ESC to return"))

	return content.String()
}

// Render volume control screen
func (m Model) renderAlarmVolume() string {
	var content strings.Builder

	title := fmt.Sprintf("🔊 VOLUME FOR ALARM %d", m.app.editingAlarm)
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	var alarm *config.Alarm
	if m.app.editingAlarm == 1 {
		alarm = &m.app.config.Alarm1
	} else {
		alarm = &m.app.config.Alarm2
	}

	content.WriteString(fmt.Sprintf("Current Volume: %d%%\n\n", alarm.Volume))

	// Volume bar
	barWidth := 20
	filledBars := int(float64(alarm.Volume) / 100.0 * float64(barWidth))
	volumeBar := strings.Repeat("█", filledBars) + strings.Repeat("░", barWidth-filledBars)
	content.WriteString(fmt.Sprintf("[%s] %d%%\n\n", volumeBar, alarm.Volume))

	content.WriteString(m.app.instructionStyle.Render("←→ to adjust volume  •  ESC to return"))

	return content.String()
}

// Render tone selection screen
func (m Model) renderAlarmToneSelect() string {
	var content strings.Builder

	title := fmt.Sprintf("🎵 SELECT TONE FOR ALARM %d", m.app.editingAlarm)
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	for i, tone := range m.app.availableTones {
		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s ", tone)))
		} else {
			content.WriteString(fmt.Sprintf("   %s", tone))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  ENTER to select  •  ESC to return"))

	return content.String()
}

// Render custom path input screen
func (m Model) renderAlarmCustomPath() string {
	var content strings.Builder

	var alarm *config.Alarm
	if m.app.editingAlarm == 1 {
		alarm = &m.app.config.Alarm1
	} else {
		alarm = &m.app.config.Alarm2
	}

	var title, prompt, example string
	if alarm.Source == config.SourceMP3 {
		title = fmt.Sprintf("🎵 MP3 PATH FOR ALARM %d", m.app.editingAlarm)
		prompt = "Enter MP3 file or directory path:"
		example = "Examples: /home/user/music/alarm.mp3, /home/user/music/"
	} else if alarm.Source == config.SourceRadio {
		title = fmt.Sprintf("📻 RADIO URL FOR ALARM %d", m.app.editingAlarm)
		prompt = "Enter radio stream URL:"
		example = "Examples: http://stream.com/radio.m3u, https://radio.com/stream"
	}

	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")
	content.WriteString(prompt + "\n")
	content.WriteString(example + "\n\n")

	inputDisplay := m.app.customPathInput
	if len(inputDisplay) == 0 {
		if alarm.Source == config.SourceMP3 {
			inputDisplay = "/path/to/music/"
		} else {
			inputDisplay = "http://radio.url"
		}
	}

	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1).
		Render(fmt.Sprintf(" %s_ ", inputDisplay)))

	content.WriteString("\n\n")
	content.WriteString(m.app.instructionStyle.Render("Type path/URL  •  ENTER to save  •  ESC to cancel"))

	return content.String()
}

// Render sleep timer duration control screen
func (m Model) renderSleepDuration() string {
	var content strings.Builder

	title := "⏱️ DURATION FOR SLEEP TIMER"
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	sleepTimer := &m.app.config.SleepTimer

	content.WriteString(fmt.Sprintf("Current Duration: %d minutes\n\n", sleepTimer.Duration))

	// Duration bar (5-120 minutes range)
	barWidth := 20
	// Calculate percentage: (current - min) / (max - min)
	percentage := float64(sleepTimer.Duration-5) / float64(120-5)
	filledBars := int(percentage * float64(barWidth))
	durationBar := strings.Repeat("█", filledBars) + strings.Repeat("░", barWidth-filledBars)
	content.WriteString(fmt.Sprintf("[%s] %d min\n\n", durationBar, sleepTimer.Duration))

	content.WriteString(m.app.instructionStyle.Render("←→ to adjust duration (5-120 min)  •  ESC to return"))

	return content.String()
}

// Render sleep timer volume control screen
func (m Model) renderSleepVolume() string {
	var content strings.Builder

	title := "🔊 VOLUME FOR SLEEP TIMER"
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	sleepTimer := &m.app.config.SleepTimer

	content.WriteString(fmt.Sprintf("Current Volume: %d%%\n\n", sleepTimer.Volume))

	// Volume bar
	barWidth := 20
	filledBars := int(float64(sleepTimer.Volume) / 100.0 * float64(barWidth))
	volumeBar := strings.Repeat("█", filledBars) + strings.Repeat("░", barWidth-filledBars)
	content.WriteString(fmt.Sprintf("[%s] %d%%\n\n", volumeBar, sleepTimer.Volume))

	content.WriteString(m.app.instructionStyle.Render("←→ to adjust volume  •  ESC to return"))

	return content.String()
}

// Render sleep timer sound selection screen
func (m Model) renderSleepSoundSelect() string {
	var content strings.Builder

	title := "🎵 SELECT SOOTHER SOUND FOR SLEEP TIMER"
	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")

	availableSounds := getAvailableFiles(config.SourceSoother)

	for i, sound := range availableSounds {
		if i == m.app.selectedMenu {
			content.WriteString(m.app.selectedStyle.Render(fmt.Sprintf(" > %s ", sound)))
		} else {
			content.WriteString(fmt.Sprintf("   %s", sound))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.app.instructionStyle.Render("↑↓ to navigate  •  ENTER to select  •  ESC to return"))

	return content.String()
}

// Render sleep timer custom path input screen
func (m Model) renderSleepCustomPath() string {
	var content strings.Builder

	sleepTimer := &m.app.config.SleepTimer

	var title, prompt, example string
	if sleepTimer.Source == config.SourceMP3 {
		title = "🎵 MP3 PATH FOR SLEEP TIMER"
		prompt = "Enter MP3 file or directory path:"
		example = "Examples: /home/user/music/sleep.mp3, /home/user/music/"
	} else if sleepTimer.Source == config.SourceRadio {
		title = "📻 RADIO URL FOR SLEEP TIMER"
		prompt = "Enter radio stream URL:"
		example = "Examples: http://stream.com/radio.m3u, https://radio.com/stream"
	}

	content.WriteString(m.app.titleStyle.Render(title))
	content.WriteString("\n\n")
	content.WriteString(prompt + "\n")
	content.WriteString(example + "\n\n")

	inputDisplay := m.app.customPathInput
	if len(inputDisplay) == 0 {
		if sleepTimer.Source == config.SourceMP3 {
			inputDisplay = "/path/to/music/"
		} else {
			inputDisplay = "http://radio.url"
		}
	}

	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#333333")).
		Padding(0, 1).
		Render(fmt.Sprintf(" %s_ ", inputDisplay)))

	content.WriteString("\n\n")
	content.WriteString(m.app.instructionStyle.Render("Type path/URL  •  ENTER to save  •  ESC to cancel"))

	return content.String()
}

// Run starts the application
func (app *App) Run() error {
	_, err := app.program.Run()
	return err
}

// Stop stops the application
func (app *App) Stop() {
	if app.program != nil {
		app.program.Kill()
	}
}

// Helper functions
func getBoolText(value bool) string {
	if value {
		return "ON"
	}
	return "OFF"
}

// GetProgram returns the tea program for compatibility
func (app *App) GetProgram() *tea.Program {
	return app.program
}

// SetFocus sets the focus to the specified alarm when it becomes active
func (app *App) SetFocus(alarmID int) {
	if app.state == StateMainClock {
		// Set selectedMenu to the alarm button (ALARM 1 = index 1, ALARM 2 = index 2)
		app.selectedMenu = alarmID
	}
}

// discoverToneFiles scans for available .tone files in the buzzer directory
func discoverToneFiles() []string {
	toneDir := "include/sounds/buzzer"
	var tones []string

	files, err := os.ReadDir(toneDir)
	if err != nil {
		// Return default if directory doesn't exist
		return []string{"pattern1.tone", "pattern2.tone", "pattern3.tone", "pattern4.tone", "pattern5.tone"}
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".tone") {
			tones = append(tones, file.Name())
		}
	}

	if len(tones) == 0 {
		// Return default if no files found
		return []string{"pattern1.tone", "pattern2.tone", "pattern3.tone", "pattern4.tone", "pattern5.tone"}
	}

	return tones
}
