package main

import (
	"bufio"
	"fmt"
	"github.com/114514ns/BiliClient"
	"github.com/bytedance/sonic"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

type RoomConfig struct {
	OnlyAudio       bool   //仅音频
	ReEncoding      bool   //重新编码
	Encoder         string //编码器
	PreferEncoding  string //从b站首选的流
	EncodeChunkTime int    //每块到多大的时候开始编码
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
	var ticker = time.NewTicker(time.Second * 2)
	var dst, _ = os.Create(dst0)
	writer := bufio.NewWriter(dst)
	var m = make(map[string]bool)
	//var m = make(map[string]bool)
	var u, _ = url.Parse(stream)
	w := bufio.NewWriter(dst)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			str, _ := client.Resty.R().Get(stream)
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
									command := exec.Command("ffmpeg", "-i", "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+".mp4", "-vcodec", config.Encoder /*"-b:v", "5000k", */, "-c:a", "copy", "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+"-code.mp4")
									//command.Stderr = os.Stderr
									//command.Stdout = os.Stdout
									command.Run()
									if count/config.EncodeChunkTime == 1 {
										dst.Close()
										os.Remove(dst0)
										err := os.Rename("temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+"-code.mp4", dst0)
										if err != nil {
											fmt.Println(err)
										}
									} else {
										listContent := fmt.Sprintf("file '%s'\nfile '%s'\n", dst0, "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+"-code.mp4")
										os.WriteFile("list.txt", []byte(listContent), 0644)
										cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", "list.txt", "-c", "copy", dst0)
										cmd.Stdout = os.Stdout
										cmd.Stderr = os.Stderr
										cmd.Run()
										//os.Remove("temp/" + strconv.Itoa(count/config.EncodeChunkTime-1) + "-code.ts")
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
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	var client = bili.NewClient("", bili.ClientOptions{})
	TraceStream(client, 1883943131, "out.mp4", RoomConfig{
		ReEncoding: true,
		//Encoder:         "hevc_qsv",
		//Encoder: "hevc_amf",
		Encoder: "libsvtav1",
		//Encoder:         "hevc_qsv",
		//Encoder:         "av1_amf",
		//Encoder:         "av1_nvenc",
		PreferEncoding:  "avc",
		EncodeChunkTime: 15,
	})
}
