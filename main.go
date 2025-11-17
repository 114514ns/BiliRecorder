package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	bili "github.com/114514ns/BiliClient"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

func initResty() {
	client.OnBeforeRequest(func(c *resty.Client, request *resty.Request) error {
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
		if strings.Contains(request.URL, "bilibili.com") {
			request.Header.Set("Cookie", "buvid3="+uuid.New().String()+"infoc")
		}
		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, response *resty.Response) error {
		if response.StatusCode() > 299 && response.StatusCode() != 404 {
			log.Println(response.StatusCode())
			log.Println(response.String())
			log.Println(response.Request.URL)
			debug.PrintStack()
		}
		return nil
	})
}

func TraceAudio(client *resty.Client, room int, config RoomConfig, live Live) {
	var typo = config.Dst.(Storage).Type()
	var stream = biliDirectClient.GetLiveStream(room, true)

	log.Printf("[%s ]Audio Stream: %v\n", live.UName, stream)

	var ext = ".flv"

	resp, err := client.R().
		SetHeader("Referer", "https://live.bilibili.com").
		SetDoNotParseResponse(true).
		Get(stream)
	if err != nil {
		log.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.RawBody().Close()

	var w *bufio.Writer

	var bytes int64 = 0

	buffer := make([]byte, 1024*128)

	oneDriveId := ""
	oneDriveUrl := ""

	oneDriveChunk := 1

	if typo == "onedrive" {

		oneDriveId = oneDriveMkDir(oneDrive, oneDrive.RootID, live.UName)
		oneDriveId = oneDriveMkDir(oneDrive, oneDriveId, strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-"))
		oneDriveUrl = oneDriveCreate(config.Dst.(*OneDriveStorageConfig), oneDriveId, live.Title+"-"+toString(int64(oneDriveChunk))+ext)

	}
	if typo == "local" {
		var dst, _ = CreateFile(config.Dst.(LocalStorageConfig).Location + "/" + live.UName + "/" + strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-") + "/" + live.Title + "-" + toString(int64(oneDriveChunk)) + ext)
		w = bufio.NewWriter(dst)
		defer dst.Close()
		defer func() {
			cmd := exec.Command("ffmpeg", "-i", dst.Name(), "-acodec", "copy", strings.Replace(dst.Name(), ".flv", ".aac", 1))
			//cmd.Stdout = os.Stdout
			//cmd.Stderr = os.Stderr
			cmd.Run()
		}()

	}

	for {
		n, err := io.ReadFull(resp.RawBody(), buffer)
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			if n > 0 {
				if typo == "onedrive" {
					oneDriveUpload(oneDrive, bytes, oneDrive.AudioChunkSize, oneDrive.AudioChunkSize, oneDriveUrl, make([]byte, 16))
				}
				if typo == "local" {
					w.Write(buffer[:n])
					w.Flush()

				}
			}
			break
		} else if err != nil {
			log.Printf("读取流失败: %v\n", err)
		}

		if typo == "local" {
			w.Write(buffer)
			w.Flush()
		}
		if typo == "onedrive" {
			if oneDrive.ChunkSize-bytes <= 1024*1024*5 {
				oneDriveUpload(oneDrive, bytes, oneDrive.AudioChunkSize-1, oneDrive.AudioChunkSize, oneDriveUrl, buffer[:n])
				oneDriveChunk++
				oneDriveUrl = oneDriveCreate(config.Dst.(*OneDriveStorageConfig), oneDriveId, live.Title+"-"+toString(int64(oneDriveChunk))+ext)
				bytes = 0
			} else {
				oneDriveUpload(oneDrive, bytes, bytes+int64(n), oneDrive.AudioChunkSize, oneDriveUrl, buffer[:n])
				bytes = bytes + int64(n)
			}
		}

	}

}
func TraceStream(client *resty.Client, room int, dst0 string, config RoomConfig) {

	refreshTicker := time.NewTicker(time.Minute * 40)

	var live Live

	var ext = ".mp4"
	var obj interface{}
	res, _ := client.R().Get("https://api.live.bilibili.com/room/v1/Room/get_info?room_id=" + toString(int64(room)))
	json.Unmarshal(res.Body(), &obj)
	var uid = getInt64(obj, "data.uid")
	var begin = getString(obj, "data.live_time")
	var title = getString(obj, "data.title")
	var cover = getString(obj, "data.user_cover")
	res, _ = client.R().Get("https://api.live.bilibili.com/xlive/custom-activity-interface/baseActivity/GeneralGetUserInfo?uids=" + toString(uid))
	json.Unmarshal(res.Body(), &obj)
	var uname = getString(obj, "data.data."+toString(uid)+".uname")
	var count = 0
	var stream = biliClient.GetLiveStream(room, false)
	var dstType = config.Dst.(Storage).Type()
	log.Printf("[%s ]Video Stream: \n"+stream, uname)
	_, err := os.Stat("temp")
	if os.IsNotExist(err) {
		os.Mkdir("temp", os.ModePerm)
	}
	var ticker = time.NewTicker(time.Second * 2)

	var m0 = make(map[string]bool)

	var oneDriveChunk = 1 //OneDrive分片，分了多少个
	var u, _ = url.Parse(stream)

	defer ticker.Stop()
	done := make(chan bool, 1)
	sigs := make(chan os.Signal, 1)
	var oneDriveId = ""
	var token = ""

	log.Printf("[%s] Living\n", uname)

	live.UName = uname
	live.UID = uid
	live.Title = title
	live.Time, _ = time.Parse(time.DateTime, begin)
	live.Cover = cover

	mutex.Lock()

	r := m[room]
	r.Room = room
	r.Title = title
	r.UName = uname
	r.UID = uid
	r.Live = live.Time
	r.Record = time.Now()

	mutex.Unlock()

	var dst, _ = os.Create("")
	if dstType == "local" {
		dst, _ = CreateFile(config.Dst.(LocalStorageConfig).Location + "/" + live.UName + "/" + strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-") + "/" + title + "." + ext)
	}

	w := bufio.NewWriter(dst)
	defer func() {
		log.Printf("[%s] Exit\n", uname)
		mutex.Lock()
		delete(m, room)
		mutex.Unlock()
		if dstType == "local" {
			dst.Close()
			cmd := exec.Command("ffmpeg", "-i", dst.Name(), "-vcodec", config.Encoder, strings.Replace(dst.Name(), ".mp4", "-code.mp4", 1))
			//cmd.Stdout = os.Stdout
			//cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}()

	var bytes int64 = 0  //用于标记OneDrive上传的字节数
	var oneDriveUrl = "" //OneDrive上传url
	pr, pw := io.Pipe()  //用于上传到Alist
	if dstType == "alist" {
		token = alistGetToken(config.Dst.(AlistStorageConfig))
	}
	if !(dstType == "alist") {
		pw.Close()
		pr.Close()
	}

	if dstType == "onedrive" {

		oneDriveId = oneDriveMkDir(oneDrive, oneDrive.RootID, uname)
		oneDriveId = oneDriveMkDir(oneDrive, oneDriveId, strings.ReplaceAll(begin, ":", "-"))
		oneDriveUrl = oneDriveCreate(config.Dst.(*OneDriveStorageConfig), oneDriveId, title+"-"+toString(int64(oneDriveChunk))+ext)
	}
	go func() {
		//TraceAudio(client, room, config, live)
	}()
	go func() {

		for range refreshTicker.C {
			var t0 = stream
			stream = biliClient.GetLiveStream(room, false)
			log.Printf("[%s] Last Stream%s\nRefresh Stream: \n"+stream, uname, t0)
		}
	}()
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if config.Dst.(Storage).Type() == "alist" {
			alistUploadFile(pr, "Microsoft365/小雾uya.mp4", token, config.Dst.(AlistStorageConfig).Server)
		}
	}()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			str, err := biliClient.Resty.R().Get(stream)
			var retry = 1
			for {
				if str.StatusCode() != 200 && retry > 0 {
					stream = biliClient.GetLiveStream(room, false)
					str, err = client.R().Get(stream)
					retry--
				} else {
					break
				}
			}
			if err != nil || str.StatusCode() != 200 {
				refreshTicker.Stop()
				ticker.Stop()
				if config.Dst.(Storage).Type() == "alist" {
					pw.Close()
				}
				if config.Dst.(Storage).Type() == "onedrive" {
					oneDriveUpload(oneDrive, bytes, oneDrive.ChunkSize, oneDrive.ChunkSize, oneDriveUrl, make([]byte, 16))
				}
				done <- true
			}

			for _, s := range strings.Split(str.String(), "\n") {
				if !strings.HasPrefix(s, "#") {
					_, ok := m0[s]
					if !ok {
						path := u.Path
						split := strings.Split(path, "/")
						var d = ""
						for i, s2 := range split {
							if i != len(split)-1 {
								d += s2 + "/"
							}
						}
						r, err1 := client.R().Get("https://" + u.Host + d + s)
						if err1 != nil {
							log.Println(err1)
						}
						count++
						if config.ReEncoding { //如果启用了重新编码
							w.Write(r.Body())
							w.Flush()

						} else {
							if config.Dst.(Storage).Type() == "local" {
								var chunk = count / config.ChunkTime
								name := "temp/" + strconv.Itoa(chunk) + ".mp4"
								_, err := os.Stat(name)
								if err != nil {
									os.Create(name)
									var f, _ = os.Create(name)
									w = bufio.NewWriter(f)
								}
								w.Write(r.Body())
								w.Flush()
							}
							if config.Dst.(Storage).Type() == "alist" {
								pw.Write(r.Body())
							}
							if dstType == "onedrive" {
								if len(r.Body()) == 0 {
									log.Printf("[%s] Length Error\n", uname)
									log.Printf("[%s] "+r.Request.URL, uname)
									log.Printf("[%s] %d", uname, r.StatusCode())
								}
								if oneDrive.ChunkSize-bytes <= 1024*1024*10 {
									oneDriveUpload(oneDrive, bytes, oneDrive.ChunkSize-1, oneDrive.ChunkSize, oneDriveUrl, r.Body())
									oneDriveChunk++
									oneDriveUrl = oneDriveCreate(config.Dst.(*OneDriveStorageConfig), oneDriveId, title+"-"+toString(int64(oneDriveChunk))+ext)
									bytes = 0
									fmt.Println("end")
								} else {
									oneDriveUpload(oneDrive, bytes, bytes+int64(len(r.Body())), oneDrive.ChunkSize, oneDriveUrl, r.Body())
									bytes = bytes + int64(len(r.Body()))
								}
							}
						}

						m0[s] = true
					}
				}
			}
		}
	}
}

var m = make(map[int]*RoomStatus)

var mutex sync.Mutex

func RefreshStatus(id []int64) {
	var s = "https://api.live.bilibili.com/xlive/fuxi-interface/UserService/getUserInfo?_ts_rpc_args_=[["
	for i, i2 := range id {
		s = s + strconv.FormatInt(i2, 10)
		if i != len(id)-1 {
			s = s + ","
		}
	}
	s = s + `],true,""]`
	res, _ := biliClient.Resty.R().Get(s)
	var m0 map[string]interface{}
	json.Unmarshal(res.Body(), &m0)

	if !strings.Contains(res.String()[0:70], "_ts_rpc_return_") || strings.Contains(res.String()[0:70], "服务调用超时") {
		fmt.Println(res.String())
		time.Sleep(time.Second * 3)
		RefreshStatus(id)

	}
	for _, i := range m0["_ts_rpc_return_"].(map[string]interface{})["data"].(map[string]interface{}) {
		if getString(i, "liveStatus") == "1" {
			var room = toInt(getString(i, "roomId"))
			mutex.Lock()
			_, ok := m[room]
			if !ok {
				go func() {
					m[room] = &RoomStatus{}
					TraceStream(biliClient.Resty, room, "", config.GlobalConfig)
				}()
			}
			mutex.Unlock()
		}
	}
}

var client = resty.New()
var oneDrive = &OneDriveStorageConfig{}
var cookie, _ = os.ReadFile("cookie.txt")
var biliClient = bili.NewClient(string(cookie), bili.ClientOptions{
	HttpProxy: config.ProxyPass,
	ProxyUser: config.ProxyUser,
	ProxyPass: config.ProxyPass,
})

var biliDirectClient = bili.NewClient(string(cookie), bili.ClientOptions{})
var config Config

func loadConfig() {
	bytes, err := os.ReadFile("config.json")
	if err != nil {
		log.Println("[Error]", err)
		debug.PrintStack()
		os.Exit(1)
	}
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		log.Println("[Error]", err)
		debug.PrintStack()
		os.Exit(1)
	}
}

func saveConfig() {
	bytes, err := json.Marshal(config)
	if err != nil {
		log.Println("[Error]", err)
		debug.PrintStack()
	}
	err = os.WriteFile("config.json", bytes, 0644)
}

func main() {
	go InitHTTP()
	c := cron.New()
	loadConfig()
	initResty()
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	if config.GlobalConfig.Dst.Type() == "onedrive" {
		var c = config.GlobalConfig.Dst.(*OneDriveStorageConfig)
		oneDrive = c
		oneDriveInit(oneDrive)
	}
	RefreshStatus(config.Livers)
	c.AddFunc("@every 60s", func() {
		RefreshStatus(config.Livers)
	})
	c.Start()
	/*
		page := biliClient.GetAreaLiveByPage(9, 1)

		page = page[:5]

		for i := range page {
			go func() {
				TraceStream(client, page[i].Room, "out.mp4", RoomConfig{
					KeepTemp:   true,
					ReEncoding: false,
					//Encoder:    "hevc_qsv",
					//Encoder: "hevc_amf",
					//Encoder: "libsvtav1",
					//Encoder:         "hevc_qsv",
					//Encoder:         "av1_amf",
					Encoder:   "av1_nvenc",
					ChunkTime: 60,
					Dst:       oneDrive,
				})
			}()
			time.Sleep(5 * time.Second)
		}

	*/

	select {}

}
