package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	appauth "memory-match/internal/auth"
	"memory-match/internal/game"
	"memory-match/internal/handlers"
	"memory-match/internal/store"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	db, err := store.ConnectDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(context.Background(), db); err != nil {
		log.Fatal(err)
	}

	log.Println("PostgreSQL connected successfully")

	gameService := game.NewService(db)
	gameHandler := handlers.NewGameHandler(gameService)

	baseURL := envOr("BASE_URL", "http://localhost:8082")
	clientID := requireEnv("GOOGLE_CLIENT_ID")
	clientSecret := requireEnv("GOOGLE_CLIENT_SECRET")
	sessionSecret := requireEnv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be at least 32 characters")
	}
	secureCookies, err := strconv.ParseBool(envOr("COOKIE_SECURE", strconv.FormatBool(strings.HasPrefix(baseURL, "https://"))))
	if err != nil {
		log.Fatal("COOKIE_SECURE must be true or false")
	}
	oauthConfig := oauth2.Config{
		ClientID: clientID, ClientSecret: clientSecret,
		Endpoint: google.Endpoint, Scopes: []string{"openid", "email", "profile"},
	}
	authService := appauth.NewService(db, oauthConfig, appauth.NewGoogleVerifier(context.Background(), clientID), baseURL, secureCookies)
	authHandler := handlers.NewAuthHandler(authService, []byte(sessionSecret), secureCookies)

	router := gin.Default()
	router.Use(func(c *gin.Context) {
		c.Header("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=86400")
		} else if c.Request.URL.Path == "/app.js" || c.Request.URL.Path == "/styles.css" {
			c.Header("Cache-Control", "public, max-age=300")
		} else {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "TEDx MindStage backend is running",
		})
	})

	router.Static("/assets", "web/assets")
	router.StaticFile("/styles.css", "web/styles.css")
	router.StaticFile("/app.js", "web/app.js")
	router.GET("/auth/google", handlers.NewRateLimitMiddleware(120, time.Minute), authHandler.BeginGoogle)
	router.GET("/auth/google/callback", authHandler.GoogleCallback)
	router.POST("/auth/logout", authHandler.Logout)
	router.GET("/me", authHandler.RequirePlayer(), authHandler.Me)
	router.PUT("/me/username", authHandler.RequirePlayer(), authHandler.UpdateUsername)
	gameLimit := handlers.NewRateLimitMiddleware(360, time.Minute)
	protected := router.Group("")
	protected.Use(authHandler.RequirePlayer(), gameLimit)
	protected.POST("/games", gameHandler.CreateGame)
	protected.GET("/games/:id", gameHandler.GetGame)
	protected.POST("/games/:id/flip", gameHandler.FlipCard)
	protected.POST("/games/:id/resolve", gameHandler.ResolveMismatch)
	router.GET("/leaderboard", handlers.NewRateLimitMiddleware(120, time.Minute), gameHandler.GetLeaderboard)
	router.GET("/", func(c *gin.Context) { c.File("web/index.html") })
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func requireEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s environment variable is not set", name)
	}
	return value
}
