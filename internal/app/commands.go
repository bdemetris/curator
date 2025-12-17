package app

import (
	"bdemetris/curator/pkg/model"
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

func (a *App) handleShowDevices(ctx context.Context, channelID, userID string, args []string) {
	if len(args) == 0 {
		a.sendText(channelID, "Usage: `@bot show <all | mine | available | AssetTag>`")
		return
	}

	firstArg := strings.ToLower(strings.TrimSpace(args[0]))

	allDevices, err := a.DB.ListDevices(ctx)
	if err != nil {
		log.Printf("DB Error: %v", err)
		a.sendText(channelID, "❌ Error retrieving devices.")
		return
	}

	var filtered []model.Device
	var title string

	switch firstArg {
	case "all":
		filtered = allDevices
		title = "All Devices"

	case "mine":
		user, _ := a.API.GetUserInfo(userID)
		email := strings.ToLower(strings.TrimSpace(user.Profile.Email))
		for _, d := range allDevices {
			if strings.ToLower(strings.TrimSpace(d.AssignedTo)) == email && email != "" {
				filtered = append(filtered, d)
			}
		}
		title = "Your Checked-out Devices"

	case "available":
		filterText := ""
		if len(args) > 1 {
			filterText = strings.ToLower(strings.TrimSpace(strings.Join(args[1:], " ")))
		}

		for _, d := range allDevices {
			if d.AssignedTo == "" {
				dbModel := strings.ToLower(d.DeviceModel)
				dbType := strings.ToLower(d.DeviceType)
				dbAssetTag := strings.ToLower(d.AssetTag)

				if filterText == "" ||
					strings.Contains(dbModel, filterText) ||
					strings.Contains(dbType, filterText) ||
					strings.Contains(dbAssetTag, filterText) {
					filtered = append(filtered, d)
				}
			}
		}
		title = "Available Devices"
		if filterText != "" {
			title += fmt.Sprintf(" (Filter: '%s')", filterText)
		}

	default:
		for _, d := range allDevices {
			if strings.ToLower(strings.TrimSpace(d.AssetTag)) == firstArg {
				filtered = append(filtered, d)
			}
		}
		title = fmt.Sprintf("Lookup Asset Tag: %s", args[0])
	}

	if len(filtered) == 0 {
		a.sendText(channelID, fmt.Sprintf("No devices found for: *%s*", title))
		return
	}

	if len(filtered) == 1 {
		a.renderSingleDeviceDetail(channelID, filtered[0])
		return
	}

	a.renderDeviceTable(channelID, title, filtered)
}

func (a *App) handleCheckoutDevice(ctx context.Context, channelID, userID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot checkout <AssetTag>`")
		return
	}

	serial := args[0]

	user, err := a.API.GetUserInfo(userID)
	if err != nil {
		log.Printf("Slack API Error (GetUserInfo for %s): %v", userID, err)
		a.sendText(channelID, "❌ Failed to retrieve your user profile from Slack.")
		return
	}

	userEmail := user.Profile.Email
	if userEmail == "" {
		userEmail = user.Profile.DisplayName
		if userEmail == "" {
			userEmail = user.RealName
		}
		log.Printf("Warning: No email found for user %s, using fallback: %s", userID, userEmail)
	}

	updates := make(map[string]interface{})
	now := time.Now()

	updates["AssignedTo"] = userEmail
	updates["AssignedDate"] = &now

	if err := a.DB.UpdateDevice(ctx, serial, updates); err != nil {
		log.Printf("DB Update Error (Checkout %s by %s): %v", serial, userEmail, err)
		a.sendText(channelID, fmt.Sprintf("❌ Failed to checkout device `%s`: %v", serial, err))
		return
	}

	a.sendText(channelID, fmt.Sprintf("✅ Device `%s` is now checked out to *%s*.", serial, userEmail))
}
