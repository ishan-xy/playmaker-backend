package handlers

import (
	"app/config"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/gofiber/fiber/v3"
)

// CODE struct to bind the incoming request body
type CODE struct {
	Code string `json:"code"`
}

// TokenResponse struct to unmarshal the JSON response from Spotify
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func Login(c fiber.Ctx) error {
	var req CODE
	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   utils.WithStack(err),
			"message": "Invalid request body",
		})
	}
	if req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Code is required",
			"message": "Please provide a valid code",
		})
	}

	// --- Exchange Code for Access Token ---

	// 1. Get credentials from environment variables for security
	clientID := config.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := config.Getenv("SPOTIFY_CLIENT_SECRET")
	redirectURI := config.Getenv("SPOTIFY_REDIRECT_URI") // e.g., "http://127.0.0.1:3000/callback"

	// 2. Prepare the request body for Spotify's token endpoint
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", req.Code)
	data.Set("redirect_uri", redirectURI)

	// 3. Create the HTTP request
	tokenURL := "https://accounts.spotify.com/api/token"
	r, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create request"})
	}

	// 4. Set the required headers, including the Authorization header
	authHeader := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	r.Header.Add("Authorization", "Basic "+authHeader)
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// 5. Execute the request
	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get token from Spotify"})
	}
	defer resp.Body.Close()

	// 6. Check for non-200 responses from Spotify
	if resp.StatusCode != http.StatusOK {
		// You can add more detailed error handling here by reading resp.Body
		return c.Status(resp.StatusCode).JSON(fiber.Map{"error": "Spotify returned an error"})
	}

	// 7. Decode the successful JSON response into our struct
	var tokenResponse TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to decode Spotify response"})
	}

	// 8. Return the tokens to the frontend
	return c.Status(fiber.StatusOK).JSON(tokenResponse)
}