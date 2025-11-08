package main

import "github.com/gin-gonic/gin"

func InitHTTP() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {

	})
}
