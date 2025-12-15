package app

import (
	"fmt"
	"log"

	"github.com/slack-go/slack"
)

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

func createUnknownCommandMessage(userID string) []slack.Block {
	text := fmt.Sprintf("Sorry <@%s>, I don't recognize that command. Type `@botName help` to see what I can do!", userID)
	return []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", text, false, false), nil, nil),
	}
}
