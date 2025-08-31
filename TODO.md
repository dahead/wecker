When snoozing, stop the playing alarm.







saving the alarm settings does not work.

selection menu is very complicated with up/down, also screen does not repaint arrows correct. you still dont use colors every where to make the app look modern / hacker style.

use go-figure again, it looked better with it. make the font name customizable in the app settings.

make the input of alarm time simpler. let the user just input numbers instead of scrolling up/down. ENTER also leaves the time set menu.



these are all not used:

	StateBrightnessMenu
	StateBacklightMenu
	StateTimeFormatMenu
	StateSecondsMenu


display.go is very long. refactor the module and move stuff like getAvailableFiles out of it. just keep the ui stuff in.

remove all the ======= styles and all other styles. just make a simple normal TUI app which shows the time (ascii art, big) and at the bottom [SETTINGS]     [ALARM 1] [ALARM 2] and so on. be a little creative.




use bubbletea and lipgloss for formatting an 
highlighing

dont use ascii in the menu

let the user edit the alarm time completely
    via HH:MM:SS (input via keyboard, left,right 0-1, return/esc)

alarm: let the user select each day with an [X] 

alarm: like now let the user select the source.
    below it let the user customize it.
        buzzer: load all *.tone files in include/sounds/buzzer
        and use ToneParser.go for it.
        Let the user select the filename of the .tone file

        soother: remove it completely from the app.
        buzzer is enough.

        mp3: let the user input a directory name
        radio: let the user input a url/path for a m3u
        

use Bubbletea and lipgloss for everything and make it look nice.
only display the main time in the main view with ascii art.








implement the menu functionality according to README.md.

open the menu, let the user switch through the settings. ask me before making descisions you are unsure which is totally ok.

show the menu in the main screen where normally the current time is shown. use ascii art to make it look nice like TIME, ALARM 1, ALARM 2, BRIGHTNESS, BACKLIGHT, 12/24 HOURS,

(switchable with up/down or editable with sub-menu screens when setting the alarms 1 or 2 (setting for enabled, time, days, source, ...))

remove the sidecontrols right of the time, that messes the display up. center the time in the middle of the clock output.
put the side controls at the top or bottom above/below the time.

remove the seconds display as default, make it an configurable option.

use bubbletea and libgloss for colors 

