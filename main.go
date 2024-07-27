package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"strconv"
	"syscall"
	"time"

	"github.com/fatih/color"
	"go.mau.fi/whatsmeow"
)

type Config struct {
	Client           *whatsmeow.Client
	Connected        bool
	DesiredMinRate   float64
	DesiredMaxRate   float64
	NotifyTarget     string
	IsGroup          bool
	LastNotifiedRate float64
}

var (
	config Config
	logger *log.Logger
)

func main() {
    // Set up logger
    logger = log.New(os.Stdout, "CIMB Go: ", log.Ldate|log.Ltime)

    // Print application information
    printAppInfo()

    // Set up WhatsApp client
    //setupWhatsAppClient(&config)
    err := setupWhatsAppClient(&config)
    if err != nil {
        logger.Fatalf("Failed to set up WhatsApp client: %v", err)
    }

    // Set up signal handling for graceful shutdown
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

    //defer killAllChromeInstances()

    for {
        choice := showMainMenu()
        switch choice {
        case "1":
            listJoinedGroups(&config)
        case "2":
            startProgram(signalChan)
	case "h","H":
            helpInfo()
        case "q", "Q":
            logger.Println("Exiting program...")
            return
        default:
            logger.Println("Invalid choice. Please try again.")
        }
    }
}

func showMainMenu() string {
    fmt.Println("\nMain Menu:")
    fmt.Println("1. List joined WhatsApp groups")
    fmt.Println("2. Start program")
    fmt.Println("H. How to use")
    fmt.Println("Q. Quit")
    fmt.Print("Enter your choice: ")
	
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Scan()
    return scanner.Text()
}

func startProgram(signalChan chan os.Signal) {
    redColor := color.New(color.FgRed).SprintfFunc()

    // Set up user preferences
    setupUserPreferences()

    // Previous label value
    var prevRate float64

    // Create initial context
    ctx, cancel := createChromeContext()
    defer cancel()

    // Create a channel to signal program restart
    restartChan := make(chan bool)

    // Start input checker in a separate goroutine
    go checkForRestart(restartChan)

    // Create a ticker for 1-minute intervals
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    fmt.Println(redColor("Program started.... Press 's' or 'S' and Enter at any time to restart."))

    // Perform initial fetch
    err := fetchAndPrintLabelWithRetry(ctx, &prevRate, &config)
    if err != nil {
        logger.Printf("Initial fetch error: %v", err)
    }

    for {
        select {
        case <-ticker.C:
            // Fetch and print label every 1 minute
            err := fetchAndPrintLabelWithRetry(ctx, &prevRate, &config)
            if err != nil {
                logger.Printf("Error after retries: %v. Recreating Chrome context.", err)
                cancel()
                ctx, cancel = createChromeContext()
            }
        case <-restartChan:
            logger.Println("Restarting program...")
            cancel()
            killAllChromeInstances()
            return
        case <-signalChan:
            logger.Println("Received interrupt signal. Shutting down...")
            cancel()
            killAllChromeInstances()
            return
        case <-ctx.Done():
            // Exit if the context is done
            return
        default:
            // Small sleep to prevent CPU hogging
            time.Sleep(100 * time.Millisecond)
        }
    }
}

func checkForRestart(restartChan chan<- bool) {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        input := scanner.Text()
        if input == "s" || input == "S" {
            restartChan <- true
            return
        }
    }
    if err := scanner.Err(); err != nil {
        logger.Printf("Error reading standard input: %v", err)
    }
}

func setupUserPreferences() {
	scanner := bufio.NewScanner(os.Stdin)
	hiCyanColor := color.New(color.FgHiCyan).SprintfFunc()
	
	// Get desired minimum rate
	for {
		fmt.Print("Enter desired minimum rate: ")
		scanner.Scan()
		input := scanner.Text()
		var err error
		config.DesiredMinRate, err = strconv.ParseFloat(input, 64)
		if err == nil {
			break
		}
		logger.Println("Invalid input. Please enter a valid number.")
	}

	// Get desired maximum rate
	for {
		fmt.Print("Enter desired maximum rate: ")
		scanner.Scan()
		input := scanner.Text()
		var err error
		config.DesiredMaxRate, err = strconv.ParseFloat(input, 64)
		if err == nil && config.DesiredMaxRate > config.DesiredMinRate {
			break
		}
		if err != nil {
			logger.Println("Invalid input. Please enter a valid number.")
		} else {
			logger.Println("Maximum rate must be greater than minimum rate. Please try again.")
		}
	}

	// Get WhatsApp target
	fmt.Println("Enter WhatsApp target:")
	fmt.Println("- For personal notifications, enter a phone number (e.g., 60123456789)")
	fmt.Println("- For group notifications, enter the group name or group ID")
	fmt.Print("Your input: ")
	scanner.Scan()
	config.NotifyTarget = strings.TrimSpace(scanner.Text())

	// Determine if it's a group or personal number
	config.IsGroup = isGroupIdentifier(config.NotifyTarget)

	if config.IsGroup {
		matchedGroupID, err := matchGroupID(config.Client, config.NotifyTarget)
		if err != nil {
			logger.Printf("Error: %v\n", err)
			logger.Println("Setting target to personal WhatsApp number.")
			config.IsGroup = false
		} else {
			config.NotifyTarget = matchedGroupID
			logger.Printf("Matched group ID: %s\n", config.NotifyTarget)
			logger.Println("Target set to WhatsApp group")
		}
	} else {
		logger.Println("Target set to personal WhatsApp number:", config.NotifyTarget)
	}

	// Confirm settings
	fmt.Println(hiCyanColor("\nCurrent settings:"))
	fmt.Println(hiCyanColor("Minimum Rate: %.4f", config.DesiredMinRate))
	fmt.Println(hiCyanColor("Maximum Rate: %.4f", config.DesiredMaxRate))
	fmt.Println(hiCyanColor("Notification Target: %s (%s)\n", config.NotifyTarget, map[bool]string{true: "Group", false: "Personal"}[config.IsGroup]))
}
