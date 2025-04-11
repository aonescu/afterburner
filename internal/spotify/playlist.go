package spotify

import (
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/zmb3/spotify"
)

const playlistName = "Wiggly O2 Legacy"

// CreatePlaylistFromDB fetches Spotify links from the DB and builds a playlist.
func CreatePlaylistFromDB(db *sql.DB, userID string, client *spotify.Client) {
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
	playlist, err := client.CreatePlaylistForUser(userID, playlistName, "Telegram Spotify links", true)
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

func extractSpotifyTrackID(link string) (spotify.ID, error) {
	const prefix = "https://open.spotify.com/track/"
	if !strings.HasPrefix(link, prefix) {
		return "", errors.New("invalid Spotify track link")
	}

	idPart := strings.TrimPrefix(link, prefix)
	id := strings.Split(idPart, "?")[0]
	if len(id) != 22 {
		return "", errors.New("invalid Spotify ID length")
	}
	return spotify.ID(id), nil
}