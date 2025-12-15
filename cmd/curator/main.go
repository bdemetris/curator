package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"bdemetris/curator/internal/app"
	"bdemetris/curator/pkg/database"
	"bdemetris/curator/pkg/store"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	ctx := context.Background()

	// START STORE INIT

	provider := os.Getenv("DATABASE_PROVIDER")
	if provider == "" {
		provider = store.ProviderDynamoDB // Default to DynamoDB
	}

	cfg := store.StoreConfig{
		Provider:         provider,
		DynamoDBEndpoint: os.Getenv("DYNAMODB_ENDPOINT"), // e.g., "http://localhost:8000"
	}

	constructors := map[string]store.StoreConstructor{
		store.ProviderDynamoDB: func(ctx context.Context, config string) (store.Store, error) {
			return database.NewDynamoStore(ctx, config)
		},
		// add new store constructors here
	}

	dbStore, err := store.NewStoreFactory(ctx, cfg, constructors)
	if err != nil {
		log.Fatalf("Failed to initialize database store: %v", err)
	}
	defer dbStore.Close()

	log.Printf("Successfully initialized store using provider: %s", cfg.Provider)

	// END STORE INIT

	// START SLACK BOT INIT

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

	slackApp := &app.App{
		API:    api,
		Client: client,
		DB:     dbStore,
	}

	fmt.Println("Starting Socket Mode listener...")

	go slackApp.HandleEvents(ctx)

	if err := client.Run(); err != nil {
		log.Fatalf("Socket Mode client failed: %v", err)
	}
}
