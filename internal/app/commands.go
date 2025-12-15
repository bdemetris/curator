package app

import (
	"bdemetris/curator/pkg/model"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

func (a *App) handleAddDevice(ctx context.Context, channelID string, args []string) {
	if len(args) != 3 {
		a.sendText(channelID, "Usage: `@bot add <SerialNumber> <AssetTag> <DeviceType>` (AssetTag must be a number)")
		return
	}

	serial := args[0]
	assetTag, err := strconv.Atoi(args[1])
	if err != nil {
		a.sendText(channelID, "Error: Asset Tag must be a valid integer.")
		return
	}
	deviceType := args[2]
	if !IsArgumentAccepted(deviceTypes, deviceType) {
		a.sendText(channelID, "Usage: `@bot add <SerialNumber> <AssetTag> <DeviceType>` (Device Type must be one of android, ios, macos, windows)")
		return
	}

	device := model.Device{SerialNumber: serial, AssetTag: assetTag, DeviceType: deviceType}
	if err := a.DB.PutDevice(ctx, device); err != nil {
		log.Printf("DynamoDB Put Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error saving device to DynamoDB: %v", err))
		return
	}

	a.sendText(channelID, fmt.Sprintf("✅ Device `%s` saved to local DynamoDB!", serial))
}

func (a *App) handleGetDevice(ctx context.Context, channelID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot get <SerialNumber>`")
		return
	}

	serial := args[0]
	device, err := a.DB.GetDevice(ctx, serial)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			a.sendText(channelID, fmt.Sprintf("Device SerialNumber `%s` was not found in the database.", serial))
			return
		}
		log.Printf("DynamoDB Get Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error retrieving device: %v", err))
		return
	}

	resultBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Device Found:* *%s*", device.SerialNumber), false, false),
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssignedTo:*\n%s", device.AssignedTo), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssignedDate:*\n%s", device.AssignedDate), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssetTag:*\n%d", device.AssetTag), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*DeviceType:*\n%s", device.DeviceType), false, false),
		},
		nil,
	)

	a.sendBlocks(channelID, []slack.Block{resultBlock})
}

// @bot list
func (a *App) handleListDevices(ctx context.Context, channelID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot list <DeviceType>` (DeviceType is one of all, android, ios, macos, windows)")
		return
	}
	query := args[0]

	devices, err := a.DB.ListDevices(ctx, query)
	if err != nil {
		log.Printf("DynamoDB List Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error listing devices: %v", err))
		return
	}

	if len(devices) == 0 {
		a.sendText(channelID, "The database is currently empty. Try `@bot add D001 Laptop 1500`!")
		return
	}

	listBlocks := []slack.Block{}
	count := 0
	const maxDisplay = 10

	// 1. Add Header Block
	listBlocks = append(listBlocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", "*🔎 Device Inventory (Showing top %d)*", false, false),
		nil, nil,
	))
	listBlocks = append(listBlocks, slack.NewDividerBlock())

	// 2. Add Column Headers
	headerText := fmt.Sprintf("```%-15s | %-15s | %-15s | %s```",
		"SERIAL NUMBER", "TYPE", "MODEL", "ASSIGNED TO")

	listBlocks = append(listBlocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", headerText, false, false), nil, nil))

	// 3. Loop through Devices and Format Rows
	var rows strings.Builder

	for _, dev := range devices {
		if count >= maxDisplay {
			break
		}

		// Use fixed-width formatting within a single text block
		// Note: The number of spaces must be exact to line up with the header
		row := fmt.Sprintf("` %-15s | %-15s | %-15s | %s`\n",
			dev.SerialNumber,
			strings.ToUpper(dev.DeviceType),
			dev.ModelName,
			dev.AssignedTo)

		rows.WriteString(row)
		count++
	}

	// Append all rows as a single text block for consistent alignment
	listBlocks = append(listBlocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", rows.String(), false, false), nil, nil))

	// 4. Add "and more" message if applicable
	if len(devices) > maxDisplay {
		listBlocks = append(listBlocks, slack.NewContextBlock("",
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("... and *%d* more devices. Filter or use `@bot get <SN>` for details.", len(devices)-count), false, false)))
	}

	a.sendBlocks(channelID, listBlocks)
}

// internal/app/events.go
// handleAssignDevice assigns a device to a user and datestamps the transaction
func (a *App) handleAssignDevice(ctx context.Context, channelID string, args []string) {
	if len(args) != 2 {
		a.sendText(channelID, "Usage: `@bot assign <SerialNumber> <UserName>` (e.g., `@bot assign ABC-123 john.doe`)")
		return
	}

	deviceID := args[0]
	newAssignedUser := args[1]

	if strings.TrimSpace(newAssignedUser) == "" {
		a.sendText(channelID, "Error: User name cannot be empty.")
		return
	}

	updates := make(map[string]interface{})

	updates["AssignedTo"] = newAssignedUser
	updates["AssignedDate"] = time.Now()

	if err := a.DB.UpdateDevice(ctx, deviceID, updates); err != nil {
		log.Printf("DB Update Error (Assign Device %s to %s): %v", deviceID, newAssignedUser, err)

		a.sendText(channelID, fmt.Sprintf("❌ Failed to assign device `%s`: %v", deviceID, err))
		return
	}

	a.sendText(channelID, fmt.Sprintf("✅ Device `%s` is now assigned to *%s*.", deviceID, newAssignedUser))
}
