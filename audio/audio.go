package audio

import (
	"fmt"
	"os/exec"
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
	case config.SourceBuzzer, config.SourceSoother:
		// For buzzer and soother, AlarmSourceValue contains the .tone file path
		audioPath = alarm.AlarmSourceValue
		if audioPath == "" {
			// Default fallback
			if alarm.Source == config.SourceBuzzer {
				audioPath = "include/sounds/buzzer/pattern1.tone"
			} else {
				audioPath = "include/sounds/soother/noise1.tone"
			}
		}
	case config.SourceMP3:
		// For MP3, AlarmSourceValue contains directory path
		audioPath = alarm.AlarmSourceValue
	case config.SourceRadio:
		// For radio, AlarmSourceValue contains URL or M3U playlist path
		audioPath = alarm.AlarmSourceValue
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
		// Default to first soother file
		audioPath = "include/sounds/soother/noise1.tone"
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
