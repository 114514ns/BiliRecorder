package main

import (
	"encoding/json"
	"fmt"
	"io"
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

	r.GET("/list", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"data": m,
		})
	})

	r.POST("/convert", func(c *gin.Context) {
		var oneDrive *OneDriveStorageConfig
		for _, i := range config.Storages {
			if getString(i, "Type") == "onedrive" {
				oneDrive = (getDstByLabel(getString(i, "Label")).(*OneDriveStorageConfig))
			}
		}

		var link = c.Query("link")

		var fn = c.Query("fName")

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

		var b = []byte(c.PostForm("mapping")) //分块索引

		var dirId = c.PostForm("dir") //要保存到的目录id

		var itemId = c.PostForm("id") //原始的文件id，成功转封装后自动删除

		c.JSON(http.StatusOK, gin.H{
			"msg": "submit",
		})

		taskPool.Submit(func() {
			var metaRes []byte
			if strings.Contains(link, "sharepoint.com/") {
				metaRes = b
			} else {
				v, _ := client.R().Get(strings.Replace(link, ".mp4", ".json", 1))
				metaRes = v.Body()
			}

			var rangeHeader = "bytes=0-"
			var meta []string
			e := json.Unmarshal(metaRes, &meta)
			if e != nil {

			} else {
				rangeHeader = rangeHeader + strings.Split(meta[len(meta)-1], ",")[1]
			}
			//link = meta[0]
			split := strings.Split(link, "/")

			var fName = split[len(split)-1]

			if fn != "" {
				fName = fn
			}

			res, e := client.R().SetHeader("Range", rangeHeader).SetDoNotParseResponse(true).Get(link)

			defer res.RawBody().Close()
			out, _ := os.Create(fName)
			defer out.Close()
			_, err = io.Copy(out, res.RawBody())

			if e != nil {
				log.Println(e)
				return
			}

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
					os.Remove(fName)
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
			}

			if dirId == "" {
				dirId = oneDriveMkDir(oneDrive, oneDriveMkDir(oneDrive, oneDrive.RootID, split[5]), split[6])
			}

			stat, e := os.Stat(open.Name())

			var u = oneDriveCreate(oneDrive, dirId, strings.Replace(fName, ".mp4", "-CONVERT.mp4", 1))

			var from int64 = 0
			var CHUNK_SIZE int64 = 1024 * 1024 * 100 // 建议先用较小值测试，比如 10MB

			for {
				to := from + CHUNK_SIZE
				if to > stat.Size() {
					to = stat.Size()
				}
				chunkLen := to - from
				section := io.NewSectionReader(open, from, chunkLen)

				req, _ := http.NewRequest("PUT", u, section)
				//req.Header.Set("Authorization", oneDrive.AccessToken)

				req.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to-1, stat.Size()))

				req.ContentLength = to - from

				h := http.Client{}
				res0, e0 := h.Do(req)
				if e0 != nil {
					log.Println("请求失败:", e0)
					break
				}

				respBytes, _ := io.ReadAll(res0.Body)
				res0.Body.Close()
				log.Println("服务器返回:", string(respBytes))

				if res0.StatusCode == 200 || res0.StatusCode == 201 {
					oneDriveDelete(oneDrive, itemId)
					break
				} else if res0.StatusCode == 202 {
					from = to
				} else {
					log.Printf("上传状态异常: %d\n", res0.StatusCode)
					break
				}

				if to >= stat.Size() {
					break
				}
			}

		})

	})

	r.Run(":" + toString(int64(config.Port)))
}
