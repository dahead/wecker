package display

import (
	"fmt"
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

// MenuState represents the current menu state
type MenuState int

const (
	StateTime MenuState = iota
	StateMainMenu
	StateAlarm1Menu
	StateAlarm2Menu
	StateBrightnessMenu
	StateBacklightMenu
	StateTimeFormatMenu
	StateSecondsMenu
)

// MainMenuItem represents items in the main menu
type MainMenuItem int

const (
	MenuItemTime MainMenuItem = iota
	MenuItemAlarm1
	MenuItemAlarm2
	MenuItemBrightness
	MenuItemBacklight
	MenuItemTimeFormat
	MenuItemSeconds
)

// AlarmMenuItem represents items in alarm configuration menu
type AlarmMenuItem int

const (
	AlarmMenuEnabled AlarmMenuItem = iota
	AlarmMenuTime
	AlarmMenuDays
	AlarmMenuSource
	AlarmMenuVolume
	AlarmMenuBack
)

// App holds the main TUI application
type App struct {
	program      *tea.Program
	config       *config.Config
	currentTime  time.Time
	brightness   int
	isActive     bool
	menuState    MenuState
	selectedItem int
	inMenu       bool
}

// Model represents the bubbletea model
type Model struct {
	app          *App
	alarmManager *alarm.Manager
	timerManager *timer.Manager
	audioPlayer  *audio.Player
}

// TickMsg is sent every second to update the clock
type TickMsg time.Time

// NewApp creates a new display application
func NewApp(cfg *config.Config, alarmMgr *alarm.Manager, timerMgr *timer.Manager, audioPlayer *audio.Player) *App {
	app := &App{
		config:       cfg,
		brightness:   cfg.Brightness,
		isActive:     true,
		menuState:    StateTime,
		selectedItem: 0,
		inMenu:       false,
	}

	model := Model{
		app:          app,
		alarmManager: alarmMgr,
		timerManager: timerMgr,
		audioPlayer:  audioPlayer,
	}
	app.program = tea.NewProgram(model, tea.WithAltScreen())

	return app
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
	)
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		m.app.currentTime = time.Time(msg)
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit
		case "p", "P":
			// Power/Sleep button
			m.handlePowerButton()
		case "s", "S":
			// Snooze button
			m.handleSnoozeButton()
		case "m", "M", "i", "I":
			// Info/Menu button
			m.handleMenuButton()
		case "up":
			// Up button
			m.handleUpButton()
		case "down":
			// Down button
			m.handleDownButton()
		case "enter", " ":
			// Select button
			m.handleSelectButton()
		case "d", "D":
			// Dimmer button
			m.handleDimmerButton()
		case "h", "H":
			// Sleep timer hold
			m.handleSleepTimerHold()
		}
	}
	return m, nil
}

// View implements tea.Model
func (m Model) View() string {
	if m.app.currentTime.IsZero() {
		m.app.currentTime = time.Now()
	}

	// Create header with controls repositioned to top
	header := "    POWER / SLEEP                             SNOOZE                            INFO / MENU\n" +
		strings.Repeat("=", 93) + "\n" +
		"==                                        UP / DOWN                                     SELECT\n" +
		"=="

	// Display menu or time based on current state
	var content string
	if m.app.inMenu {
		content = m.app.renderMenu()
	} else {
		content = m.app.renderTimeDisplay()
	}

	// Create footer
	footer := "==\n" +
		strings.Repeat("=", 93)

	// Combine all sections
	fullContent := fmt.Sprintf("%s\n%s\n%s", header, content, footer)

	// Apply brightness styling
	style := m.app.getBrightnessStyle()
	return style.Render(fullContent)
}

// renderTimeDisplay renders the centered time display without side controls
func (app *App) renderTimeDisplay() string {
	// Create status section
	status := app.getStatusText()

	// Create clock section with ASCII art
	timeStr := app.config.FormatTime(app.currentTime)
	asciiTime := app.generateASCIIClock(timeStr)

	// Center the time without side controls
	lines := strings.Split(asciiTime, "\n")
	centeredLines := make([]string, len(lines))
	for i, line := range lines {
		// Center each line by padding with spaces
		padding := (93 - len(line)) / 2
		if padding > 0 {
			centeredLines[i] = strings.Repeat(" ", padding) + line
		} else {
			centeredLines[i] = line
		}
	}
	centeredClock := strings.Join(centeredLines, "\n")

	return fmt.Sprintf("%s\n%s", status, centeredClock)
}

// renderMenu renders the current menu based on menu state
func (app *App) renderMenu() string {
	switch app.menuState {
	case StateMainMenu:
		return app.renderMainMenu()
	case StateAlarm1Menu:
		return app.renderAlarmMenu(&app.config.Alarm1, "ALARM 1")
	case StateAlarm2Menu:
		return app.renderAlarmMenu(&app.config.Alarm2, "ALARM 2")
	default:
		return app.renderMainMenu()
	}
}

// renderMainMenu renders the main menu with ASCII art
func (app *App) renderMainMenu() string {
	menuItems := []string{
		"TIME",
		"ALARM 1",
		"ALARM 2",
		"BRIGHTNESS",
		"BACKLIGHT",
		"12/24 HOURS",
		"SECONDS",
	}

	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==                                    *** MENU ***")
	menuLines = append(menuLines, "==")

	for i, item := range menuItems {
		prefix := "==  "
		if i == app.selectedItem {
			// Highlight selected item
			prefix = "==► "
		}

		// Generate ASCII art for the menu item
		ascii := figure.NewFigure(item, "small", true)
		asciiLines := strings.Split(strings.TrimSpace(ascii.String()), "\n")

		for j, line := range asciiLines {
			if j == 0 {
				menuLines = append(menuLines, prefix+line)
			} else {
				menuLines = append(menuLines, "==  "+line)
			}
		}
		menuLines = append(menuLines, "==")
	}

	return strings.Join(menuLines, "\n")
}

// renderAlarmMenu renders alarm configuration menu
func (app *App) renderAlarmMenu(alarm *config.Alarm, title string) string {
	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, fmt.Sprintf("==                                  *** %s ***", title))
	menuLines = append(menuLines, "==")

	// Menu items with current values
	items := []string{
		fmt.Sprintf("ENABLED: %s", getBoolText(alarm.Enabled)),
		fmt.Sprintf("TIME: %s", alarm.Time),
		fmt.Sprintf("DAYS: %d/7", app.countActiveDays(alarm.Days)),
		fmt.Sprintf("SOURCE: %s", strings.ToUpper(string(alarm.Source))),
		fmt.Sprintf("VOLUME: %d", alarm.Volume),
		"BACK",
	}

	for i, item := range items {
		prefix := "==  "
		if i == app.selectedItem {
			prefix = "==► "
		}
		menuLines = append(menuLines, prefix+item)
		menuLines = append(menuLines, "==")
	}

	return strings.Join(menuLines, "\n")
}

// getBoolText returns text representation of boolean
func getBoolText(value bool) string {
	if value {
		return "ON"
	}
	return "OFF"
}

// getMaxMenuItems returns the number of items in the current menu
func (app *App) getMaxMenuItems() int {
	switch app.menuState {
	case StateMainMenu:
		return 7 // TIME, ALARM 1, ALARM 2, BRIGHTNESS, BACKLIGHT, 12/24 HOURS, SECONDS
	case StateAlarm1Menu, StateAlarm2Menu:
		return 6 // ENABLED, TIME, DAYS, SOURCE, VOLUME, BACK
	default:
		return 7
	}
}

// Run starts the TUI application
func (app *App) Run() error {
	_, err := app.program.Run()
	return err
}

// Stop stops the TUI application
func (app *App) Stop() {
	app.isActive = false
	if app.program != nil {
		app.program.Quit()
	}
}

// getStatusText generates the status area text
func (app *App) getStatusText() string {
	alarm1Status := "OFF"
	if app.config.Alarm1.Enabled {
		alarm1Status = "ON"
	}

	alarm2Status := "OFF"
	if app.config.Alarm2.Enabled {
		alarm2Status = "ON"
	}

	// Format days for display (show count of active days)
	alarm1Days := app.countActiveDays(app.config.Alarm1.Days)
	alarm2Days := app.countActiveDays(app.config.Alarm2.Days)

	statusText := fmt.Sprintf(`==                                                                                Alarms: %s%s
==
==  Alarm 1: %s (%d/7)     %s
==  Alarm 2: %s (%d/7)     %s`,
		getBoolIcon(app.config.Alarm1.Enabled),
		getBoolIcon(app.config.Alarm2.Enabled),
		app.config.Alarm1.Time, alarm1Days, alarm1Status,
		app.config.Alarm2.Time, alarm2Days, alarm2Status)

	return statusText
}

// generateASCIIClock creates ASCII art for the current time using go-figure
func (app *App) generateASCIIClock(timeStr string) string {
	// Use go-figure to render the time string as ASCII art
	ascii := figure.NewFigure(timeStr, "", true)
	return ascii.String()
}

// addSideControls adds UP/DOWN control text to the right side
func (app *App) addSideControls(clockText string) string {
	lines := strings.Split(clockText, "\n")
	if len(lines) >= 6 {
		// Add UP/DOWN text to the right side of the 4th line (index 3)
		lines[3] += "                    UP / DOWN"
	}
	return strings.Join(lines, "\n")
}

// countActiveDays counts how many days are enabled for an alarm
func (app *App) countActiveDays(days []bool) int {
	count := 0
	for _, day := range days {
		if day {
			count++
		}
	}
	return count
}

// getBoolIcon returns an icon character for boolean status
func getBoolIcon(enabled bool) string {
	if enabled {
		return "●"
	}
	return "○"
}

// SetBrightness adjusts display brightness
func (app *App) SetBrightness(level int) {
	if level < 1 {
		level = 1
	}
	if level > 10 {
		level = 10
	}
	app.brightness = level
	app.config.Brightness = level
}

// getBrightnessStyle returns a lipgloss style based on brightness level
func (app *App) getBrightnessStyle() lipgloss.Style {
	var color lipgloss.Color
	switch {
	case app.brightness <= 2:
		color = lipgloss.Color("#444444")
	case app.brightness <= 4:
		color = lipgloss.Color("#666666")
	case app.brightness <= 6:
		color = lipgloss.Color("#888888")
	case app.brightness <= 8:
		color = lipgloss.Color("#AAAAAA")
	default:
		color = lipgloss.Color("#FFFFFF")
	}

	return lipgloss.NewStyle().
		Foreground(color).
		Background(lipgloss.Color("#000000"))
}

// GetProgram returns the underlying bubbletea program for input handling
func (app *App) GetProgram() *tea.Program {
	return app.program
}

// Handler methods for input events
func (m *Model) handlePowerButton() {
	// Basic power/sleep functionality - can be expanded
}

func (m *Model) handleSnoozeButton() {
	// Basic snooze functionality - can be expanded
}

func (m *Model) handleMenuButton() {
	// Toggle between time display and menu
	m.app.inMenu = !m.app.inMenu
	if m.app.inMenu {
		m.app.menuState = StateMainMenu
		m.app.selectedItem = 0
	} else {
		m.app.menuState = StateTime
	}
}

func (m *Model) handleUpButton() {
	if m.app.inMenu {
		// Navigate up in menu
		maxItems := m.app.getMaxMenuItems()
		if m.app.selectedItem > 0 {
			m.app.selectedItem--
		} else {
			// Wrap to bottom
			m.app.selectedItem = maxItems - 1
		}
	} else {
		// Brightness up when not in menu
		if m.app.brightness < 10 {
			m.app.SetBrightness(m.app.brightness + 1)
		}
	}
}

func (m *Model) handleDownButton() {
	if m.app.inMenu {
		// Navigate down in menu
		maxItems := m.app.getMaxMenuItems()
		if m.app.selectedItem < maxItems-1 {
			m.app.selectedItem++
		} else {
			// Wrap to top
			m.app.selectedItem = 0
		}
	} else {
		// Brightness down when not in menu
		if m.app.brightness > 1 {
			m.app.SetBrightness(m.app.brightness - 1)
		}
	}
}

func (m *Model) handleSelectButton() {
	if !m.app.inMenu {
		return
	}

	switch m.app.menuState {
	case StateMainMenu:
		m.handleMainMenuSelect()
	case StateAlarm1Menu:
		m.handleAlarmMenuSelect(&m.app.config.Alarm1)
	case StateAlarm2Menu:
		m.handleAlarmMenuSelect(&m.app.config.Alarm2)
	}
}

func (m *Model) handleMainMenuSelect() {
	switch m.app.selectedItem {
	case int(MenuItemTime):
		// Return to time display
		m.app.inMenu = false
		m.app.menuState = StateTime
	case int(MenuItemAlarm1):
		// Open alarm 1 sub-menu
		m.app.menuState = StateAlarm1Menu
		m.app.selectedItem = 0
	case int(MenuItemAlarm2):
		// Open alarm 2 sub-menu
		m.app.menuState = StateAlarm2Menu
		m.app.selectedItem = 0
	case int(MenuItemBrightness):
		// Cycle brightness (1-10)
		newLevel := m.app.brightness + 1
		if newLevel > 10 {
			newLevel = 1
		}
		m.app.SetBrightness(newLevel)
	case int(MenuItemBacklight):
		// Cycle backlight (1-10)
		newLevel := m.app.config.Backlight + 1
		if newLevel > 10 {
			newLevel = 1
		}
		m.app.config.Backlight = newLevel
	case int(MenuItemTimeFormat):
		// Toggle 12/24 hour format
		m.app.config.Hour24Format = !m.app.config.Hour24Format
	case int(MenuItemSeconds):
		// Toggle seconds display
		m.app.config.ShowSeconds = !m.app.config.ShowSeconds
	}
}

func (m *Model) handleAlarmMenuSelect(alarm *config.Alarm) {
	switch m.app.selectedItem {
	case int(AlarmMenuEnabled):
		// Toggle alarm enabled state
		alarm.Enabled = !alarm.Enabled
	case int(AlarmMenuTime):
		// TODO: Implement time setting (would need time picker)
		// For now, just cycle through some preset times
		switch alarm.Time {
		case "06:00":
			alarm.Time = "06:30"
		case "06:30":
			alarm.Time = "07:00"
		case "07:00":
			alarm.Time = "07:30"
		case "07:30":
			alarm.Time = "08:00"
		case "08:00":
			alarm.Time = "06:00"
		default:
			alarm.Time = "07:00"
		}
	case int(AlarmMenuDays):
		// TODO: Implement days selection (would need day picker)
		// For now, toggle between weekdays and all days
		if m.app.countActiveDays(alarm.Days) == 5 {
			// Set to all days
			alarm.Days = []bool{true, true, true, true, true, true, true}
		} else {
			// Set to weekdays only
			alarm.Days = []bool{false, true, true, true, true, true, false}
		}
	case int(AlarmMenuSource):
		// Cycle through alarm sources
		switch alarm.Source {
		case config.SourceBuzzer:
			alarm.Source = config.SourceMP3
		case config.SourceMP3:
			alarm.Source = config.SourceRadio
		case config.SourceRadio:
			alarm.Source = config.SourceSoother
		case config.SourceSoother:
			alarm.Source = config.SourceBuzzer
		default:
			alarm.Source = config.SourceBuzzer
		}
	case int(AlarmMenuVolume):
		// Cycle volume (10, 20, 30, 40, 50, 60, 70, 80, 90, 100)
		newVolume := alarm.Volume + 10
		if newVolume > 100 {
			newVolume = 10
		}
		alarm.Volume = newVolume
	case int(AlarmMenuBack):
		// Return to main menu
		m.app.menuState = StateMainMenu
		m.app.selectedItem = 0
	}
}

func (m *Model) handleDimmerButton() {
	// Cycle through brightness levels
	newLevel := m.app.brightness + 1
	if newLevel > 10 {
		newLevel = 1
	}
	m.app.SetBrightness(newLevel)
}

func (m *Model) handleSleepTimerHold() {
	// Basic sleep timer functionality - can be expanded
}
