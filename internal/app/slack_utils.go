package app

import (
	"bdemetris/curator/pkg/model"
	"fmt"
	"log"
	"strings"

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
		"• `@botName add <SerialNumber> <AssetTag> <Type>` - Saves a *device*\n" +
		"• `@botName get <SerialNumber>` - Retrieves a single *device*\n" +
		"• `@botName list` - Retrieves *devices*. Supports <all>, <device type> eg android, ios \n" +
		"• `@botName assign <SerialNumber> <UserName>...` - *assigns* device to user."

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

func (a *App) renderDeviceTable(channelID, title string, devices []model.Device) {
	// Slack blocks have a 3000 char limit. 10-12 devices is the "safe" zone for a table.
	const maxDisplay = 10

	listBlocks := []slack.Block{
		slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🔎 *%s* (%d found)", title, len(devices)), false, false), nil, nil),
		slack.NewDividerBlock(),
	}

	var rows strings.Builder

	rows.WriteString(fmt.Sprintf("```%-15s | %-20s | %s```\n", "ASSET TAG", "TYPE", "ASSIGNED TO"))

	for i, dev := range devices {
		if i >= maxDisplay {
			break
		}

		status := "Available"
		if dev.AssignedTo != "" {
			// Truncate email if it's too long to keep the table aligned
			status = dev.AssignedTo
			if len(status) > 20 {
				status = status[:17] + "..."
			}
		}

		line := fmt.Sprintf("`%-15s | %-12s | %s`\n",
			dev.AssetTag,
			strings.ToUpper(dev.DeviceType),
			status,
		)
		rows.WriteString(line)
	}

	listBlocks = append(listBlocks, slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", rows.String(), false, false), nil, nil))

	if len(devices) > maxDisplay {
		remaining := len(devices) - maxDisplay
		footerText := fmt.Sprintf("_Showing top %d results. There are *%d* more devices. Try a more specific search (e.g., `@bot show available macbook`)_", maxDisplay, remaining)
		listBlocks = append(listBlocks, slack.NewContextBlock("",
			slack.NewTextBlockObject("mrkdwn", footerText, false, false),
		))
	}

	a.sendBlocks(channelID, listBlocks)
}

func (a *App) renderSingleDeviceDetail(channelID string, dev model.Device) {
	status := "✅ Available"
	if dev.AssignedTo != "" {
		status = fmt.Sprintf("👤 Assigned to %s", dev.AssignedTo)
	}

	fields := []*slack.TextBlockObject{
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Asset Tag:*\n%s", dev.AssetTag), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Type:*\n%s", strings.ToUpper(dev.DeviceType)), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Model:*\n%s", dev.DeviceModel), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Status:*\n%s", status), false, false),
	}

	if dev.AssignedDate != nil {
		fields = append(fields, slack.NewTextBlockObject("mrkdwn",
			fmt.Sprintf("*Checked Out:*\n%s", dev.AssignedDate.Format("Jan 02, 2006")), false, false))
	}

	blocks := []slack.Block{
		slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", "📱 Device Information", false, false)),
		slack.NewSectionBlock(nil, fields, nil),
	}

	a.sendBlocks(channelID, blocks)
}
