package app

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"bdemetris/curator/internal/database"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// App is the main structure holding all clients.
type App struct {
	API    *slack.Client
	Client *socketmode.Client
	DB     *database.DynamoClient // Our new database client
}

// HandleEvents listens for and processes incoming Slack events.
func (a *App) HandleEvents(ctx context.Context) { // <-- ctx is available here
	for evt := range a.Client.Events {
		switch evt.Type {
		// ... (other cases) ...
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				a.Client.Debugf("Ignored %+v\n", evt)
				continue
			}

			a.Client.Ack(*evt.Request)

			if eventsAPIEvent.Type == slackevents.CallbackEvent {
				// CORRECTED: Pass ctx as the first argument
				a.handleCallbackEvent(ctx, eventsAPIEvent)
			}
		}
	}
}

// internal/app/events.go

// handleCallbackEvent processes the inner event from a generic EventsAPI payload.
// It now takes the full EventsAPIEvent structure.
func (a *App) handleCallbackEvent(ctx context.Context, eventsAPIEvent slackevents.EventsAPIEvent) {
	// Access the InnerEvent field from the passed structure
	innerEvent := eventsAPIEvent.InnerEvent

	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Printf("Received app_mention: %s", ev.Text)

		authTestResponse, err := a.API.AuthTest()
		// ... (rest of AppMention handling code is the same)
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
		a.handleAddProduct(ctx, channelID, userID, args)
	case "get":
		a.handleGetProduct(ctx, channelID, userID, args)
	default:
		a.sendBlocks(channelID, createUnknownCommandMessage(userID))
	}
}

// ------------------------------------------
// DynamoDB Command Handlers
// ------------------------------------------

// @bot add P1002 Monitor 250
func (a *App) handleAddProduct(ctx context.Context, channelID, userID string, args []string) {
	if len(args) != 3 {
		a.sendText(channelID, "Usage: `@bot add <ID> <Name> <Price>` (Price must be a number)")
		return
	}

	id := args[0]
	name := args[1]
	price, err := strconv.Atoi(args[2])
	if err != nil {
		a.sendText(channelID, "Error: Price must be a valid integer.")
		return
	}

	product := database.Product{ID: id, Name: name, Price: price}
	if err := a.DB.PutProduct(ctx, product); err != nil {
		log.Printf("DynamoDB Put Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error saving product to DynamoDB: %v", err))
		return
	}

	a.sendText(channelID, fmt.Sprintf("✅ Product `%s` saved to local DynamoDB!", id))
}

// @bot get P1002
func (a *App) handleGetProduct(ctx context.Context, channelID, userID string, args []string) {
	if len(args) != 1 {
		a.sendText(channelID, "Usage: `@bot get <ID>`")
		return
	}

	id := args[0]
	product, err := a.DB.GetProduct(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			a.sendText(channelID, fmt.Sprintf("Product ID `%s` was not found in the database.", id))
			return
		}
		log.Printf("DynamoDB Get Error: %v", err)
		a.sendText(channelID, fmt.Sprintf("Error retrieving product: %v", err))
		return
	}

	// Use Block Kit for a nice result display
	resultBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Product Found:* `%s`", product.ID), false, false),
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Name:*\n%s", product.Name), false, false),
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Price:*\n$%d", product.Price), false, false),
		},
		nil,
	)

	a.sendBlocks(channelID, []slack.Block{resultBlock})
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

// ------------------------------------------
// BLOCK KIT MESSAGE GENERATORS (Unchanged)
// ------------------------------------------

func createHelpMessage(userID string) []slack.Block {
	headerText := fmt.Sprintf("👋 Hello <@%s>! I'm your DynamoDB Bot. Try me!", userID)
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", headerText, false, false))

	divider := slack.NewDividerBlock()

	sectionText := "*Here are the commands I support:*\n\n" +
		"• `@botName help` - Displays this message.\n" +
		"• `@botName add <ID> <Name> <Price>` - Saves a product to local DynamoDB.\n" +
		"• `@botName get <ID>` - Retrieves a product from local DynamoDB."

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
