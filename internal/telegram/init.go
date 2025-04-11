package telegram

import (
	"github.com/zelenin/go-tdlib/client"
	"log"
	"path/filepath"
)

const (
	apiId   = 25681228
	apiHash = "854f4a044d9593bb5187085cf347ae5d"
	chatId  = -1001322114329 
)

func MustGetTdClient() {
	// Reduce TDLib verbosity
	_, _ = client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{NewVerbosityLevel: 0})

	tdlibParameters := &client.SetTdlibParametersRequest{
		UseTestDc:              false,
		DatabaseDirectory:      filepath.Join(".tdlib", "database"),
		FilesDirectory:         filepath.Join(".tdlib", "files"),
		UseFileDatabase:        true,
		UseChatInfoDatabase:    true,
		UseMessageDatabase:     true,
		UseSecretChats:         false,
		ApiId:                  apiId,
		ApiHash:                apiHash,
		SystemLanguageCode:     "en",
		DeviceModel:            "Server",
		SystemVersion:          "1.0.0",
		ApplicationVersion:     "1.0.0",
	}
	authorizer := client.ClientAuthorizer(tdlibParameters)
	go client.CliInteractor(authorizer)

	tdlibClient, err := client.NewClient(authorizer)
	if err != nil {
		log.Fatalf("❌ NewClient error: %s", err)
	}

	messages, err := tdlibClient.GetChatHistory(&client.GetChatHistoryRequest{
		ChatId:        chatId,
		FromMessageId: 0,
		Offset:        0,
		Limit:         15,
		OnlyLocal:     false,
	})
	if err != nil {
		log.Fatalf("❌ GetChatHistory error: %s", err)
	}

	log.Printf("✅ Fetched %d messages:\n", len(messages.Messages))
	for _, msg := range messages.Messages {
		text := "<non-text message>"
		if content, ok := msg.Content.(*client.MessageText); ok {
			text = content.Text.Text
		}
		log.Printf("📨 [%d] %s", msg.Id, text)
	}
}