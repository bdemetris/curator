package app

import (
	"bdemetris/curator/pkg/model"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/slack-go/slack"
)

var RunOverdueCheckerEvery = 1 * time.Hour // testing run check ever minute
var ItemIsOverduePeriod = 30 * time.Hour   // testing 30 seconds until overdue

// StartOverdueChecker runs a background loop that checks for overdue devices every 24 hours.
func (a *App) StartOverdueChecker(ctx context.Context) {
	ticker := time.NewTicker(RunOverdueCheckerEvery)
	defer ticker.Stop()

	log.Println("🚀 Background overdue checker started...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.checkAndNotifyOverdue(ctx)
		}
	}
}

func (a *App) checkAndNotifyOverdue(ctx context.Context) {
	devices, _ := a.DB.ListDevices(ctx)
	now := time.Now()

	for _, dev := range devices {
		if dev.DueDate != nil && dev.AssignedTo != "" {
			if now.After(*dev.DueDate) {
				log.Printf("⚠️ Device %s is past due date (%v)", dev.AssetTag, dev.DueDate)
				a.notifyOverdueAssignee(dev)
			}
		}
	}
}

func (a *App) notifyOverdueAssignee(dev model.Device) {
	user, err := a.API.GetUserByEmail(dev.AssignedTo)
	if err != nil {
		log.Printf("❌ Could not find Slack user for email %s: %v", dev.AssignedTo, err)
		return
	}

	channel, _, _, err := a.API.OpenConversation(&slack.OpenConversationParameters{
		Users: []string{user.ID},
	})
	if err != nil {
		log.Printf("❌ Failed to open DM with %s: %v", user.ID, err)
		return
	}

	message := fmt.Sprintf(
		"👋 Hi %s! You've had device `%s` (%s) for over 30 days. Please return it or renew it!",
		user.RealName, dev.AssetTag, dev.DeviceModel,
	)

	a.sendText(channel.ID, message)
}
