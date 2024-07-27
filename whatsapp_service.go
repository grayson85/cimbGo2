package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite"
)

func setupWhatsAppClient(config *Config) error {
	dbLog := waLog.Stdout("Database", "ERROR", true)
	clientLog := waLog.Stdout("Client", "ERROR", true)

	dbString := "file:whatsapp.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

	container, err := sqlstore.New("sqlite", dbString, dbLog)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		return fmt.Errorf("failed to get device store: %v", err)
	}

	config.Client = whatsmeow.NewClient(deviceStore, clientLog)
	config.Client.AddEventHandler(func(evt interface{}) {
		eventHandler(evt, config)
	})

	if config.Client.Store.ID == nil {
		// No ID stored, new login
		if err := qrLogin(config.Client); err != nil {
			return fmt.Errorf("error during QR login: %v", err)
		}
	} else {
		// Already logged in, just connect
		if err := config.Client.Connect(); err != nil {
			return fmt.Errorf("error connecting: %v", err)
		}
	}

	config.Connected = true
	logger.Println("Connected successfully to WhatsApp!")
	return nil
}

func qrLogin(client *whatsmeow.Client) error {
	qrChan, _ := client.GetQRChannel(context.Background())
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	for evt := range qrChan {
		if evt.Event == "code" {
			qr, err := qrcode.New(evt.Code, qrcode.Low)
			if err != nil {
				return fmt.Errorf("failed to generate QR code: %w", err)
			}
			logger.Println("Scan this QR code with your WhatsApp app:")
			logger.Println(qr.ToSmallString(false))
		} else {
			logger.Println("Login event:", evt.Event)
		}
	}

	return nil
}

func eventHandler(evt interface{}, config *Config) {
	switch v := evt.(type) {
	case *events.Connected:
		//logger.Println("Connected to WhatsApp")
		config.Connected = true
	case *events.Disconnected:
		logger.Println("Disconnected from WhatsApp")
		config.Connected = false
		// Attempt to reconnect
		go func() {
			for !config.Connected {
				logger.Println("Attempting to reconnect...")
				err := config.Client.Connect()
				if err != nil {
					logger.Printf("Failed to reconnect: %v", err)
					time.Sleep(5 * time.Second)
				} else {
					logger.Println("Reconnected successfully")
					config.Connected = true
				}
			}
		}()
	case *events.LoggedOut:
		logger.Println("Device logged out")
		config.Connected = false
		// Prompt for new login
		err := qrLogin(config.Client)
		if err != nil {
			logger.Printf("Failed to login with QR: %v", err)
		}
	default:
		_ = v
	}
}

func sendWhatsAppNotification(config *Config, rate float64) {
	if !config.Connected {
		logger.Println("WhatsApp client not connected. Skipping notification.")
		return
	}

	var recipient types.JID
	if config.IsGroup {
		matchedGroupID, err := matchGroupID(config.Client, config.NotifyTarget)
		if err != nil {
			logger.Printf("Error matching group ID: %v", err)
			return
		}
		// Ensure the group ID is in the correct format
		trimmedID := strings.TrimSuffix(matchedGroupID, "@g.us")
		recipient = types.NewJID(trimmedID, types.GroupServer)
	} else {
		recipient = types.NewJID(config.NotifyTarget, types.DefaultUserServer)
	}

	message := fmt.Sprintf("Alert: The current rate is SGD 1.00 = MYR %.4f", rate)
	msg := &waProto.Message{Conversation: proto.String(message)}

	maxRetries := 3
	retryDelay := time.Second * 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := config.Client.SendMessage(context.Background(), recipient, msg)
		if err == nil {
			logger.Println("WhatsApp notification sent successfully")
			return
		}

		logger.Printf("Attempt %d failed: %v. Retrying in %v...", attempt+1, err, retryDelay)
		time.Sleep(retryDelay)
	}

	logger.Printf("Failed to send WhatsApp message after %d attempts", maxRetries)
	
    // If all attempts fail, try sending to yourself as a fallback
	if config.IsGroup {
		selfJID := config.Client.Store.ID.ToNonAD()
		_, err := config.Client.SendMessage(context.Background(), selfJID, msg)
		if err != nil {
			logger.Printf("Failed to send fallback message to self: %v", err)
		} else {
			logger.Println("Fallback message sent to self successfully")
		}
	}
}

func listJoinedGroups(config *Config) {
	groups, err := config.Client.GetJoinedGroups()
	if err != nil {
		logger.Printf("Error fetching joined groups: %v", err)
		return
	}

	logger.Println("Joined Groups:")
	for _, group := range groups {
		fullJID := group.JID.String()
		trimmedJID := strings.TrimSuffix(fullJID, "@g.us")
		logger.Printf("- Name: %s\n  Full ID: %s\n  Trimmed ID: %s\n  Owner: %s\n\n",
			group.Name,
			fullJID,
			trimmedJID,
			group.OwnerJID.String())
	}
}

func matchGroupID(client *whatsmeow.Client, inviteLinkID string) (string, error) {
	groups, err := client.GetJoinedGroups()
	if err != nil {
		return "", fmt.Errorf("error fetching joined groups: %v", err)
	}

	for _, group := range groups {
		trimmedJID := strings.TrimSuffix(group.JID.String(), "@g.us")
		if strings.Contains(group.Name, inviteLinkID) ||
			strings.Contains(trimmedJID, inviteLinkID) ||
			strings.Contains(group.JID.String(), inviteLinkID) {
			return trimmedJID, nil
		}
	}

	return "", fmt.Errorf("no matching group found for ID: %s", inviteLinkID)
}

func isGroupIdentifier(input string) bool {
	for _, char := range input {
		if (char < '0' || char > '9') && char != '-' && char != '_' {
			return true
		}
	}
	return false
}

func extractGroupIDFromLink(link string) string {
	parts := strings.Split(link, "/")
	return parts[len(parts)-1]
}

func setupWhatsAppPreferences(config *Config) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Enter WhatsApp target:")
	fmt.Println("- For personal notifications, enter a phone number (e.g., 60123456789)")
	fmt.Println("- For group notifications, enter the group name or group ID")
	fmt.Print("Your input: ")
	scanner.Scan()
	config.NotifyTarget = strings.TrimSpace(scanner.Text())

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
}