package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"app/config"
	"app/middleware"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/gofiber/fiber/v3"
)

const (
    spotifyApiURL = "https://api.spotify.com"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type CreatePlaylistRequest struct {
    Name      string `json:"name"`
    ArtistURL string `json:"artist_url"`
}

/* --------- Structs for Spotify API ---------- */
type CreatePlaylistBody struct {
    Name   string `json:"name"`
    Public bool   `json:"public"`
}
type CreatePlaylistResponse struct {
    ID   string `json:"id"`
    URI  string `json:"uri"`
    Name string `json:"name"`
}
type AddTracksBody struct {
    URIs []string `json:"uris"`
}
type SimplifiedAlbum struct {
    ID          string `json:"id"`
    ReleaseDate string `json:"release_date"`
}
type ArtistAlbumsResponse struct {
    Items []SimplifiedAlbum `json:"items"`
    Next  string            `json:"next"`
}
type TrackArtist struct {
    ID string `json:"id"`
}
type SimplifiedTrack struct {
    Name    string        `json:"name"`
    ID      string        `json:"id"`
    URI     string        `json:"uri"`
    AlbumID string        `json:"album_id"`
    Artists []TrackArtist `json:"artists"`
}
type AlbumTracksResponse struct {
    Items []SimplifiedTrack `json:"items"`
    Next  string            `json:"next"`
}

type TrackWithDate struct {
    Track       SimplifiedTrack
    ReleaseDate string
}

/* ------------ Utility Functions ------------ */

func extractArtistID(artistURL string) string {
    parts := strings.Split(artistURL, "/")
    for i, part := range parts {
        if part == "artist" && i+1 < len(parts) {
            return parts[i+1]
        }
    }
    return ""
}

/* ---------------- MAIN HANDLER ----------------- */
func CreatePlaylist(c fiber.Ctx) error {
    userInterface := c.Locals("user")
    if userInterface == nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "User not authenticated - no token",
        })
    }
    user, ok := userInterface.(middleware.User)
    if !ok {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Invalid token data type",
        })
    }
    if user.TOKEN == "" || user.ID == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "User not authenticated - empty token",
        })
    }

    var req CreatePlaylistRequest
    if err := c.Bind().Body(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
    }
    artistID := extractArtistID(req.ArtistURL)
    if artistID == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid artist URL"})
    }

    // 1. Fetch all tracks for the artist (filtered by artist), with album IDs
    tracks, err := getCachedArtistTracks(artistID, user.TOKEN)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": utils.WithStack(err),
        })
    }
    if len(tracks) == 0 {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "No tracks found for this artist",
        })
    }

    // 2. Fetch album release dates for sorting
    albumReleaseDates, err := getArtistAlbumsReleaseDates(artistID, user.TOKEN)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to get album release dates",
        })
    }

    // 3. Compose slice of TrackWithDate to sort by release date
    sortableTracks := make([]TrackWithDate, 0, len(tracks))
    for _, t := range tracks {
        releaseDate := albumReleaseDates[t.AlbumID]
        sortableTracks = append(sortableTracks, TrackWithDate{
            Track:       t,
            ReleaseDate: releaseDate,
        })
    }

    // 4. Sort tracks by release date (oldest first)
    sort.Slice(sortableTracks, func(i, j int) bool {
        return sortableTracks[i].ReleaseDate > sortableTracks[j].ReleaseDate
    })

    // 5. Extract URIs in sorted order
    var uris []string
    for _, td := range sortableTracks {
        uris = append(uris, td.Track.URI)
    }

    // 6. Create the playlist on user's account
    playlistID, err := createPlaylist(user.ID, req.Name, user.TOKEN)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": fmt.Sprintf("Failed to create playlist: %v", err),
        })
    }
    log.Printf("🎼 Playlist \"%s\" created. Adding songs…", req.Name)

    // 7. Add tracks in batches of 100, with progress logs
    const batchSize = 100
    for i := 0; i < len(uris); i += batchSize {
        end := i + batchSize
        if end > len(uris) {
            end = len(uris)
        }
        batch := uris[i:end]
        if err := addTracksToPlaylist(playlistID, batch, user.TOKEN); err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("Failed to add tracks: %v", err),
            })
        }
        for j := i; j < end; j++ {
            log.Printf("🟢 Added: %s\n", sortableTracks[j].Track.Name)
        }
    }

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "message":  fmt.Sprintf("Playlist '%s' created and %d tracks added.", req.Name, len(uris)),
        "playlist": playlistID,
        "count":    len(uris),
    })
}

/* -------------- Helper functions -------------- */

func getArtistAlbumsReleaseDates(artistID, token string) (map[string]string, error) {
    albumReleaseDates := make(map[string]string)
    nextURL := fmt.Sprintf("%s/v1/artists/%s/albums?include_groups=album,single&limit=50", spotifyApiURL, artistID)
    for nextURL != "" {
        var albumsResponse ArtistAlbumsResponse
        err := makeAPIRequest(nextURL, token, &albumsResponse)
        if err != nil {
            return nil, err
        }
        for _, album := range albumsResponse.Items {
            albumReleaseDates[album.ID] = album.ReleaseDate
        }
        nextURL = albumsResponse.Next
    }
    return albumReleaseDates, nil
}

func getAllArtistTracksWithAlbumID(artistID, token string) ([]SimplifiedTrack, error) {
    albumIDs, err := getArtistAlbumIDs(artistID, token)
    if err != nil {
        return nil, err
    }
    uniqueTracks := make(map[string]SimplifiedTrack)
    for _, albumID := range albumIDs {
        tracks, err := getAlbumTracksWithAlbumID(albumID, token, artistID)
        if err != nil {
            log.Printf("⚠️ Could not fetch tracks for album %s: %v", albumID, err)
            continue
        }
        for _, track := range tracks {
            if track.ID != "" {
                uniqueTracks[track.ID] = track
            }
        }
    }
    result := make([]SimplifiedTrack, 0, len(uniqueTracks))
    for _, track := range uniqueTracks {
        result = append(result, track)
    }
    return result, nil
}

func getArtistAlbumIDs(artistID, token string) ([]string, error) {
    var albumIDs []string
    nextURL := fmt.Sprintf("%s/v1/artists/%s/albums?include_groups=album,single&limit=50", spotifyApiURL, artistID)
    for nextURL != "" {
        var albumsResponse ArtistAlbumsResponse
        err := makeAPIRequest(nextURL, token, &albumsResponse)
        if err != nil {
            return nil, err
        }
        for _, album := range albumsResponse.Items {
            albumIDs = append(albumIDs, album.ID)
        }
        nextURL = albumsResponse.Next
    }
    return albumIDs, nil
}

func getAlbumTracksWithAlbumID(albumID, token, targetArtistID string) ([]SimplifiedTrack, error) {
    var tracks []SimplifiedTrack
    nextURL := fmt.Sprintf("%s/v1/albums/%s/tracks?limit=50", spotifyApiURL, albumID)

    for nextURL != "" {
        var tracksResponse AlbumTracksResponse
        err := makeAPIRequest(nextURL, token, &tracksResponse)
        if err != nil {
            return nil, err
        }
        for _, track := range tracksResponse.Items {
            track.AlbumID = albumID

            // Include track only if targetArtistID exists in track.Artists
            containsArtist := false
            for _, artist := range track.Artists {
                if artist.ID == targetArtistID {
                    containsArtist = true
                    break
                }
            }
            if containsArtist {
                tracks = append(tracks, track)
            }
        }
        nextURL = tracksResponse.Next
    }
    return tracks, nil
}

func createPlaylist(userID string, name string, token string) (string, error) {
    url := fmt.Sprintf("%s/v1/users/%s/playlists", spotifyApiURL, userID)
    body := CreatePlaylistBody{Name: name, Public: false}
    b, _ := json.Marshal(body)

    req, err := http.NewRequest("POST", url, strings.NewReader(string(b)))
    if err != nil {
        return "", err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        raw, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("failed to create playlist: %s", string(raw))
    }
    var playlistRes CreatePlaylistResponse
    if err := json.NewDecoder(resp.Body).Decode(&playlistRes); err != nil {
        return "", err
    }
    return playlistRes.ID, nil
}

func addTracksToPlaylist(playlistID string, uris []string, token string) error {
    url := fmt.Sprintf("%s/v1/playlists/%s/tracks", spotifyApiURL, playlistID)
    body := AddTracksBody{URIs: uris}
    b, _ := json.Marshal(body)

    req, err := http.NewRequest("POST", url, strings.NewReader(string(b)))
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        raw, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("failed to add tracks: %s", string(raw))
    }
    return nil
}

func makeAPIRequest(url, token string, target interface{}) error {
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    resp, err := httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
    }
    return json.NewDecoder(resp.Body).Decode(target)
}


// Caches as JSON under key "artist_tracks:{artistID}" with a TTL (e.g., 6h).
func getCachedArtistTracks(artistID, token string) ([]SimplifiedTrack, error) {
    ctx := context.Background()
    cacheKey := fmt.Sprintf("artist_tracks:%s", artistID)
    client := config.RedisClient

    // 1. Try to read from cache.
    result, err := client.Get(ctx, cacheKey).Result()
    if err == nil {
        var tracks []SimplifiedTrack
        if err := json.Unmarshal([]byte(result), &tracks); err == nil {
            log.Printf("🔍 Redis cache hit for artist %s (%d tracks)", artistID, len(tracks))
            return tracks, nil
        }
        // If decode error, fall through and refill cache.
        log.Printf("⚠️ Redis cache for artist %s is corrupt, refetching", artistID)
    }

    // 2. Cache miss or decode problem: Fetch from Spotify and cache result
    log.Printf("🚀 Redis cache miss for artist %s, fetching tracks", artistID)
    tracks, err := getAllArtistTracksWithAlbumID(artistID, token)
    if err != nil {
        return nil, err
    }

    // Marshal and save to redis with a TTL (e.g. 6 hours)
    data, err := json.Marshal(tracks)
    if err == nil {
        _ = client.Set(ctx, cacheKey, data, 6*time.Hour).Err()
        log.Printf("💾 Saved %d tracks to Redis for artist %s", len(tracks), artistID)
    }
    return tracks, nil
}


func clearArtistCache(artistID string) {
    ctx := context.Background()
    cacheKey := fmt.Sprintf("artist_tracks:%s", artistID)
    if config.RedisClient != nil {
        config.RedisClient.Del(ctx, cacheKey)
        log.Printf("❌ Cleared Redis cache for artist %s", artistID)
    }
}
