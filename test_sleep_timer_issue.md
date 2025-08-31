# Sleep Timer Issue Reproduction

## Current Problem
- When moving the sleep timer duration slider left/right, sounds start playing immediately
- Multiple slider movements cause multiple simultaneous sounds
- User wants sounds to play ONLY when leaving the sleep timer settings window

## Code Flow Analysis
1. User moves slider left/right in sleep timer settings
2. handleLeft() or handleRight() functions are called (display.go lines 726-743, 779-796)
3. These functions call StartSleepTimer() immediately on slider movement
4. StartSleepTimer() triggers OnTimerStarted callback (timer.go line 123)
5. OnTimerStarted callback calls PlaySleepAudio() (main.go line 68)
6. PlaySleepAudio() starts playing sounds continuously (audio.go lines 148-209)

## Solution Needed
- Remove StartSleepTimer() calls from handleLeft/handleRight during slider movement
- Only adjust the duration value in the config
- Start the timer only when user exits the sleep timer settings window