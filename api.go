package main

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var tasks = make(map[string]bool)

var taskMutex sync.Mutex

var taskPool = NewPool(1)

func InitHTTP() {
	r := gin.Default()
	r.POST("/add", func(c *gin.Context) {
		var uid = toInt64(c.Query("uid"))
		if (uid) <= 0 {
			c.JSON(400, gin.H{
				"msg": "bad request",
			})
			return
		}
		var has = false
		for i := range config.Livers {
			if uid == config.Livers[i].UID {
				has = true
			}
		}
		if !has {
			config.Livers = append(config.Livers, Liver{
				UID: uid,
			})
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

	r.POST("/once", func(context *gin.Context) {
		//biliClient.BatchGetFace()
	})

	r.GET("/list", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": m,
		})
	})

	r.Run(":" + toString(int64(config.Port)))
}
