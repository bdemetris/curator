package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"bdemetris/curator/internal/app"
	"bdemetris/curator/internal/database"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	ctx := context.Background()

	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")

	if appToken == "" || !strings.HasPrefix(appToken, "xapp-") || botToken == "" || !strings.HasPrefix(botToken, "xoxb-") {
		log.Fatal("SLACK_APP_TOKEN (xapp-...) and SLACK_BOT_TOKEN (xoxb-...) environment variables are required.")
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(false),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(false),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	log.Println("Initializing local DynamoDB connection...")
	dbClient, err := database.NewDynamoClient(ctx)
	if err != nil {
		log.Fatalf("FATAL: Could not connect to local DynamoDB: %v. Is Docker running?", err)
	}
	log.Println("DynamoDB client initialized and table assured.")

	slackApp := &app.App{
		API:    api,
		Client: client,
		DB:     dbClient,
	}

	fmt.Println("Starting Socket Mode listener...")

	go slackApp.HandleEvents(ctx)

	if err := client.Run(); err != nil {
		log.Fatalf("Socket Mode client failed: %v", err)
	}
}
