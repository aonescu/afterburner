package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/zelenin/go-tdlib/client"
	"github.com/zmb3/spotify"
)

var (
	ch          = make(chan *spotify.Client)
	redirectURI = "https://5bd0-2a00-1858-1028-8442-e4ac-ff77-82c7-940c.ngrok-free.app/callback"
	auth        = spotify.NewAuthenticator(redirectURI, spotify.ScopePlaylistModifyPrivate, spotify.ScopePlaylistModifyPublic)
	state       = "abc123"
)

const dbURI = "postgres://alex:root@localhost:5432/api?sslmode=disable"

// Regex pattern for extracting Spotify links
var spotifyLinkRegex = regexp.MustCompile(`https?://open\.spotify\.com/track/[a-zA-Z0-9]+`)

func init() {
	// Load environment variables from .env file (optional)
	_ = godotenv.Load()
	auth.SetAuthInfo(os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))

	// Read configuration from config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("❌ Failed to read config file: %v", err)
	}
}

func main() {
	// Get Spotify credentials and redirect URI from config
	spotifyClientID := viper.GetString("spotify.client_id")
	spotifyClientSecret := viper.GetString("spotify.client_secret")
	redirectURI := viper.GetString("spotify.redirect_uri")

	// Validate if the credentials are set
	if spotifyClientID == "" || spotifyClientSecret == "" || redirectURI == "" {
		log.Fatal("❌ SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET or REDIRECT_URI are not set in the config")
	}

	// Start connecting to the Postgres database
	log.Println("🚀 Connecting to Postgres...")
	db, err := sql.Open("postgres", dbURI)
	if err != nil {
		log.Fatalf("❌ Failed to connect to DB: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("❌ Failed to ping DB: %v", err)
	}
	log.Println("✅ Connected to DB")

	// Authenticate and get the Spotify client
	client := authenticateSpotify()

	// Check number of Spotify messages in the database
	var songCount int
	err = db.QueryRow("SELECT COUNT(*) FROM spotify_messages").Scan(&songCount)
	if err != nil {
		log.Fatalf("❌ Failed to query song count: %v", err)
	}

	// If the song count is exactly 1234, run the playlist creation process
	if songCount == 1234 {
		log.Println("🎧 Creating playlist from DB...")
		createPlaylistFromDB(db, "0y0qggz6wbi30t986h9yjkmov", client)
	} else {
		log.Printf("⚠️ Song count is not 1234. Skipping playlist creation. Current count: %d", songCount)
	}

	// Call Telegram API to fetch and store Spotify links
	writeSpotifySongsToDB()
}

// authenticateSpotify performs the Spotify authentication process
func authenticateSpotify() *spotify.Client {
	// Generate the authorization URL
	url := auth.AuthURL(state)
	fmt.Println("🔗 Please log in to Spotify by visiting the following URL in your browser:\n\n", url)

	// Wait for callback to finish (done in main)
	client := <-ch

	// Get current user info
	user, err := client.CurrentUser()
	if err != nil {
		log.Fatalf("❌ Failed to get current user: %v", err)
	}
	log.Printf("✅ Logged in as: %s (%s)\n", user.ID, user.DisplayName)

	return client
}

// completeAuth is the callback handler to complete the Spotify auth process
func completeAuth(w http.ResponseWriter, r *http.Request) {
	// Complete Spotify authentication
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatalf("❌ Token error: %v", err)
		return
	}

	// Verify state value to prevent CSRF
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("❌ State mismatch: %s != %s", st, state)
		return
	}

	client := auth.NewClient(tok)
	fmt.Fprintln(w, "🎉 Login successful! You can close this tab.")
	ch <- &client
}

// createPlaylistFromDB fetches Spotify links from the DB and builds a playlist.
func createPlaylistFromDB(db *sql.DB, userID string, client *spotify.Client) {
	log.Println("🎧 Creating playlist from DB...")

	// Step 1: Collect Spotify links from DB
	rows, err := db.Query(`SELECT link FROM spotify_messages`)
	if err != nil {
		log.Fatalf("❌ Failed to query links: %v", err)
	}
	defer rows.Close()

	var trackURLs []string
	for rows.Next() {
		var link string
		if err := rows.Scan(&link); err != nil {
			log.Printf("⚠️ Error scanning link: %v", err)
			continue
		}
		trackURLs = append(trackURLs, link)
	}
	log.Printf("🎧 Found %d Spotify links", len(trackURLs))

	if len(trackURLs) == 0 {
		log.Println("⚠️ No Spotify links found in the database.")
		return
	}

	// Step 3: Extract track IDs
	var trackIDs []spotify.ID
	for _, url := range trackURLs {
		id, err := extractSpotifyTrackID(url)
		if err != nil {
			log.Printf("⚠️ Skipping invalid link: %s", url)
			continue
		}
		trackIDs = append(trackIDs, id)
	}
	if len(trackIDs) == 0 {
		log.Println("⚠️ No valid Spotify track IDs found.")
		return
	}

	// Step 4: Create playlist
	playlist, err := client.CreatePlaylistForUser(userID, "Wiggly O2 Legacy", "Telegram Spotify links", true)
	if err != nil {
		log.Fatalf("❌ Failed to create playlist: %v", err)
	}
	log.Printf("✅ Created playlist: %s", playlist.Name)

	// Step 5: Add tracks in batches of 100
	const batchSize = 100
	for i := 0; i < len(trackIDs); i += batchSize {
		end := i + batchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]

		for attempt := 1; attempt <= 3; attempt++ {
			_, err := client.AddTracksToPlaylist(playlist.ID, batch...)
			if err == nil {
				log.Printf("🎶 Added batch %d–%d", i+1, end)
				break
			}
			log.Printf("⚠️ Retry %d: failed to add batch %d–%d: %v", attempt, i+1, end, err)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		time.Sleep(1 * time.Second) // simple rate limit
	}

	log.Println("🎉 All tracks added to playlist.")
}

// extractSpotifyTrackID extracts the Spotify track ID from a link.
func extractSpotifyTrackID(link string) (spotify.ID, error) {
	const prefix = "https://open.spotify.com/track/"
	if !strings.HasPrefix(link, prefix) {
		return "", fmt.Errorf("invalid Spotify track link")
	}

	idPart := strings.TrimPrefix(link, prefix)
	id := strings.Split(idPart, "?")[0]
	if len(id) != 22 {
		return "", fmt.Errorf("invalid Spotify ID length")
	}
	return spotify.ID(id), nil
}

// WriteSpotifySongsToDB fetches Spotify links from Telegram messages and writes them to DB.
func writeSpotifySongsToDB() {
	_, _ = client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{NewVerbosityLevel: 0})
	db, err := sql.Open("postgres", dbURI)
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

const (
	apiId   = 25681228
	apiHash = "854f4a044d9593bb5187085cf347ae5d"
	chatId  = -1001322114329
)

func extractSpotifyLinks(text string) []string {
	return spotifyLinkRegex.FindAllString(text, -1)
}
