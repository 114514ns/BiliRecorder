package main

import (
	"bufio"
	"encoding/json"
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

// TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
type LocalStorageConfig struct {
	Type     string
	Location string //本地存储
}
type AlistStorageConfig struct {
	Type    string
	User    string //Alist
	Pass    string
	RootDir string
}
type CMStorageConfig struct {
	Type    string
	Auth    string //移动网盘，支持边录边传
	RootDir string
}
type S3StorageConfig struct {
	Type    string
	User    string //S3，支持边录边传,不过成本太高，也许不会实现，先占个位
	Pass    string
	RootDir string
}
type RoomConfig struct {
	OnlyAudio       bool   //仅音频
	ReEncoding      bool   //重新编码
	Encoder         string //编码器
	PreferEncoding  string //从b站首选的流
	EncodeChunkTime int    //每块到多大的时候开始编码
	KeepTemp        bool   //保留分片，调试用
	Dst             interface{}
	EnableSTT       bool
	STTEndpoint     string
	STTAuth         string
	ChatEndPoint    string
	Model           string
}
type Config struct {
	GlobalConfig   RoomConfig
	OverrideConfig map[string]RoomConfig
	Rooms          []int
	Port           int //api端口
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
			if err != nil || str.StatusCode() != 200 {
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
						if config.ReEncoding { //如果启用了重新编码
							if count%config.EncodeChunkTime == 0 {
								go func() {
									var fileName = "temp/" + strconv.Itoa(count/config.EncodeChunkTime-1) + "-code.mp4"
									command := exec.Command("ffmpeg", "-i", "temp/"+strconv.Itoa(count/config.EncodeChunkTime-1)+".mp4", "-g", "60", "-vcodec", config.Encoder /*"-b:v", "5000k", */, "-c:a", "copy", fileName)
									//command.Stderr = os.Stderr
									//command.Stdout = os.Stdout
									command.Run()
									if config.EnableSTT {
										go func() {
											exec.Command("ffmpeg", "-y", "-i", fileName, "tmp.mp3").Run()
											res, err := client.Resty.R().SetHeader("Authorization", "Bearer "+config.STTAuth).SetFile("file", "tmp.mp3").SetFormData(map[string]string{"model": "gpt-4o-transcribe"}).Post(config.STTEndpoint)
											if err != nil {
												fmt.Println(err)
											} else {
												var obj map[string]string
												json.Unmarshal(res.Body(), &obj)
												var text = obj["text"]
												var j = fmt.Sprintf(`
{"model": "%s","stream":false,"messages": [{"role": "user","content": "我给你发一段文本，来源于b站直播的文本转录，你需要帮我给下面的文本加上标点符号，并修复可能出现的识别错误,只需给我修正后的内容，不要包含其他文本。%s"}]}`, config.Model, text)
												res, err = client.Resty.R().SetHeader("Authorization", "Bearer "+config.STTAuth).SetHeader("Content-Type", "application/json").SetBody([]byte(j)).Post(config.ChatEndPoint)
												type Response struct {
													Choices []struct {
														Message struct {
															Content string `json:"content"`
														} `json:"message"`
													} `json:"choices"`
												}
												if err != nil {
													fmt.Println(err)
												} else {
													var obj Response
													json.Unmarshal(res.Body(), &obj)
													fmt.Println(obj.Choices[0].Message.Content)
												}
											}
											if !config.KeepTemp {
												os.Remove("tmp.mp3")
											}
										}()

									}
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
	TraceStream(client, 23375552, "out.mp4", RoomConfig{
		KeepTemp:   true,
		ReEncoding: true,
		Encoder:    "hevc_qsv",
		//Encoder: "hevc_amf",
		//Encoder: "libsvtav1",
		//Encoder:         "hevc_qsv",
		//Encoder:         "av1_amf",
		//Encoder:         "av1_nvenc",
		EncodeChunkTime: 60,
		EnableSTT:       false,
		STTEndpoint:     "https://jeniya.cn/v1/audio/transcriptions",
		STTAuth:         "sk",
		ChatEndPoint:    "http://jeniya.cn/v1/chat/completions",
		Model:           "deepseek-v3-250324",
	})
}
