package router

import (
	"app/handlers"
	"app/middleware"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	utils "github.com/ItsMeSamey/go_utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	fiberRecover "github.com/gofiber/fiber/v3/middleware/recover"
)

func init() {
	defer func() {
		if err := recover(); err != nil {
			log.Fatal(utils.WithStack(errors.New("Error initializing router: " + fmt.Sprint(err))))
		}
	}()
	app := fiber.New(fiber.Config{
		CaseSensitive:      true,
		Concurrency:        1024 * 1024,
		IdleTimeout:        30 * time.Second,
		DisableDefaultDate: true,
		JSONEncoder:        json.Marshal,
		JSONDecoder:        json.Unmarshal,
		BodyLimit:          100 * 1024 * 1024,
	})
    
	app.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
        AllowMethods:     []string{"GET", "POST", "HEAD", "PUT", "DELETE", "PATCH", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
        AllowCredentials: true,
    }))

	app.Use(fiberRecover.New(fiberRecover.Config{EnableStackTrace: true}))
	app.Use(logger.New())
	
	log.Println("Default logging enabled")

	utils.SetErrorStackTrace(true)	

	app.Post("/login", handlers.Login)
	app.Get("/playlists", middleware.IsAuthenticated, handlers.GetUserPlaylists)
	app.Post("/playlist/create", middleware.IsAuthenticated, handlers.CreatePlaylist)
	app.Post("/playlist/modify", middleware.IsAuthenticated, handlers.ModifyPlaylist)
	app.Get("/test", func(c fiber.Ctx) error {
        log.Println("TEST ROUTE CALLED!")
        fmt.Println("TEST ROUTE CALLED WITH FMT!")
        return c.SendString("Test route works")
    })
	// Start the server
	log.Fatal(
		app.Listen(":8080", fiber.ListenConfig{
			EnablePrintRoutes: true,
		}),
	)
	log.Println("Routes initialized successfully")
}