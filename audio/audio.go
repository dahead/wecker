package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
	"wecker/config"
)

// Player manages audio playback
type Player struct {
	currentProcess *exec.Cmd
	mutex          sync.Mutex
	config         *config.Config
	isPlaying      bool
	currentVolume  int
	volumeRamp     bool
	startTime      time.Time
}

// NewPlayer creates a new audio player
func NewPlayer(cfg *config.Config) *Player {
	return &Player{
		config: cfg,
	}
}

// PlayAlarm plays an alarm sound based on the alarm configuration
func (p *Player) PlayAlarm(alarm *config.Alarm) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop any currently playing audio
	p.stopInternal()

	var audioPath string
	var err error

	switch alarm.Source {
	case config.SourceBuzzer:
		audioPath, err = p.getBuzzerPath(alarm.BuzzerType)
	case config.SourceMP3:
		audioPath = alarm.MP3Directory
	case config.SourceRadio:
		audioPath = alarm.RadioURL
	case config.SourceSoother:
		audioPath, err = p.getSootherPath(alarm.SootherType)
	default:
		return fmt.Errorf("unknown alarm source: %s", alarm.Source)
	}

	if err != nil {
		return fmt.Errorf("failed to get audio path: %v", err)
	}

	// Start playback
	return p.startPlayback(audioPath, alarm.Volume, alarm.VolumeRamp)
}

// PlaySleepAudio plays audio for sleep timer (last used source)
func (p *Player) PlaySleepAudio() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop any currently playing audio
	p.stopInternal()

	var audioPath string

	// Use last played source (priority: radio > mp3 > soother)
	if p.config.LastRadioURL != "" {
		audioPath = p.config.LastRadioURL
	} else if p.config.LastMP3Path != "" {
		audioPath = p.config.LastMP3Path
	} else {
		var err error
		audioPath, err = p.getSootherPath(p.config.LastSootherType)
		if err != nil {
			return fmt.Errorf("failed to get soother path: %v", err)
		}
	}

	// Start playback with default volume
	return p.startPlayback(audioPath, 50, false)
}

// startPlayback starts audio playback with the specified parameters
func (p *Player) startPlayback(audioPath string, volume int, rampVolume bool) error {
	if audioPath == "" {
		return fmt.Errorf("empty audio path")
	}

	// Prepare command arguments
	args := []string{audioPath}

	// Add volume control if supported
	if volume > 0 && volume <= 100 {
		volumePercent := volume
		if rampVolume {
			// Start with lower volume for ramping
			volumePercent = max(1, volume/4)
		}
		args = append(args, "--volume", fmt.Sprintf("%d", volumePercent))
	}

	// Add loop flag for continuous playback
	args = append(args, "--loop")

	// Create and start process
	p.currentProcess = exec.Command(p.config.PlayerCommand, args...)

	// Redirect output to avoid cluttering terminal
	p.currentProcess.Stdout = nil
	p.currentProcess.Stderr = nil

	err := p.currentProcess.Start()
	if err != nil {
		return fmt.Errorf("failed to start audio player: %v", err)
	}

	p.isPlaying = true
	p.currentVolume = volume
	p.volumeRamp = rampVolume
	p.startTime = time.Now()

	// Start volume ramping if enabled
	if rampVolume {
		go p.volumeRampLoop(volume)
	}

	return nil
}

// volumeRampLoop gradually increases volume over time
func (p *Player) volumeRampLoop(targetVolume int) {
	if !p.volumeRamp || targetVolume <= 0 {
		return
	}

	ticker := time.NewTicker(30 * time.Second) // Increase every 30 seconds
	defer ticker.Stop()

	startVolume := max(1, targetVolume/4)
	currentVol := startVolume

	for p.isPlaying && currentVol < targetVolume {
		select {
		case <-ticker.C:
			currentVol = min(targetVolume, currentVol+10)
			p.setVolume(currentVol)
		}
	}
}

// setVolume adjusts the current playback volume
func (p *Player) setVolume(volume int) {
	// Note: This is a simplified implementation
	// In practice, you might need to use a different approach
	// depending on the player command capabilities
	p.currentVolume = volume
}

// Stop stops the current audio playback
func (p *Player) Stop() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.stopInternal()
}

// stopInternal stops playback (internal, assumes mutex is held)
func (p *Player) stopInternal() {
	if p.currentProcess != nil {
		p.currentProcess.Process.Kill()
		p.currentProcess.Wait()
		p.currentProcess = nil
	}
	p.isPlaying = false
	p.volumeRamp = false
}

// IsPlaying returns whether audio is currently playing
func (p *Player) IsPlaying() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.isPlaying
}

// getBuzzerPath returns the path to a buzzer sound file
func (p *Player) getBuzzerPath(buzzerType int) (string, error) {
	if buzzerType < 1 || buzzerType > 5 {
		return "", fmt.Errorf("invalid buzzer type: %d (must be 1-5)", buzzerType)
	}

	buzzerFile := fmt.Sprintf("buzzer_%d.mp3", buzzerType)
	buzzerPath := filepath.Join("include", "sounds", "buzzer", buzzerFile)

	// Check if file exists
	if _, err := os.Stat(buzzerPath); os.IsNotExist(err) {
		return "", fmt.Errorf("buzzer file not found: %s", buzzerPath)
	}

	// Return absolute path
	absPath, err := filepath.Abs(buzzerPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	return absPath, nil
}

// getSootherPath returns the path to a sound soother file
func (p *Player) getSootherPath(sootherType int) (string, error) {
	if sootherType < 1 || sootherType > 27 {
		return "", fmt.Errorf("invalid soother type: %d (must be 1-27)", sootherType)
	}

	sootherFile := fmt.Sprintf("soother_%02d.mp3", sootherType)
	sootherPath := filepath.Join("include", "sounds", "soother", sootherFile)

	// Check if file exists
	if _, err := os.Stat(sootherPath); os.IsNotExist(err) {
		return "", fmt.Errorf("soother file not found: %s", sootherPath)
	}

	// Return absolute path
	absPath, err := filepath.Abs(sootherPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}

	return absPath, nil
}

// UpdateConfig updates the configuration reference
func (p *Player) UpdateConfig(cfg *config.Config) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.config = cfg
}

// GetCurrentVolume returns the current playback volume
func (p *Player) GetCurrentVolume() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentVolume
}

// SetVolume sets the current playback volume
func (p *Player) SetVolume(volume int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}

	p.setVolume(volume)
}
