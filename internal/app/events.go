package app

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"bdemetris/curator/internal/database"
	"bdemetris/curator/store"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

var deviceTypes = []string{"android", "ios", "macos", "windows"}

// App is the main structure holding all clients.
type App struct {
	API    *slack.Client
	Client *socketmode.Client
	DB     *database.DynamoClient
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
	case "hello", "hi":
		a.sendBlocks(channelID, createSimpleGreeting(userID))
	case "add":
		a.handleAddDevice(ctx, channelID, args)
	case "get":
		a.handleGetDevice(ctx, channelID, args)
	case "list":
		a.handleListDevices(ctx, channelID, args)
	case "update": // <<< NEW COMMAND ROUTING
		a.handleUpdateDevice(ctx, channelID, userID, args)
	default:
		a.sendBlocks(channelID, createUnknownCommandMessage(userID))
	}
}

// ------------------------------------------
// DynamoDB Command Handlers
// ------------------------------------------

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

	device := store.Device{SerialNumber: serial, AssetTag: assetTag, DeviceType: deviceType}
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
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Device Found:* `%s`", device.ID), false, false),
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*SerialNumber:*\n%s", device.SerialNumber), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*AssetTag:*\n%d", device.AssetTag), false, false),
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

	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text",
		fmt.Sprintf("📋 Found %d Device(s) in Local DynamoDB", len(devices)), false, false))

	var listBlocks []slack.Block
	listBlocks = append(listBlocks, headerBlock, slack.NewDividerBlock())

	count := 0
	const maxDisplay = 10

	for _, dev := range devices {
		if count >= maxDisplay {
			listBlocks = append(listBlocks, slack.NewContextBlock("",
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("... and %d more. Use `@bot get ID` for details.", len(devices)-count), false, false)))
			break
		}

		text := fmt.Sprintf("*<%s>* - (`$%d`)", dev.SerialNumber, dev.AssetTag)
		listBlocks = append(listBlocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil))
		count++
	}

	a.sendBlocks(channelID, listBlocks)
}

// internal/app/events.go

// @bot update D001 price:1600 type:Laptop
func (a *App) handleUpdateDevice(ctx context.Context, channelID, userID string, args []string) {
	if len(args) < 2 {
		a.sendText(channelID, "Usage: `@bot update <ID> <field:value>...` (e.g., `@bot update D001 price:1600 name:Monitor`)")
		return
	}

	deviceID := args[0]
	updates := make(map[string]interface{})

	// Iterate over the rest of the arguments (field:value pairs)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			a.sendText(channelID, fmt.Sprintf("Invalid update format: `%s`. Must be `field:value`.", arg))
			return
		}

		key := parts[0]
		valueString := parts[1]

		// 1. Map command keys to DynamoDB attribute names
		var dbKey string
		switch strings.ToLower(key) {
		case "deviceType":
			dbKey = "DeviceType"
		case "assetTag":
			dbKey = "AssetTag"
		case "assignedTo":
			dbKey = "AssignedTo"
		default:
			a.sendText(channelID, fmt.Sprintf("Unknown field to update: `%s`. Must be 'type', 'assetTag', or 'assignedTo'.", key))
			return
		}

		// 2. Handle AssetTag as an integer (DynamoDB attribute value typing)
		var dbValue interface{}
		if dbKey == "AssetTag" {
			assetTag, err := strconv.Atoi(valueString)
			if err != nil {
				a.sendText(channelID, "Error: AssetTag must be a valid integer.")
				return
			}
			dbValue = assetTag
		} else {
			dbValue = valueString
		}

		updates[dbKey] = dbValue
	}

	// 3. Call the DynamoDB update helper
	err := a.DB.UpdateDevice(ctx, deviceID, updates)
	if err != nil {
		log.Printf("DynamoDB Update Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Failed to update device `%s`: %v", deviceID, err))
		return
	}

	a.sendText(channelID, fmt.Sprintf("✅ Device `%s` successfully updated!", deviceID))
}

// ------------------------------------------
// SLACK UTILITIES (Senders)
// ------------------------------------------

func (a *App) sendText(channelID, text string) {
	_, _, err := a.API.PostMessage(
		channelID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Printf("ERROR: Failed to post text message to channel %s: %v", channelID, err)
	}
}

func (a *App) sendBlocks(channelID string, blocks []slack.Block) {
	_, _, err := a.API.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Printf("ERROR: Failed to post block message to channel %s: %v", channelID, err)
	}
}

// -----------------------------
// BLOCK KIT MESSAGE GENERATORS
// -----------------------------

func createHelpMessage(userID string) []slack.Block {
	headerText := fmt.Sprintf("👋 Hello <@%s>! I'm your DynamoDB Bot. Try me!", userID)
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, false, false))

	divider := slack.NewDividerBlock()

	sectionText := "*Here are the commands I support:*\n\n" +
		"• `@botName help` - Displays this message.\n" +
		"• `@botName add <ID> <Name> <Price> <Type>` - Saves a *device* to local DynamoDB.\n" +
		"• `@botName get <ID>` - Retrieves a single *device* from local DynamoDB.\n" +
		"• `@botName list` - Retrieves *all devices* from local DynamoDB.\n" +
		"• `@botName update <ID> <field:value>...` - *Updates* device fields (name, price, type)."

	sectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", sectionText, false, false),
		nil, nil,
	)

	contextText := "Note: You must have a DynamoDB Docker container running on localhost:8000."
	contextBlock := slack.NewContextBlock("", slack.NewTextBlockObject("mrkdwn", contextText, false, false))

	return []slack.Block{
		headerBlock,
		divider,
		sectionBlock,
		contextBlock,
	}
}

func createSimpleGreeting(userID string) []slack.Block {
	text := fmt.Sprintf("Hello <@%s>! I received your greeting.", userID)
	return []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
	}
}

func createUnknownCommandMessage(userID string) []slack.Block {
	text := fmt.Sprintf("Sorry <@%s>, I don't recognize that command. Type `@botName help` to see what I can do!", userID)
	return []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
	}
}

// -----------------------------
// OTHER HELPERS
// -----------------------------

func IsArgumentAccepted(accepted []string, arg string) bool {
	lowerArg := strings.ToLower(arg)
	for _, acceptedArg := range accepted {
		if lowerArg == acceptedArg {
			return true
		}
	}
	return false
}
