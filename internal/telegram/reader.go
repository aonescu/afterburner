package telegram

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "github.com/lib/pq"
	"github.com/zelenin/go-tdlib/client"
)


const (
	dbURI   = "postgres://alex:root@localhost:5432/api?sslmode=disable"
	// apiId   = 25681228
	// apiHash = "854f4a044d9593bb5187085cf347ae5d"
	// chatId  = -1001322114329
)

var spotifyLinkRegex = regexp.MustCompile(`https?://open\.spotify\.com/track/[a-zA-Z0-9]+`)

type SpotifyMessage struct {
	MessageID int64
	Link      string
	PostedAt  time.Time
}

func extractSpotifyLinks(text string) []string {
	return spotifyLinkRegex.FindAllString(text, -1)
}

func initDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", dbURI)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS spotify_messages (
			message_id BIGINT PRIMARY KEY,
			link TEXT NOT NULL,
			posted_at TIMESTAMP NOT NULL
		);
	`)
	return db, err
}

func WriteSpotifySongstoDB() {
	_, _ = client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{NewVerbosityLevel: 0})
	db, err := initDB()
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %s", err)
	}
	defer db.Close()

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

	fromMessageId := int64(0)
	totalSaved := 0
	for {
		messages, err := tdlibClient.GetChatHistory(&client.GetChatHistoryRequest{
			ChatId:        chatId,
			FromMessageId: fromMessageId,
			Offset:        0,
			Limit:         100,
			OnlyLocal:     false,
		})
		if err != nil || len(messages.Messages) == 0 {
			break
		}

		for _, msg := range messages.Messages {
			if content, ok := msg.Content.(*client.MessageText); ok {
				links := extractSpotifyLinks(content.Text.Text)
				for _, link := range links {
					_, err := db.Exec(
						"INSERT INTO spotify_messages (message_id, link, posted_at) VALUES ($1, $2, to_timestamp($3)) ON CONFLICT DO NOTHING",
						msg.Id,
						link,
						msg.Date,
					)
					if err != nil {
						log.Printf("⚠️ Failed to insert message %d: %s", msg.Id, err)
						continue
					}
					log.Printf("✅ Saved message [%d]: %s", msg.Id, link)
					totalSaved++
				}
			}
			fromMessageId = msg.Id
		}
	}

	log.Printf("🎉 Done. Saved %d Spotify links.", totalSaved)
	os.Exit(0)
}
