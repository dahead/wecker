package display

import (
	"fmt"
	"path/filepath"
	"sort"
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
	StateTimeEdit
	StateDaysEdit
	StateSourceEdit
	StateSourceTypeSelect
	StateSourceFileSelect
	StateSourceStringInput
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
	program             *tea.Program
	config              *config.Config
	currentTime         time.Time
	brightness          int
	isActive            bool
	menuState           MenuState
	previousMenuState   MenuState
	selectedItem        int
	inMenu              bool
	timeEditPosition    int
	currentEditingAlarm *config.Alarm
	// Source configuration fields
	availableFiles    []string // Available files for current source type
	selectedFileIndex int      // Index of selected file
	stringInput       string   // String input for mp3/radio paths
	stringEditMode    bool     // Whether we're editing the string input
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
		case "enter":
			// Select button
			m.handleSelectButton()
		case " ":
			// Space - play button for file selection or select for other menus
			if m.app.menuState == StateSourceFileSelect {
				m.handlePlayButton()
			} else {
				m.handleSelectButton()
			}
		case "left":
			// Left navigation (for time editing)
			m.handleLeftButton()
		case "right":
			// Right navigation (for time editing)
			m.handleRightButton()
		case "esc":
			// Escape (cancel editing)
			m.handleEscapeButton()
		case "backspace":
			// Backspace for string editing
			if m.app.menuState == StateSourceStringInput && m.app.stringEditMode && len(m.app.stringInput) > 0 {
				m.app.stringInput = m.app.stringInput[:len(m.app.stringInput)-1]
			}
		case "d", "D":
			// Dimmer button
			m.handleDimmerButton()
		case "h", "H":
			// Sleep timer hold
			m.handleSleepTimerHold()
		default:
			// Handle character input for string editing
			if m.app.menuState == StateSourceStringInput && m.app.stringEditMode {
				if len(msg.String()) == 1 {
					char := msg.String()
					// Allow alphanumeric, /, ., -, _ and space
					if (char >= "a" && char <= "z") || (char >= "A" && char <= "Z") ||
						(char >= "0" && char <= "9") || char == "/" || char == "." ||
						char == "-" || char == "_" || char == " " || char == ":" {
						m.app.stringInput += char
					}
				}
			}
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
	case StateTimeEdit:
		return app.renderTimeEditMenu()
	case StateDaysEdit:
		return app.renderDaysEditMenu()
	case StateSourceEdit:
		return app.renderSourceEditMenu()
	case StateSourceTypeSelect:
		return app.renderSourceTypeSelectMenu()
	case StateSourceFileSelect:
		return app.renderSourceFileSelectMenu()
	case StateSourceStringInput:
		return app.renderSourceStringInputMenu()
	default:
		return app.renderMainMenu()
	}
}

// renderMainMenu renders the main menu without ASCII art
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

	// Create centered menu title with Lipgloss styling
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Width(89).
		Align(lipgloss.Center)

	menuLines = append(menuLines, "==  "+titleStyle.Render("*** MENU ***"))
	menuLines = append(menuLines, "==")

	for i, item := range menuItems {
		itemStyle := lipgloss.NewStyle().
			Width(80).
			PaddingLeft(2)

		if i == app.selectedItem {
			// Highlight selected item with color and arrow
			itemStyle = itemStyle.
				Foreground(lipgloss.Color("11")).
				Bold(true)
			menuLines = append(menuLines, "==► "+itemStyle.Render(item))
		} else {
			itemStyle = itemStyle.
				Foreground(lipgloss.Color("7"))
			menuLines = append(menuLines, "==  "+itemStyle.Render(item))
		}
	}
	menuLines = append(menuLines, "==")

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

// renderTimeEditMenu renders the time editing interface
func (app *App) renderTimeEditMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==                               *** EDIT TIME ***")
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==")

	// Get current alarm being edited
	currentAlarm := app.currentEditingAlarm

	if currentAlarm != nil {
		// Parse current time
		timeStr := currentAlarm.Time
		if len(timeStr) < 8 {
			timeStr = timeStr + ":00" // Add seconds if missing
		}

		// Display editable time format HH:MM:SS with cursor position
		timeParts := strings.Split(timeStr, ":")
		if len(timeParts) == 3 {
			timeDisplay := fmt.Sprintf("    Time: %s:%s:%s", timeParts[0], timeParts[1], timeParts[2])

			// Add cursor indicator based on editing position
			cursorPos := app.timeEditPosition % 3
			switch cursorPos {
			case 0: // Hours
				timeDisplay = fmt.Sprintf("    Time: [%s]:%s:%s", timeParts[0], timeParts[1], timeParts[2])
			case 1: // Minutes
				timeDisplay = fmt.Sprintf("    Time: %s:[%s]:%s", timeParts[0], timeParts[1], timeParts[2])
			case 2: // Seconds
				timeDisplay = fmt.Sprintf("    Time: %s:%s:[%s]", timeParts[0], timeParts[1], timeParts[2])
			}

			menuLines = append(menuLines, "=="+timeDisplay)
		}
	}

	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==    Use LEFT/RIGHT to move cursor, UP/DOWN to change value")
	menuLines = append(menuLines, "==    Press ENTER to confirm, ESC to cancel")
	menuLines = append(menuLines, "==")

	return strings.Join(menuLines, "\n")
}

// renderDaysEditMenu renders the day selection interface
func (app *App) renderDaysEditMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==                              *** SELECT DAYS ***")
	menuLines = append(menuLines, "==")

	// Get current alarm being edited
	currentAlarm := app.currentEditingAlarm

	if currentAlarm != nil {
		dayNames := []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

		for i, dayName := range dayNames {
			checkbox := "[ ]"
			if currentAlarm.Days[i] {
				checkbox = "[X]"
			}

			prefix := "==  "
			if i == app.selectedItem {
				prefix = "==► "
			}

			menuLines = append(menuLines, fmt.Sprintf("%s%s %s", prefix, checkbox, dayName))
		}

		menuLines = append(menuLines, "==")
		menuLines = append(menuLines, "==► BACK")
	}

	menuLines = append(menuLines, "==")
	return strings.Join(menuLines, "\n")
}

// renderSourceEditMenu renders the alarm source configuration interface
func (app *App) renderSourceEditMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==                            *** ALARM SOURCE ***")
	menuLines = append(menuLines, "==")

	// Get current alarm being edited
	currentAlarm := app.currentEditingAlarm

	if currentAlarm != nil {
		sources := []string{"BUZZER", "MP3", "RADIO"}

		for i, source := range sources {
			prefix := "==  "
			if i == app.selectedItem {
				prefix = "==► "
			}

			selected := ""
			if strings.ToUpper(string(currentAlarm.Source)) == source {
				selected = " ◄"
			}

			menuLines = append(menuLines, fmt.Sprintf("%s%s%s", prefix, source, selected))
		}

		menuLines = append(menuLines, "==")

		// Show current source configuration
		valueText := currentAlarm.AlarmSourceValue
		if valueText == "" {
			valueText = "Not set"
		}

		switch currentAlarm.Source {
		case config.SourceBuzzer:
			menuLines = append(menuLines, fmt.Sprintf("==  Buzzer File: %s", valueText))
		case config.SourceSoother:
			menuLines = append(menuLines, fmt.Sprintf("==  Soother File: %s", valueText))
		case config.SourceMP3:
			menuLines = append(menuLines, fmt.Sprintf("==  MP3 Directory: %s", valueText))
		case config.SourceRadio:
			menuLines = append(menuLines, fmt.Sprintf("==  Radio URL/M3U: %s", valueText))
		}

		// Add options to configure the selected source
		menuLines = append(menuLines, "==")
		menuLines = append(menuLines, "==► CONFIGURE SOURCE")

		menuLines = append(menuLines, "==")
		menuLines = append(menuLines, "==► BACK")
	}

	menuLines = append(menuLines, "==")
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
	case StateSourceEdit:
		return 6 // BUZZER, SOOTHER, MP3, RADIO, CONFIGURE SOURCE, BACK
	case StateSourceTypeSelect:
		return 5 // BUZZER, SOOTHER, MP3, RADIO, BACK
	case StateSourceFileSelect:
		return len(app.availableFiles) + 1 // Files + BACK
	case StateSourceStringInput:
		return 2 // SAVE, BACK
	case StateDaysEdit:
		return 8 // 7 days + BACK
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
	ascii := figure.NewFigure(timeStr, "doom", true)
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
		switch m.app.menuState {
		case StateTimeEdit:
			// Increase time value at current cursor position
			m.handleTimeValueChange(1)
		case StateSourceFileSelect:
			// Navigate up in file selection and update selectedFileIndex
			maxItems := m.app.getMaxMenuItems()
			if m.app.selectedItem > 0 {
				m.app.selectedItem--
				if m.app.selectedItem < len(m.app.availableFiles) {
					m.app.selectedFileIndex = m.app.selectedItem
				}
			} else {
				m.app.selectedItem = maxItems - 1
				if m.app.selectedItem < len(m.app.availableFiles) {
					m.app.selectedFileIndex = m.app.selectedItem
				}
			}
		case StateDaysEdit, StateSourceEdit, StateSourceTypeSelect, StateSourceStringInput:
			// Navigate up in editing menus
			maxItems := m.app.getMaxMenuItems()
			if m.app.selectedItem > 0 {
				m.app.selectedItem--
			} else {
				m.app.selectedItem = maxItems - 1
			}
		default:
			// Navigate up in menu
			maxItems := m.app.getMaxMenuItems()
			if m.app.selectedItem > 0 {
				m.app.selectedItem--
			} else {
				// Wrap to bottom
				m.app.selectedItem = maxItems - 1
			}
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
	case StateSourceEdit:
		m.handleSourceEditSelect()
	case StateSourceTypeSelect:
		m.handleSourceTypeSelect()
	case StateSourceFileSelect:
		m.handleSourceFileSelect()
	case StateSourceStringInput:
		m.handleSourceStringInputSelect()
	case StateDaysEdit:
		m.handleDaysEditSelect()
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
		// Enter time editing mode
		m.app.previousMenuState = m.app.menuState
		m.app.menuState = StateTimeEdit
		m.app.timeEditPosition = 0
		m.app.currentEditingAlarm = alarm
		m.app.selectedItem = 0
	case int(AlarmMenuDays):
		// Enter days editing mode
		m.app.previousMenuState = m.app.menuState
		m.app.menuState = StateDaysEdit
		m.app.currentEditingAlarm = alarm
		m.app.selectedItem = 0
	case int(AlarmMenuSource):
		// Enter source editing mode
		m.app.previousMenuState = m.app.menuState
		m.app.menuState = StateSourceEdit
		m.app.currentEditingAlarm = alarm
		m.app.selectedItem = 0
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

func (m *Model) handleLeftButton() {
	if m.app.menuState == StateTimeEdit {
		// Move cursor left in time editing
		if m.app.timeEditPosition > 0 {
			m.app.timeEditPosition--
		}
	}
}

func (m *Model) handleRightButton() {
	if m.app.menuState == StateTimeEdit {
		// Move cursor right in time editing
		if m.app.timeEditPosition < 2 {
			m.app.timeEditPosition++
		}
	}
}

func (m *Model) handleEscapeButton() {
	// Cancel editing and return to previous menu
	switch m.app.menuState {
	case StateTimeEdit, StateDaysEdit, StateSourceEdit:
		m.app.menuState = m.app.previousMenuState
		m.app.selectedItem = 0
		m.app.currentEditingAlarm = nil
	case StateSourceTypeSelect, StateSourceFileSelect:
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.availableFiles = nil
	case StateSourceStringInput:
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.stringInput = ""
		m.app.stringEditMode = false
	case StateMainMenu:
		// Exit menu mode
		m.app.inMenu = false
		m.app.menuState = StateTime
	}
}

func (m *Model) handleTimeValueChange(direction int) {
	if m.app.currentEditingAlarm == nil {
		return
	}

	// Parse current time
	timeStr := m.app.currentEditingAlarm.Time
	if len(timeStr) < 8 {
		timeStr = timeStr + ":00"
	}

	timeParts := strings.Split(timeStr, ":")
	if len(timeParts) != 3 {
		return
	}

	// Convert to integers
	hours, _ := strconv.Atoi(timeParts[0])
	minutes, _ := strconv.Atoi(timeParts[1])
	seconds, _ := strconv.Atoi(timeParts[2])

	// Modify based on cursor position
	switch m.app.timeEditPosition {
	case 0: // Hours
		hours += direction
		if hours < 0 {
			hours = 23
		} else if hours > 23 {
			hours = 0
		}
	case 1: // Minutes
		minutes += direction
		if minutes < 0 {
			minutes = 59
		} else if minutes > 59 {
			minutes = 0
		}
	case 2: // Seconds
		seconds += direction
		if seconds < 0 {
			seconds = 59
		} else if seconds > 59 {
			seconds = 0
		}
	}

	// Update the alarm time
	m.app.currentEditingAlarm.Time = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// getAvailableFiles returns a list of available files for the given source type
func (app *App) getAvailableFiles(source config.AlarmSource) []string {
	var files []string
	var searchDir string

	switch source {
	case config.SourceBuzzer:
		searchDir = "include/sounds/buzzer"
	case config.SourceSoother:
		searchDir = "include/sounds/soother"
	default:
		return files
	}

	matches, err := filepath.Glob(filepath.Join(searchDir, "*.tone"))
	if err != nil {
		return files
	}

	sort.Strings(matches)
	return matches
}

// renderSourceTypeSelectMenu renders the source type selection interface
func (app *App) renderSourceTypeSelectMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==                        *** SELECT ALARM SOURCE TYPE ***")
	menuLines = append(menuLines, "==")

	currentAlarm := app.currentEditingAlarm
	if currentAlarm != nil {
		sources := []config.AlarmSource{config.SourceBuzzer, config.SourceSoother, config.SourceMP3, config.SourceRadio}
		sourceNames := []string{"BUZZER", "SOOTHER", "MP3", "RADIO"}

		for i, source := range sources {
			prefix := "==  "
			if i == app.selectedItem {
				prefix = "==► "
			}

			selected := ""
			if currentAlarm.Source == source {
				selected = " ◄"
			}

			menuLines = append(menuLines, fmt.Sprintf("%s%s%s", prefix, sourceNames[i], selected))
		}

		menuLines = append(menuLines, "==")
		menuLines = append(menuLines, "==► BACK")
	}

	menuLines = append(menuLines, "==")
	return strings.Join(menuLines, "\n")
}

// renderSourceFileSelectMenu renders the file selection interface for buzzer/soother
func (app *App) renderSourceFileSelectMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")

	currentAlarm := app.currentEditingAlarm
	if currentAlarm == nil {
		return strings.Join(menuLines, "\n")
	}

	sourceType := strings.ToUpper(string(currentAlarm.Source))
	menuLines = append(menuLines, fmt.Sprintf("==                        *** SELECT %s FILE ***", sourceType))
	menuLines = append(menuLines, "==")

	if len(app.availableFiles) == 0 {
		app.availableFiles = app.getAvailableFiles(currentAlarm.Source)
		app.selectedFileIndex = 0
	}

	for i, filePath := range app.availableFiles {
		fileName := filepath.Base(filePath)
		prefix := "==  "
		if i == app.selectedFileIndex {
			prefix = "==► "
		}

		selected := ""
		if filePath == currentAlarm.AlarmSourceValue {
			selected = " ◄"
		}

		// Add play button indicator
		playButton := ""
		if i == app.selectedFileIndex {
			playButton = " [PLAY]"
		}

		menuLines = append(menuLines, fmt.Sprintf("%s%s%s%s", prefix, fileName, selected, playButton))
	}

	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==► BACK")
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==  Use UP/DOWN to browse, ENTER to select, SPACE to play")

	return strings.Join(menuLines, "\n")
}

// renderSourceStringInputMenu renders the string input interface for mp3/radio
func (app *App) renderSourceStringInputMenu() string {
	var menuLines []string
	menuLines = append(menuLines, "==")

	currentAlarm := app.currentEditingAlarm
	if currentAlarm == nil {
		return strings.Join(menuLines, "\n")
	}

	sourceType := strings.ToUpper(string(currentAlarm.Source))
	if currentAlarm.Source == config.SourceMP3 {
		menuLines = append(menuLines, "==                    *** ENTER MP3 DIRECTORY PATH ***")
	} else {
		menuLines = append(menuLines, "==                 *** ENTER RADIO URL OR M3U PATH ***")
	}
	menuLines = append(menuLines, "==")

	inputValue := app.stringInput
	if inputValue == "" {
		inputValue = currentAlarm.AlarmSourceValue
	}

	cursor := ""
	if app.stringEditMode {
		cursor = "_"
	}

	menuLines = append(menuLines, fmt.Sprintf("==  %s: %s%s", sourceType, inputValue, cursor))
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==► SAVE")
	menuLines = append(menuLines, "==► BACK")
	menuLines = append(menuLines, "==")
	menuLines = append(menuLines, "==  Type path and press ENTER to save, ESC to cancel")

	return strings.Join(menuLines, "\n")
}

// handleSourceEditSelect handles selection in the source edit menu
func (m *Model) handleSourceEditSelect() {
	currentAlarm := m.app.currentEditingAlarm
	if currentAlarm == nil {
		return
	}

	sources := []config.AlarmSource{config.SourceBuzzer, config.SourceSoother, config.SourceMP3, config.SourceRadio}

	if m.app.selectedItem < len(sources) {
		// Source type selection
		currentAlarm.Source = sources[m.app.selectedItem]
		currentAlarm.AlarmSourceValue = "" // Reset value when changing type
	} else if m.app.selectedItem == len(sources) {
		// CONFIGURE SOURCE option
		switch currentAlarm.Source {
		case config.SourceBuzzer, config.SourceSoother:
			m.app.previousMenuState = m.app.menuState
			m.app.menuState = StateSourceFileSelect
			m.app.availableFiles = m.app.getAvailableFiles(currentAlarm.Source)
			m.app.selectedFileIndex = 0
			m.app.selectedItem = 0
		case config.SourceMP3, config.SourceRadio:
			m.app.previousMenuState = m.app.menuState
			m.app.menuState = StateSourceStringInput
			m.app.stringInput = currentAlarm.AlarmSourceValue
			m.app.stringEditMode = false
			m.app.selectedItem = 0
		}
	} else {
		// BACK option
		m.app.menuState = m.app.previousMenuState
		m.app.selectedItem = 0
		m.app.currentEditingAlarm = nil
	}
}

// handleSourceTypeSelect handles selection in the source type selection menu
func (m *Model) handleSourceTypeSelect() {
	currentAlarm := m.app.currentEditingAlarm
	if currentAlarm == nil {
		return
	}

	sources := []config.AlarmSource{config.SourceBuzzer, config.SourceSoother, config.SourceMP3, config.SourceRadio}

	if m.app.selectedItem < len(sources) {
		// Set the source type
		currentAlarm.Source = sources[m.app.selectedItem]
		currentAlarm.AlarmSourceValue = "" // Reset value when changing type

		// Automatically proceed to configuration
		switch currentAlarm.Source {
		case config.SourceBuzzer, config.SourceSoother:
			m.app.menuState = StateSourceFileSelect
			m.app.availableFiles = m.app.getAvailableFiles(currentAlarm.Source)
			m.app.selectedFileIndex = 0
		case config.SourceMP3, config.SourceRadio:
			m.app.menuState = StateSourceStringInput
			m.app.stringInput = ""
			m.app.stringEditMode = true
		}
		m.app.selectedItem = 0
	} else {
		// BACK option
		m.app.menuState = m.app.previousMenuState
		m.app.selectedItem = 0
	}
}

// handleSourceFileSelect handles selection in the file selection menu
func (m *Model) handleSourceFileSelect() {
	currentAlarm := m.app.currentEditingAlarm
	if currentAlarm == nil {
		return
	}

	if m.app.selectedItem < len(m.app.availableFiles) {
		// File selection - set the selected file
		selectedFile := m.app.availableFiles[m.app.selectedItem]
		currentAlarm.AlarmSourceValue = selectedFile
		m.app.selectedFileIndex = m.app.selectedItem

		// Return to source edit menu
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.availableFiles = nil
	} else {
		// BACK option
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.availableFiles = nil
	}
}

// handleSourceStringInputSelect handles selection in the string input menu
func (m *Model) handleSourceStringInputSelect() {
	currentAlarm := m.app.currentEditingAlarm
	if currentAlarm == nil {
		return
	}

	switch m.app.selectedItem {
	case 0: // SAVE
		if m.app.stringInput != "" {
			currentAlarm.AlarmSourceValue = m.app.stringInput
		}
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.stringInput = ""
		m.app.stringEditMode = false
	case 1: // BACK
		m.app.menuState = StateSourceEdit
		m.app.selectedItem = 0
		m.app.stringInput = ""
		m.app.stringEditMode = false
	}
}

// handleDaysEditSelect handles selection in the days edit menu
func (m *Model) handleDaysEditSelect() {
	currentAlarm := m.app.currentEditingAlarm
	if currentAlarm == nil {
		return
	}

	if m.app.selectedItem < 7 {
		// Toggle day selection
		currentAlarm.Days[m.app.selectedItem] = !currentAlarm.Days[m.app.selectedItem]
	} else {
		// BACK option
		m.app.menuState = m.app.previousMenuState
		m.app.selectedItem = 0
		m.app.currentEditingAlarm = nil
	}
}

// handlePlayButton handles playing the currently selected file
func (m *Model) handlePlayButton() {
	if m.app.menuState != StateSourceFileSelect || m.app.currentEditingAlarm == nil {
		return
	}

	if m.app.selectedItem < len(m.app.availableFiles) {
		selectedFile := m.app.availableFiles[m.app.selectedItem]

		// Create a temporary alarm config for preview
		tempAlarm := *m.app.currentEditingAlarm
		tempAlarm.AlarmSourceValue = selectedFile
		tempAlarm.Volume = 30 // Lower volume for preview

		// Stop any current playback and play the selected file
		m.audioPlayer.Stop()
		if err := m.audioPlayer.PlayAlarm(&tempAlarm); err != nil {
			// Ignore errors for preview - files might not be compatible
		}

		// Stop after 2 seconds for preview
		go func() {
			time.Sleep(2 * time.Second)
			m.audioPlayer.Stop()
		}()
	}
}
