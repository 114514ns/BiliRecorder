package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/114514ns/BiliClient"
	"github.com/bytedance/sonic"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

type RoomConfig struct {
	OnlyAudio       bool   //仅音频
	ReEncoding      bool   //重新编码
	Encoder         string //编码器
	PreferEncoding  string //从b站首选的流
	EncodeChunkTime int    //每块到多大的时候开始编码
	KeepTemp        bool
}

func GetLiveStream(client *bili.BiliClient, room int) string {
	res, _ := client.Resty.R().Get(fmt.Sprintf("https://api.live.bilibili.com/room/v1/Room/playUrl?cid=%d&qn=10000&platform=h5", room))
	var obj = make(map[string]interface{})
	sonic.Unmarshal(res.Body(), &obj)

	return obj["data"].(map[string]interface{})["durl"].([]interface{})[0].(map[string]interface{})["url"].(string)
}
func TraceStream(client *bili.BiliClient, room int, dst0 string, config RoomConfig) {
	var count = 0
	var stream = GetLiveStream(client, room)
	log.Println("Stream: " + stream)
	_, err := os.Stat("temp")
	if os.IsNotExist(err) {
		os.Mkdir("temp", os.ModePerm)
	}
	var ticker = time.NewTicker(time.Second * 2)
	var dst, _ = os.Create(dst0)
	writer := bufio.NewWriter(dst)
	var m = make(map[string]bool)
	//var m = make(map[string]bool)
	var u, _ = url.Parse(stream)
	w := bufio.NewWriter(dst)
	defer ticker.Stop()
	done := make(chan bool, 1)
	sigs := make(chan os.Signal, 1)
	go func() {
		<-sigs
		if config.ReEncoding { //按下ctrl-c的时候，如果启用了重新编码，就合并所有分片
			var content = ""
			dir, _ := ioutil.ReadDir("temp")
			for _, info := range dir {
				var name = "temp/" + info.Name()
				if strings.Contains(name, "-code.mp4") {
					content += fmt.Sprintf("file '%s'\n", name)
				}
			}
			os.WriteFile("list.txt", []byte(content), 0644)
			cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", "list.txt", "-c", "copy", dst0)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
		os.Exit(0)
	}()
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			str, err := client.Resty.R().Get(stream)
			if err != nil {
				if config.ReEncoding {
					//直播结束，如果启用了重新编码，就合并所有分片
					fmt.Println(err)
					dir, _ := ioutil.ReadDir("temp")
					var content = ""
					for _, info := range dir {
						var name = "temp/" + info.Name()
						if strings.Contains(name, "-code.mp4") {
							content += fmt.Sprintf("file '%s'\n", name)
						}
					}
					os.WriteFile("list.txt", []byte(content), 0644)
					cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", "list.txt", "-c", "copy", dst0)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()
					done <- true
				}

			}
			for _, s := range strings.Split(str.String(), "\n") {
				if !strings.HasPrefix(s, "#") {
					_, ok := m[s]
					if !ok {
						path := u.Path
						split := strings.Split(path, "/")
						var d = ""
						for i, s2 := range split {
							if i != len(split)-1 {
								d += s2 + "/"
							}
						}
						r, _ := client.Resty.R().Get("https://" + u.Host + d + s)
						count++
						if config.ReEncoding {
							if count%config.EncodeChunkTime == 0 {
								go func() {
									command := exec.Command("ffmpeg", "-i", "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+".mp4", "-g", "60", "-vcodec", config.Encoder /*"-b:v", "5000k", */, "-c:a", "copy", "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+"-code.mp4")
									//command.Stderr = os.Stderr
									//command.Stdout = os.Stdout
									command.Run()
									if !config.KeepTemp {
										os.Remove("temp/" + strconv.Itoa(count/config.EncodeChunkTime-1) + ".mp4")
									}
								}()
							}
							var chunk = count / config.EncodeChunkTime
							name := "temp/" + strconv.Itoa(chunk) + ".mp4"
							_, err := os.Stat(name)
							if err != nil {
								os.Create(name)
								var f, _ = os.Create(name)
								w = bufio.NewWriter(f)
							}
							w.Write(r.Body())
							w.Flush()

						} else {
							//没有启用重新编码，直接写入目标文件
							writer.Write(r.Body())
							writer.Flush()
						}

						m[s] = true
					}
				}
			}
		}
	}
}

func main() {
	//ffmpeg is required
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	file, _ := os.ReadFile("cookie.txt")
	var client = bili.NewClient(string(file), bili.ClientOptions{})
	TraceStream(client, 2058234, "out.mp4", RoomConfig{
		KeepTemp:   false,
		ReEncoding: true,
		Encoder:    "hevc_qsv",
		//Encoder: "hevc_amf",
		//Encoder: "libsvtav1",
		//Encoder:         "hevc_qsv",
		//Encoder:         "av1_amf",
		//Encoder:         "av1_nvenc",
		EncodeChunkTime: 15,
	})
}
