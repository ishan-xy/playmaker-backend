package handlers

import (
	"app/middleware"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v3"
)

func ModifyPlaylist(c fiber.Ctx) error {
    // Get user info and token
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
    if user.TOKEN == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "User not authenticated - empty token",
        })
    }

    // Parse input JSON with playlist_id and artist_url
    var req struct {
        PlaylistID string `json:"playlist_id"`
        ArtistURL  string `json:"artist_url"`
    }
    if err := c.Bind().Body(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
    }
    if req.PlaylistID == "" || req.ArtistURL == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "playlist_id and artist_url are required"})
    }

    artistID := extractArtistID(req.ArtistURL)
    if artistID == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid artist URL"})
    }

    // 1. Fetch all tracks from the artist (filtered by artist)
    artistTracks, err := getCachedArtistTracks(artistID, user.TOKEN)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": fmt.Sprintf("Failed to fetch artist tracks: %v", err),
        })
    }
    if len(artistTracks) == 0 {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
            "error": "No tracks found for this artist",
        })
    }

    // 2. Fetch all existing tracks in the playlist (Spotify playlists can be paginated)
    playlistTracks, err := getPlaylistTracks(req.PlaylistID, user.TOKEN)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": fmt.Sprintf("Failed to fetch playlist tracks: %v", err),
        })
    }

    // 3. Build sets of existing track IDs and map artist tracks by ID
    existingTrackIDs := make(map[string]struct{})
    for _, pt := range playlistTracks {
        existingTrackIDs[pt.ID] = struct{}{}
    }

    missingURIs := make([]string, 0)
    missingNames := make([]string, 0)
    for _, at := range artistTracks {
        if _, found := existingTrackIDs[at.ID]; !found {
            missingURIs = append(missingURIs, at.URI)
            missingNames = append(missingNames, at.Name)
        }
    }

    if len(missingURIs) == 0 {
        return c.Status(fiber.StatusOK).JSON(fiber.Map{
            "message": "All artist tracks are already in the playlist",
            "added_count": 0,
        })
    }

    // 4. Add missing tracks in batches of 100
    const batchSize = 100
    for i := 0; i < len(missingURIs); i += batchSize {
        end := i + batchSize
        if end > len(missingURIs) {
            end = len(missingURIs)
        }
        batch := missingURIs[i:end]
        if err := addTracksToPlaylist(req.PlaylistID, batch, user.TOKEN); err != nil {
            return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
                "error": fmt.Sprintf("Failed to add tracks to playlist: %v", err),
            })
        }
        // Optional: log progress
        for j := i; j < end; j++ {
            log.Printf("🟢 Added missing track: %s", missingNames[j])
        }
    }

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "message":     fmt.Sprintf("Added %d missing artist tracks to playlist %s", len(missingURIs), req.PlaylistID),
        "added_count": len(missingURIs),
    })
}


func getPlaylistTracks(playlistID, token string) ([]SimplifiedTrack, error) {
    type PlaylistTracksResponse struct {
        Items []struct {
            Track SimplifiedTrack `json:"track"`
        } `json:"items"`
        Next string `json:"next"`
    }

    var tracks []SimplifiedTrack
    nextURL := fmt.Sprintf("%s/v1/playlists/%s/tracks?limit=100", spotifyApiURL, playlistID)

    for nextURL != "" {
        var response PlaylistTracksResponse
        err := makeAPIRequest(nextURL, token, &response)
        if err != nil {
            return nil, err
        }
        for _, item := range response.Items {
            if item.Track.ID != "" {
                tracks = append(tracks, item.Track)
            }
        }
        nextURL = response.Next
    }
    return tracks, nil
}
