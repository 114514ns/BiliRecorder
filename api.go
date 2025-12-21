package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var tasks = make(map[string]bool)

var taskMutex sync.Mutex

var taskPool = NewPool(1)

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

	r.GET("/list", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": m,
		})
	})

	r.GET("/convert", func(c *gin.Context) {
		var link = c.Query("link")
		_, e := url.Parse(link)

		if e != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": e.Error(),
			})
			return
		}
		taskMutex.Lock()

		_, ok := tasks[link]

		taskMutex.Unlock()

		if ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"msg": "task is running",
			})
			return
		}

		taskMutex.Lock()

		tasks[link] = true

		taskMutex.Unlock()

		c.JSON(http.StatusOK, gin.H{
			"msg": "submit",
		})

		taskPool.Submit(func() {
			split := strings.Split(link, "/")

			_, e = client.R().SetOutput(split[len(split)-1]).Get(link)

			if e != nil {
				log.Println(e)
				return
			}

			var fName = split[len(split)-1]

			cmd := exec.Command("ffmpeg", "-i", fName, "-vcodec", "copy", "-acodec", "copy", strings.Replace(fName, ".mp4", "-COVERT.mp4", 1))
			//out, _ := cmd.CombinedOutput()
			//log.Println(string(out))
			e = cmd.Run()

			if e != nil {
				log.Println(e)
				return
			} else {
				defer func() {
					os.Remove(strings.Replace(fName, ".mp4", "-COVERT.mp4", 1))
					os.Remove(split[len(split)-1])
				}()
			}

			var d = "/"
			for i, s := range split {
				if i >= 4 {
					d = d + s
				}
			}
			open, e := os.Open(strings.Replace(fName, ".mp4", "-COVERT.mp4", 1))

			if e != nil {
				log.Println(e)
				return
			} else {

			}

			var off int64 = 0

			stat, e := open.Stat()

			if e != nil {
				log.Println(e)
				return
			}

			var CHUNK int64 = 1024 * 1024 * 100

			u := oneDriveCreate(oneDrive, oneDriveMkDir(oneDrive, oneDriveMkDir(oneDrive, oneDrive.RootID, split[5]), split[6]), strings.Replace(fName, ".mp4", "-COVERT.mp4", 1))

			for {
				var dst = make([]byte, CHUNK)

				n, _ := open.ReadAt(dst, off)

				log.Println(off)

				if len(dst[0:n]) == 0 {
					break
				}

				if len(dst[0:n]) != len(dst) {
					CHUNK--
				}

				oneDriveUpload(oneDrive, off, off+CHUNK, stat.Size(), u, dst[0:n])
				off = off + CHUNK
			}
		})

	})

	r.Run(":" + toString(int64(config.Port)))
}
