package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var (
	chromeContexts  []context.CancelFunc
	chromeProcesses []*os.Process
)

func createChromeContext() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-logging", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-infobars", true),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(logger.Printf))

	if err := chromedp.Run(ctx); err != nil {
		logger.Printf("Failed to start Chrome: %v", err)
		return ctx, func() {
			cancel()
			allocCancel()
		}
	}

	time.Sleep(time.Second)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq chrome.exe", "/FO", "CSV", "/NH")
	} else {
		cmd = exec.Command("pgrep", "chrome")
	}

	out, err := cmd.Output()
	if err != nil {
		logger.Printf("Failed to get Chrome PID: %v", err)
	} else {
		pids := parsePIDs(out)
		for _, pid := range pids {
			if process, err := os.FindProcess(pid); err == nil {
				chromeProcesses = append(chromeProcesses, process)
			}
		}
	}

	return ctx, func() {
		cancel()
		allocCancel()
	}
}

func parsePIDs(output []byte) []int {
	var pids []int
	if runtime.GOOS == "windows" {
		for _, line := range strings.Split(string(output), "\n") {
			fields := strings.Split(line, ",")
			if len(fields) > 2 {
				pid, err := strconv.Atoi(strings.Trim(fields[1], "\""))
				if err == nil {
					pids = append(pids, pid)
				}
			}
		}
	} else {
		for _, pidStr := range strings.Fields(string(output)) {
			pid, err := strconv.Atoi(pidStr)
			if err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

func killAllChromeInstances() {
	logger.Println("Attempting to kill all Chrome instances started by the application...")

	for _, cancel := range chromeContexts {
		cancel()
	}
	chromeContexts = nil

	for _, process := range chromeProcesses {
		logger.Printf("Killing Chrome process with PID: %d", process.Pid)
		err := process.Kill()
		if err != nil {
			logger.Printf("Error killing Chrome process %d: %v", process.Pid, err)
		}
	}
	chromeProcesses = nil

	logger.Println("Finished attempting to kill all Chrome instances started by the application.")
}

func getChromePIDCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("tasklist", "/FI", "IMAGENAME eq chrome.exe", "/FO", "CSV", "/NH")
	}
	return exec.Command("pgrep", "chrome")
}

func getForceKillCommand() *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/F", "/IM", "chrome.exe")
	}
	return exec.Command("pkill", "-9", "chrome")
}

func fetchAndPrintLabelWithRetry(ctx context.Context, prevRate *float64, config *Config) error {
	const (
		maxRetries = 3
		retryDelay = 5 * time.Second
	)

	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if err = fetchAndPrintLabel(ctx, prevRate, config); err == nil {
			return nil
		}
		logger.Printf("Attempt %d failed: %v. Retrying in %v...", attempt+1, err, retryDelay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
	}
	return fmt.Errorf("failed to fetch label after %d attempts: %w", maxRetries, err)
}

func fetchAndPrintLabel(ctx context.Context, prevRate *float64, config *Config) error {
	var labelContent string
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://www.cimbclicks.com.sg/sgd-to-myr"),
		chromedp.WaitVisible(`#rateStr`, chromedp.ByID),
		chromedp.Text(`#rateStr`, &labelContent, chromedp.ByID),
	)
	if err != nil {
		return fmt.Errorf("error fetching label: %w", err)
	}

	currentRate, err := parseRate(labelContent)
	if err != nil {
		return err
	}

	printColoredRate(currentRate, *prevRate)

	if shouldNotify(currentRate, config) {
		sendWhatsAppNotification(config, currentRate)
		config.LastNotifiedRate = currentRate
	}

	*prevRate = currentRate
	return nil
}

func parseRate(labelContent string) (float64, error) {
	rateStr := strings.TrimPrefix(labelContent, "SGD 1.00 = MYR ")
	currentRate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing rate: %w", err)
	}
	return currentRate, nil
}