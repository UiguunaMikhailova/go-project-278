package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func newRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	return router
}

func main() {
	router := newRouter()

	if err := router.Run(":8080"); err != nil {
		panic(err)
	}
}
