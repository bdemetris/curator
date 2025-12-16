package app

import (
	"context"
	"fmt"
	"log"
	"strings"

	"bdemetris/curator/pkg/store"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

var deviceTypes = []string{"android", "ios", "macos", "windows"}

// App is the main structure holding all clients.
type App struct {
	API    *slack.Client
	Client *socketmode.Client
	DB     store.Store
}

// HandleEvents listens for and processes incoming Slack events.
func (a *App) HandleEvents(ctx context.Context) {
	for evt := range a.Client.Events {
		switch evt.Type {
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				a.Client.Debugf("Ignored %+v\n", evt)
				continue
			}
			a.Client.Ack(*evt.Request)
			if eventsAPIEvent.Type == slackevents.CallbackEvent {
				a.handleCallbackEvent(ctx, eventsAPIEvent)
			}
		}
	}
}

// handleCallbackEvent processes the inner event from a generic EventsAPI payload.
func (a *App) handleCallbackEvent(ctx context.Context, eventsAPIEvent slackevents.EventsAPIEvent) {
	innerEvent := eventsAPIEvent.InnerEvent

	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Printf("Received app_mention: %s", ev.Text)

		authTestResponse, err := a.API.AuthTest()
		if err != nil {
			log.Printf("ERROR: Failed to get bot identity: %v", err)
			return
		}
		botUserID := authTestResponse.UserID

		mentionTag := fmt.Sprintf("<@%s>", botUserID)
		commandText := strings.TrimSpace(strings.Replace(ev.Text, mentionTag, "", 1))

		a.handleAppMentionCommand(ctx, ev.Channel, ev.User, commandText)
	}
}

// handleAppMentionCommand routes the command to the correct handler function.
func (a *App) handleAppMentionCommand(ctx context.Context, channelID, userID, command string) {
	parts := strings.Fields(strings.ToLower(command))
	if len(parts) == 0 {
		a.sendBlocks(channelID, createHelpMessage(userID))
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "help":
		a.sendBlocks(channelID, createHelpMessage(userID))
	case "get":
		a.handleGetDevice(ctx, channelID, args)
	case "list":
		a.handleListDevices(ctx, channelID)
	case "checkout":
		a.handleCheckoutDevice(ctx, channelID, userID, args)
	default:
		a.sendBlocks(channelID, createUnknownCommandMessage(userID))
	}
}
