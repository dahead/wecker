package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"wecker/alarm"
	"wecker/audio"
	"wecker/config"
	"wecker/display"
	"wecker/timer"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create managers and components
	alarmManager := alarm.NewManager(cfg)
	timerManager := timer.NewManager()
	audioPlayer := audio.NewPlayer(cfg)
	displayApp := display.NewApp(cfg, alarmManager, timerManager, audioPlayer)

	// Set up callbacks for alarm events
	alarmManager.SetCallbacks(alarm.AlarmCallbacks{
		OnAlarmTriggered: func(alarmID int, alarmCfg *config.Alarm) {
			// Set focus to the triggered alarm
			displayApp.SetFocus(alarmID)

			// Play alarm sound
			if err := audioPlayer.PlayAlarm(alarmCfg); err != nil {
				log.Printf("Failed to play alarm %d: %v", alarmID, err)
				// Fallback to buzzer if configured source fails
				if alarmCfg.Source != config.SourceBuzzer {
					fallbackAlarm := *alarmCfg
					fallbackAlarm.Source = config.SourceBuzzer
					audioPlayer.PlayAlarm(&fallbackAlarm)
				}
			}
		},
		OnAlarmSnoozed: func(alarmID int, duration time.Duration) {
			log.Printf("Alarm %d snoozed for %v", alarmID, duration)
			audioPlayer.Stop()
		},
		OnAlarmStopped: func(alarmID int) {
			log.Printf("Alarm %d stopped", alarmID)
			audioPlayer.Stop()
		},
	})

	// Set up callbacks for timer events
	timerManager.SetCallbacks(timer.TimerCallbacks{
		OnSleepTimerExpired: func() {
			log.Println("Sleep timer expired, stopping audio and resetting timer")
			audioPlayer.Stop()
			// Reset sleep timer to disabled state as required
			cfg.SleepTimer.Enabled = false
		},
		OnSnoozeTimerExpired: func() {
			log.Println("Snooze timer expired")
		},
		OnTimerStarted: func(timerType timer.TimerType, duration time.Duration) {
			if timerType == timer.TypeSleep {
				log.Printf("Sleep timer started for %v", duration)
				// Start playing sleep audio
				if err := audioPlayer.PlaySleepAudio(); err != nil {
					log.Printf("Failed to play sleep audio: %v", err)
				}
			}
		},
		OnTimerStopped: func(timerType timer.TimerType) {
			if timerType == timer.TypeSleep {
				log.Println("Sleep timer stopped")
			}
		},
	})

	// Start managers
	alarmManager.Start()
	timerManager.Start()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")

		// Save configuration
		if err := cfg.Save(); err != nil {
			log.Printf("Failed to save configuration: %v", err)
		}

		// Stop audio
		audioPlayer.Stop()

		// Stop display
		displayApp.Stop()

		os.Exit(0)
	}()

	// Run the TUI application
	if err := displayApp.Run(); err != nil {
		log.Fatalf("Failed to run application: %v", err)
	}
}
