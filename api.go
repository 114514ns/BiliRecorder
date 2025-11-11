package main

import "github.com/gin-gonic/gin"

func InitHTTP() {
	r := gin.Default()
	r.GET("/add", func(c *gin.Context) {
		var uid = toInt64(c.Query("uid"))
		if (uid) <= 0 {
			c.JSON(400, gin.H{
				"msg": "bad request",
			})
			return
		}
		var has = false
		for i := range config.Livers {
			if uid == config.Livers[i] {
				has = true
			}
		}
		if !has {
			config.Livers = append(config.Livers, uid)
			c.JSON(200, gin.H{
				"msg": "ok",
			})
			saveConfig()
		} else {
			c.JSON(400, gin.H{
				"msg": "already exists",
			})
		}
	})

	r.GET("/lst", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": m,
		})
	})

	r.Run(":8084")
}
