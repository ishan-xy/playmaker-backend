package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/gofiber/fiber/v3"
)

type User struct {
	ID    string `json:"id"`
	TOKEN string `json:"token"`
}

func extractID(jsonresponse []byte) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonresponse, &data); err != nil {
		return "", utils.WithStack(err)
	}
	if id, ok := data["id"].(string); ok {
		return id, nil
	}
	return "", utils.WithStack(fiber.NewError(fiber.StatusInternalServerError, "ID not found in response"))
}

func IsAuthenticated(c fiber.Ctx) error {
    authHeader := c.Get("Authorization")

    if authHeader == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Missing Authorization Header",
        })
    }

    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Invalid Authorization Header format",
        })
    }
    token := parts[1]

	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/me", nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}

	req.Header.Add("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}
	defer resp.Body.Close() // Move defer to right after getting response

	if resp.StatusCode != http.StatusOK {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	// Fixed body reading - use io.ReadAll instead
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}

	id, err := extractID(bodyBytes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": utils.WithStack(err),
		})
	}
	user := User{
		ID:    id,
		TOKEN: token,
	}

    c.Locals("user", user)

    return c.Next()
}