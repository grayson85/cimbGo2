package main

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

func getBriefDescription() string {
	return `
**Program Overview:**
This program monitors the SGD to MYR exchange rate from the CIMB Clicks website. It provides real-time updates and sends notifications via WhatsApp based on user-defined criteria.

**Key Features:**
- Monitors exchange rate from CIMB Clicks website.
- Provides color-coded updates:
  - Green: Rate increased
  - Red: Rate decreased
  - White: No change
- Sends WhatsApp notifications when the rate goes outside the specified range.
- Allows restarting the program with a key press or CTRL+C.

The program uses Chrome in headless mode for web scraping and the WhatsApp API for sending notifications. It's designed to run continuously to provide real-time updates on the exchange rate.
`
}

func helpInfo() {
	yellowColor := color.New(color.FgYellow).SprintfFunc()
	info :=` 
**How to Use:**

1. **Main Menu:**
   - Choose to list joined WhatsApp groups or start the monitoring program.

2. **Starting the Program:**
   - Set your desired minimum and maximum exchange rates.
   - Specify a WhatsApp target for notifications:
     - **For personal notifications, enter a phone number including the country code without the `+` sign (e.g., 60123456789).** Ensure the number starts with the country code followed directly by the phone number.
     - For group notifications, enter the group name or ID as listed in the joined groups.

3. **Monitoring:**
   - The program fetches the exchange rate every 40 - 70 seconds and displays it with color coding based on the rate's change.

4. **Notifications:**
   - Notifications are sent via WhatsApp when the rate falls outside the defined range.

5. **Restarting:**
   - Restart the program by pressing 's' or 'S' at any time.
   - Alternatively, restart the program by pressing CTRL+C and then running it again.
`
	fmt.Println(yellowColor(info))
}

func printAppInfo() {
	blueColor := color.New(color.FgBlue).SprintfFunc()
	yellowColor := color.New(color.FgYellow).SprintfFunc()
	redColor := color.New(color.FgRed).SprintfFunc()
	greenColor := color.New(color.FgGreen).SprintfFunc()

	asciiArt := `
   ______  _____  ____    ____  ______      ______            _____  
 .' ___  ||_   _||_   \  /   _||_   _ \   .' ___  |          / ___ . 
/ .'   \_|  | |    |   \/   |    | |_) | / .'   \_|   .--.  |_/___) |
| |         | |    | |\  /| |    |  __'. | |   ____ / .'  \ .'____.'
\ '.___.'\ _| |_  _| |_\/_| |_  _| |__) | \ '.___ ]  || \__. |/ /_____ 
 '.____ .'|_____||_____||_____||_______/   '.____.'  '.__.' |_______|
`

	fmt.Println(blueColor("======================================================================"))
	fmt.Println(greenColor(asciiArt))
	fmt.Println(blueColor("======================================================================"))
	fmt.Println()
	fmt.Println(blueColor("=== Version: 2.3 ==="))
	fmt.Println(blueColor("=== Grayson Lee, July 2024 ==="))
	fmt.Println()
	fmt.Println(yellowColor(getBriefDescription()))
	fmt.Println()
	fmt.Println(redColor("***** Press CTRL+C to stop the program *****"))
	fmt.Println()
}

func printColoredRate(currentRate, prevRate float64) {
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	var colorFunc func(format string, a ...interface{}) string

	switch {
	case prevRate == 0:
		colorFunc = color.New(color.FgWhite).SprintfFunc()
	case currentRate < prevRate:
		colorFunc = color.New(color.FgRed).SprintfFunc()
	case currentRate > prevRate:
		colorFunc = color.New(color.FgGreen).SprintfFunc()
	default:
		colorFunc = color.New(color.FgWhite).SprintfFunc()
	}

	fmt.Println(colorFunc("%s : Rate : SGD 1.00 = MYR %.4f", currentTime, currentRate))
}

func shouldNotify(currentRate float64, config *Config) bool {
	return (currentRate <= config.DesiredMinRate || currentRate >= config.DesiredMaxRate) &&
		currentRate != config.LastNotifiedRate
}