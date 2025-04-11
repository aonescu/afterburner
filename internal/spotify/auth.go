package spotify

// import (
// 	"log"
// 	"os"
// 	"github.com/zmb3/spotify"
// 	"github.com/joho/godotenv"
// )

// var (
// 	auth  = spotify.NewAuthenticator("", spotify.ScopePlaylistModifyPrivate, spotify.ScopePlaylistModifyPublic)
// 	state = "abc123"
// )

// func init() {
// 	// Load environment variables from .env file (optional)
// 	_ = godotenv.Load()
// 	auth.SetAuthInfo(os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))
// }

// func CreateSpotifyClient() *spotify.Client {
// 	// The callback handler registration is handled in main.go

// 	// Generate the authorization URL
// 	url := auth.AuthURL(state)
// 	log.Printf("🔗 Please log in to Spotify by visiting the following URL in your browser:\n\n %s", url)

// 	// Wait for callback to finish (done in main)
// 	client := <-ch

// 	// Get current user info
// 	user, err := client.CurrentUser()
// 	if err != nil {
// 		log.Fatalf("❌ Failed to get current user: %v", err)
// 	}
// 	log.Printf("✅ Logged in as: %s (%s)\n", user.ID, user.DisplayName)

// 	return client
// }