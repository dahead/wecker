package main

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/oto/v2"
)

func main() {
	ctx, ready, err := oto.NewContext(44100, 1, 2)
	if err != nil {
		panic(err)
	}
	<-ready

	fmt.Println("Wähle Pattern (1-5):")
	var pattern int
	fmt.Scanln(&pattern)

	switch pattern {
	case 1:
		pattern1(ctx)
	case 2:
		pattern2(ctx)
	case 3:
		pattern3(ctx)
	case 4:
		pattern4(ctx)
	case 5:
		pattern5(ctx)
	default:
		fmt.Println("Ungültiges Pattern")
	}
}

// Pattern 1: Kontinuierlicher 499Hz Ton
func pattern1(ctx *oto.Context) {
	fmt.Println("Pattern 1: Kontinuierlicher Alarm")
	for {
		playTone_one(ctx, 499, 130*time.Millisecond)
		time.Sleep(100 * time.Millisecond)
	}
}

// Pattern 2: Eskalierender Alarm (dein Original)
func pattern2(ctx *oto.Context) {
	fmt.Println("Pattern 2: Eskalierender Alarm")
	playPattern(ctx, 5, NOTE_A4, 200*time.Millisecond, 2*time.Second)
	playPattern(ctx, 10, NOTE_A4, 200*time.Millisecond, 1*time.Second)
	playPattern(ctx, 20, NOTE_A4, 200*time.Millisecond, 500*time.Millisecond)
	playPattern(ctx, 35, NOTE_A4, 200*time.Millisecond, 150*time.Millisecond)
}

// Pattern 3: Sirene (hoch-tief wechselnd)
func pattern3(ctx *oto.Context) {
	fmt.Println("Pattern 3: Sirenen-Alarm")
	for i := 0; i < 20; i++ {
		playTone_one(ctx, NOTE_E6, 300*time.Millisecond)
		playTone_one(ctx, NOTE_C5, 300*time.Millisecond)
	}
}

// Pattern 4: Dreifach-Beep mit Pause
func pattern4(ctx *oto.Context) {
	fmt.Println("Pattern 4: Dreifach-Beep Alarm")
	for i := 0; i < 10; i++ {
		for j := 0; j < 3; j++ {
			playTone_one(ctx, NOTE_A5, 150*time.Millisecond)
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(1 * time.Second)
	}
}

// Pattern 5: Melodischer Wecker (aufsteigende Tonfolge)
func pattern5(ctx *oto.Context) {
	fmt.Println("Pattern 5: Melodischer Wecker")
	notes := []float64{NOTE_C5, NOTE_D5, NOTE_E5, NOTE_F5, NOTE_G5, NOTE_A5}

	for i := 0; i < 5; i++ {
		for _, note := range notes {
			playTone_one(ctx, note, 250*time.Millisecond)
			time.Sleep(50 * time.Millisecond)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func playPattern(ctx *oto.Context, count int, freq float64, duration, delay time.Duration) {
	for i := 0; i < count; i++ {
		playTone_one(ctx, freq, duration)
		time.Sleep(delay)
	}
}

func playTone_one(ctx *oto.Context, frequency float64, duration time.Duration) {
	sampleRate := 44100
	samples := int(float64(sampleRate) * duration.Seconds())
	data := make([]byte, samples*2)

	for i := 0; i < samples; i++ {
		sample := math.Sin(2 * math.Pi * frequency * float64(i) / float64(sampleRate))
		value := int16(sample * 32767 * 0.15) // 15% Lautstärke

		data[i*2] = byte(value)
		data[i*2+1] = byte(value >> 8)
	}

	player := ctx.NewPlayer(bytes.NewReader(data))
	player.Play()
	time.Sleep(duration)
	player.Close()
}
