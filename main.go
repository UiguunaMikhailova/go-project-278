package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	sentry "github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

const (
	defaultBaseURL       = "http://localhost:8080"
	defaultAllowedOrigin = "http://localhost:5173"
)

func loadEnv() {
	err := godotenv.Load()
	if err == nil {
		log.Println("environment variables loaded from .env")
		return
	}

	if !os.IsNotExist(err) {
		log.Printf("failed to read .env: %v", err)
	}
}

func initSentry() bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Println("SENTRY_DSN is not set, error monitoring is disabled")
		return false
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:            dsn,
		Environment:    os.Getenv("APP_ENV"),
		SendDefaultPII: true,
	})
	if err != nil {
		log.Printf("failed to enable error monitoring: %v", err)
		return false
	}

	log.Println("error monitoring enabled")

	return true
}

// openDB открывает пул соединений и сразу проверяет, что база доступна.
func openDB(databaseURL string) (*sql.DB, error) {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}

	return database, nil
}

// baseURL - адрес, из которого собирается короткая ссылка. Между окружениями он разный.
func baseURL() string {
	if url := os.Getenv("BASE_URL"); url != "" {
		return url
	}

	return defaultBaseURL
}

func allowedOrigins() []string {
	raw := os.Getenv("CORS_ORIGINS")
	if raw == "" {
		return []string{defaultAllowedOrigin}
	}

	origins := strings.Split(raw, ",")
	for i, origin := range origins {
		origins[i] = strings.TrimSpace(origin)
	}

	return origins
}

func registerValidation() {
	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	engine.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}

		return name
	})
}

func newRouter(links *LinksHandler) *gin.Engine {
	registerValidation()

	router := gin.New()
	router.TrustedPlatform = gin.PlatformCloudflare

	router.Use(gin.Logger(), gin.Recovery())
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))

	router.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins(),
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut,
			http.MethodDelete, http.MethodOptions,
		},
		AllowHeaders:  []string{"Origin", "Content-Type", "Accept"},
		ExposeHeaders: []string{"Content-Range", "Accept-Ranges"},
		MaxAge:        12 * time.Hour,
	}))

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	router.GET("/debug/sentry", func(_ *gin.Context) {
		panic("test error for monitoring check")
	})

	router.GET("/r/:code", links.Redirect)

	api := router.Group("/api/links")
	{
		api.GET("", links.List)
		api.POST("", links.Create)
		api.GET("/:id", links.Get)
		api.PUT("/:id", links.Update)
		api.DELETE("/:id", links.Delete)
	}

	router.GET("/api/link_visits", links.ListVisits)

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	return router
}

func main() {
	loadEnv()

	if initSentry() {
		defer sentry.Flush(2 * time.Second)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	database, err := openDB(databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = database.Close() }()

	log.Println("database connection established")

	handler := NewLinksHandler(NewLinkService(database), baseURL())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if err := newRouter(handler).Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
