package app

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// handleGetDevice gets a device based on serial number
func (a *App) handleGetDevice(ctx context.Context, channelID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot get <AssetTag>`")
		return
	}

	serial := args[0]
	device, err := a.DB.GetDevice(ctx, serial)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			a.sendText(channelID, fmt.Sprintf("Device AssetTag `%s` was not found in the database.", serial))
			return
		}
		log.Printf("Get Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error retrieving device: %v", err))
		return
	}

	resultBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Device Found:* *%s*", device.AssetTag), false, false),
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssignedTo:*\n%s", device.AssignedTo), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssignedDate:*\n%s", device.AssignedDate), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Type:*\n%d", device.DeviceType), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Make:*\n%s", device.DeviceMake), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Location:*\n%s", device.Location), false, false),
		},
		nil,
	)

	a.sendBlocks(channelID, []slack.Block{resultBlock})
}

// handleListDevice outputs a table to devices based on a simple query or "all"
func (a *App) handleListDevices(ctx context.Context, channelID string) {
	devices, err := a.DB.ListDevices(ctx)
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

	listBlocks = append(listBlocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", "*🔎 Device Inventory (Showing top 10)*", false, false),
		nil, nil,
	))
	listBlocks = append(listBlocks, slack.NewDividerBlock())

	headerText := fmt.Sprintf("```%-15s | %-15s | %-20s | %s```",
		"ASSET TAG", "TYPE", "MODEL", "ASSIGNED TO")

	listBlocks = append(listBlocks, slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", headerText, false, false), nil, nil))

	var rows strings.Builder

	for _, dev := range devices {
		if count >= maxDisplay {
			break
		}

		row := fmt.Sprintf("` %-15s | %-15s | %-20s | %s`\n",
			dev.AssetTag,
			strings.ToUpper(dev.DeviceType),
			dev.DeviceModel,
			dev.AssignedTo)

		rows.WriteString(row)
		count++
	}

	listBlocks = append(listBlocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", rows.String(), false, false), nil, nil))

	if len(devices) > maxDisplay {
		listBlocks = append(listBlocks, slack.NewContextBlock("",
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("... and *%d* more devices. Filter or use `@bot get <AssetTag>` for details.", len(devices)-count), false, false)))
	}

	a.sendBlocks(channelID, listBlocks)
}

func (a *App) handleCheckoutDevice(ctx context.Context, channelID, userID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot checkout <SerialNumber>`")
		return
	}

	serial := args[0]

	// 1. Get the User Info from Slack to retrieve the email
	user, err := a.API.GetUserInfo(userID)
	if err != nil {
		log.Printf("Slack API Error (GetUserInfo for %s): %v", userID, err)
		a.sendText(channelID, "❌ Failed to retrieve your user profile from Slack.")
		return
	}

	// Extract the email from the profile
	userEmail := user.Profile.Email
	if userEmail == "" {
		// Fallback to DisplayName if email is hidden or restricted by Slack settings
		userEmail = user.Profile.DisplayName
		if userEmail == "" {
			userEmail = user.RealName
		}
		log.Printf("Warning: No email found for user %s, using fallback: %s", userID, userEmail)
	}

	// 2. Construct the update payload
	updates := make(map[string]interface{})
	now := time.Now()

	updates["AssignedTo"] = userEmail
	updates["AssignedDate"] = &now // Using the pointer fix

	// 3. Update the database using the SerialNumber key
	if err := a.DB.UpdateDevice(ctx, serial, updates); err != nil {
		log.Printf("DB Update Error (Checkout %s by %s): %v", serial, userEmail, err)
		a.sendText(channelID, fmt.Sprintf("❌ Failed to checkout device `%s`: %v", serial, err))
		return
	}

	// 4. Success Response
	a.sendText(channelID, fmt.Sprintf("✅ Device `%s` is now checked out to *%s*.", serial, userEmail))
}
