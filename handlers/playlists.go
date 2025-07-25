package handlers

import (
	"app/middleware"
	"encoding/json"
	"io"
	"fmt"
	"net/http"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/gofiber/fiber/v3"
)

// PlaylistsResponse represents the main response structure for user playlists
type PlaylistsResponse struct {
	Href     string     `json:"href"`
	Limit    int        `json:"limit"`
	Next     string     `json:"next"`
	Offset   int        `json:"offset"`
	Previous string     `json:"previous"`
	Total    int        `json:"total"`
	Items    []Playlist `json:"items"`
}

// Playlist represents a single playlist item
type Playlist struct {
	Collaborative bool         `json:"collaborative"`
	Description   string       `json:"description"`
	ExternalUrls  ExternalUrls `json:"external_urls"`
	Href          string       `json:"href"`
	ID            string       `json:"id"`
	Images        []Image      `json:"images"`
	Name          string       `json:"name"`
	Owner         Owner        `json:"owner"`
	Public        bool         `json:"public"`
	SnapshotID    string       `json:"snapshot_id"`
	Tracks        Tracks       `json:"tracks"`
	Type          string       `json:"type"`
	URI           string       `json:"uri"`
}

// ExternalUrls represents external URLs (typically Spotify links)
type ExternalUrls struct {
	Spotify string `json:"spotify"`
}

// Image represents an image with dimensions
type Image struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// Owner represents the owner of a show/playlist
type Owner struct {
	ExternalUrls ExternalUrls `json:"external_urls"`
	Href         string       `json:"href"`
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	URI          string       `json:"uri"`
	DisplayName  string       `json:"display_name"`
}

// Tracks represents track information
type Tracks struct {
	Href  string `json:"href"`
	Total int    `json:"total"`
}

// Example usage functions
func parsePlaylistsResponse(jsonData []byte) (*PlaylistsResponse, error) {
	var response PlaylistsResponse
	err := json.Unmarshal(jsonData, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal playlists response: %w", err)
	}
	return &response, nil
}

// extractPlaylistIDs extracts all playlist IDs from the response
func extractPlaylistIDs(response *PlaylistsResponse) []string {
	var ids []string
	for _, playlist := range response.Items {
		ids = append(ids, playlist.ID)
	}
	return ids
}

// GetPlaylistByID finds a playlist by its ID
func GetPlaylistByID(response *PlaylistsResponse, id string) *Playlist {
	for _, playlist := range response.Items {
		if playlist.ID == id {
			return &playlist
		}
	}
	return nil
}


func GetUserPlaylists(c fiber.Ctx) error {
	// Safe token retrieval with proper error handling
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

	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me/playlists", nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}

	// Set the Authorization header with the user's token
	req.Header.Add("Authorization", "Bearer "+user.TOKEN)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch playlists",
		})
	}

	// Fixed body reading - use io.ReadAll
	jsonResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}

	response, err := parsePlaylistsResponse(jsonResponse)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}

	// Return just the array of playlists as expected by the SolidJS frontend
	return c.JSON(response.Items)
}