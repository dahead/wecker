package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wecker/config"
	"wecker/tone"
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
	buzzerFiles    []string
	sootherFiles   []string
}

// NewPlayer creates a new audio player
func NewPlayer(cfg *config.Config) *Player {
	p := &Player{
		config: cfg,
	}
	p.discoverToneFiles()
	return p
}

// discoverToneFiles finds all .tone files in buzzer and soother directories
func (p *Player) discoverToneFiles() {
	buzzerDir := "include/sounds/buzzer"
	sootherDir := "include/sounds/soother"

	p.buzzerFiles = findToneFiles(buzzerDir)
	p.sootherFiles = findToneFiles(sootherDir)
}

// findToneFiles scans a directory for .tone files
func findToneFiles(dir string) []string {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue even if there's an error
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".tone") {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return []string{} // Return empty slice on error
	}

	return files
}

// PlayAlarm plays an alarm sound based on the alarm configuration
func (p *Player) PlayAlarm(alarm *config.Alarm) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop any currently playing audio
	p.stopInternal()

	switch alarm.Source {
	case config.SourceBuzzer:
		// Use ToneParser for buzzer sounds
		var toneFile string
		if alarm.AlarmSourceValue != "" {
			toneFile = alarm.AlarmSourceValue
		} else {
			// Select from discovered files
			if len(p.buzzerFiles) > 0 {
				toneFile = p.buzzerFiles[0] // Use first available file
			} else {
				return fmt.Errorf("no buzzer .tone files found")
			}
		}

		p.isPlaying = true
		p.currentVolume = alarm.Volume
		p.startTime = time.Now()

		// Play tone file in a goroutine to avoid blocking
		go func() {
			tone.PlayToneFile(toneFile)
		}()

		return nil

	case config.SourceSoother:
		// Use ToneParser for soother sounds
		var toneFile string
		if alarm.AlarmSourceValue != "" {
			toneFile = alarm.AlarmSourceValue
		} else {
			// Select from discovered files
			if len(p.sootherFiles) > 0 {
				toneFile = p.sootherFiles[0] // Use first available file
			} else {
				return fmt.Errorf("no soother .tone files found")
			}
		}

		p.isPlaying = true
		p.currentVolume = alarm.Volume
		p.startTime = time.Now()

		// Play tone file in a goroutine to avoid blocking
		go func() {
			tone.PlayToneFile(toneFile)
		}()

		return nil

	case config.SourceMP3:
		// Use PlayerCommand for MP3
		audioPath := alarm.AlarmSourceValue
		if audioPath == "" {
			audioPath = p.config.LastMP3Path
		}
		return p.startPlayback(audioPath, alarm.Volume, alarm.VolumeRamp)

	case config.SourceRadio:
		// Use PlayerCommand for radio
		audioPath := alarm.AlarmSourceValue
		if audioPath == "" {
			audioPath = p.config.LastRadioURL
		}
		return p.startPlayback(audioPath, alarm.Volume, alarm.VolumeRamp)

	default:
		return fmt.Errorf("unknown alarm source: %s", alarm.Source)
	}
}

// PlaySleepAudio plays audio for sleep timer (last used source)
func (p *Player) PlaySleepAudio() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stop any currently playing audio
	p.stopInternal()

	// Use last played source (priority: radio > mp3 > soother)
	if p.config.LastRadioURL != "" {
		return p.startPlayback(p.config.LastRadioURL, 50, false)
	} else if p.config.LastMP3Path != "" {
		return p.startPlayback(p.config.LastMP3Path, 50, false)
	} else {
		// Default to first soother .tone file using ToneParser
		if len(p.sootherFiles) > 0 {
			p.isPlaying = true
			p.currentVolume = 50
			p.startTime = time.Now()

			// Play tone file in a goroutine to avoid blocking
			go func() {
				tone.PlayToneFile(p.sootherFiles[0])
			}()

			return nil
		} else {
			return fmt.Errorf("no soother .tone files found")
		}
	}
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
