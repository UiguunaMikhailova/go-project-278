package main

import (
	"log"
	"net/http"
	"os"
	"time"

	sentry "github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func loadEnv() {
	err := godotenv.Load()
	if err == nil {
		log.Println("переменные загружены из .env")
		return
	}

	if !os.IsNotExist(err) {
		log.Printf("не удалось прочитать .env: %v", err)
	}
}

func initSentry() bool {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		log.Println("SENTRY_DSN не задан, мониторинг ошибок выключен")
		return false
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: os.Getenv("APP_ENV"),
		SendDefaultPII: true,
	})
	if err != nil {
		log.Printf("не удалось подключить мониторинг ошибок: %v", err)
		return false
	}

	log.Println("мониторинг ошибок подключен")

	return true
}

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(sentrygin.New(sentrygin.Options{Repanic: true}))

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	router.GET("/debug/sentry", func(_ *gin.Context) {
		panic("тестовая ошибка для проверки мониторинга")
	})

	return router
}

func main() {
	loadEnv()

	if initSentry() {
		defer sentry.Flush(2 * time.Second)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := newRouter()

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("не удалось запустить сервер: %v", err)
	}
}
