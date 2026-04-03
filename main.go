package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	bili "github.com/114514ns/BiliClient"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
)

var last = make(map[string]string)

func initResty() {
	httpClient := client.GetClient()

	// 配置 Transport 禁用 HTTP/2
	httpClient.Transport = &http.Transport{
		ForceAttemptHTTP2: false, // 禁用 HTTP/2
	}

	client.OnBeforeRequest(func(c *resty.Client, request *resty.Request) error {
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
		if strings.Contains(request.URL, "bilibili.com") {
			request.Header.Set("Cookie", "buvid3="+uuid.New().String()+"infoc")
		}
		if request.Header.Get("Content-Range") != "" {
			log.Println("submit   " + request.Header.Get("Content-Range"))
		}
		request.Header.Set("UUID", uuid.NewString())

		return nil
	})

	client.OnAfterResponse(func(c *resty.Client, response *resty.Response) error {
		if response.StatusCode() > 299 && response.StatusCode() != 404 {
			log.Println(response.StatusCode())
			log.Println(response.Request.Header.Get("Content-Length"))
			log.Println(response.Request.Header.Get("Content-Range"))
			log.Println(response.String())

			log.Println(response.Request.URL)
			log.Println(response.Request.Header.Get("UUID"))

			log.Println(last[response.Request.Header.Get("Label")])

			//debug.PrintStack()
		}
		if response.Request.Header.Get("Label") != "" {
			last[response.Request.Header.Get("Label")] = response.Request.Header.Get("Content-Range")
		}

		return nil
	})

	res, _ := client.R().Get("https://my.ippure.com/v1/info")
	var obj map[string]interface{}
	json.Unmarshal(res.Body(), &obj)
	log.Printf("Address : %s\n", getString(obj, "ip"))

	if config.OneDriveProxy != "" {
		oneDriveClient.SetProxy(config.OneDriveProxy)
	}
}

func GetStreamFlv(room int, client *resty.Client) string {
	res, _ := client.R().Get(fmt.Sprintf("https://api.live.bilibili.com/xlive/web-room/v2/index/getRoomPlayInfo?codec=0,1,2&format=0,1,2&protocol=0,1,2&qn=10000&room_id=%d", room))
	var obj interface{}
	json.Unmarshal(res.Body(), &obj)
	var o0 = getArray(obj, "data.playurl_info.playurl.stream")
	var o1 = o0[0 /*len(o0)-1*/]
	var o2 = getArray(o1, "format")[0]
	var o3 = getArray(o2, "codec")
	var o4 = o3[len(o3)-1]
	var o5 = getArray(o4, "url_info")[0]

	var extra = getString(o5, "extra")

	var path = getString(o4, "base_url")

	var host = getString(o5, "host")

	return host + path + extra

}
func TraceAudio(config RoomConfig, live Live, room int) {
	var typo = config.Dst.(Storage).Type()

	var ext = ".aac"

	oneDriveId := ""
	oneDriveUrl := ""
	var w *bufio.Writer
	var bytes int64 = 0

	oneDriveChunk := 1

	var oneDrive *OneDriveStorageConfig = nil

	if typo == "onedrive" {
		oneDrive = getDstByLabel(config.DstLabel).(*OneDriveStorageConfig)
		oneDriveId = oneDriveMkDir(oneDrive, oneDrive.RootID, live.UName)
		oneDriveId = oneDriveMkDir(oneDrive, oneDriveId, strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-"))
		oneDriveUrl = oneDriveCreate(oneDrive, oneDriveId, live.Title+"-"+toString(int64(oneDriveChunk))+ext)

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
	var buffer []byte
	var n = len(buffer)
	for {
		_, ok := m[room]
		if typo == "onedrive" {
			if m[room].End {
				if ok {
					buffer = m[room].AudioBufferBytes
				} else {

				}
				oneDriveUpload(oneDrive, bytes, oneDrive.AudioChunkSize-1, oneDrive.AudioChunkSize, oneDriveUrl, buffer)
				break
			}
		}

		if int64(len(m[room].AudioBufferBytes)) > 1024*768 {

			buffer = m[room].AudioBufferBytes
			n = len(buffer)
			m[room].AudioBufferBytes = make([]byte, 0)

			if typo == "onedrive" {
				if oneDrive.AudioChunkSize-bytes <= 1024*1024*10 {
					log.Println(live.Title + "-" + toString(int64(oneDriveChunk)) + ext)
					_, res := oneDriveUpload(oneDrive, bytes, oneDrive.AudioChunkSize-1, oneDrive.AudioChunkSize, oneDriveUrl, buffer)
					log.Println(res.String())
					oneDriveChunk++
					oneDriveUrl = oneDriveCreate(oneDrive, oneDriveId, live.Title+"-"+toString(int64(oneDriveChunk))+ext)
					bytes = 0
					m[room].OnedriveAudioOffset = 0
				} else {
					if code, _ := oneDriveUpload(oneDrive, bytes, bytes+int64(n-1), oneDrive.AudioChunkSize, oneDriveUrl, buffer[:n]); code != 416 {
						m[room].OnedriveAudioOffset = bytes
						bytes = bytes + int64(n)
					} else {
						bytes = m[room].OnedriveAudioOffset
					}

				}
			}
			if typo == "local" {
				w.Write(buffer)
				w.Flush()
			}
		}

		time.Sleep(100 * time.Millisecond)

	}

}
func TraceStream(client *resty.Client, room int, config0 RoomConfig) {
	label := getDstByLabel(config0.DstLabel)

	if label == nil {
		log.Printf("label %s not exist", config0.DstLabel)
		return
	}
	config0.Dst = label
	var uniqueDir = uuid.NewString()

	var roomConfig RoomConfig
	_ = copier.CopyWithOption(&roomConfig, &config0, copier.Option{
		DeepCopy: true,
	})

	var offsetMap = make([]string, 0)

	var chunk = 1 //ts分片个数

	//var worker = NewPool(1)

	refreshTicker := time.NewTicker(time.Minute * 40)

	var live Live

	var fmp4 = make(map[string][]byte)

	var container = roomConfig.Container

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
	var stream = biliClient.GetLiveStream(room, container, false)
	var dstType = roomConfig.Dst.(Storage).Type()
	log.Printf("[%s ]Video Stream: \n"+stream, uname)
	_, err := os.Stat("temp")
	if os.IsNotExist(err) {
		os.Mkdir("temp", os.ModePerm)
	}
	_, err = os.Stat("temp/" + uniqueDir)
	if os.IsNotExist(err) {
		os.Mkdir("temp/"+uniqueDir, os.ModePerm)
	}
	var ticker = time.NewTicker(time.Millisecond * 750)

	var m0 = make(map[string]bool)

	var oneDriveChunk = 1 //OneDrive分片，分了多少个
	var u, _ = url.Parse(stream)

	defer ticker.Stop()
	done := make(chan bool, 1)
	//sigs := make(chan os.Signal, 1)
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

	var oneDrive *OneDriveStorageConfig
	var dst, _ = os.Create("")
	if dstType == "local" {
		dst, _ = CreateFile(roomConfig.Dst.(LocalStorageConfig).Location + "/" + live.UName + "/" + strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-") + "/" + title + "." + ext)
	}
	if dstType == "onedrive" {
		oneDrive = label.(*OneDriveStorageConfig)
	}

	//go TraceAudio(roomConfig, live, room)
	w := bufio.NewWriter(dst)
	defer func() {
		log.Printf("[%s] Exit\n", uname)
		mutex.Lock()
		delete(m, room)
		mutex.Unlock()
		if dstType == "local" {
			var loop = 0
			for {
				dir, _ := os.ReadDir("temp/" + uniqueDir)
				var ok = true
				for i := range dir {
					if strings.HasPrefix(dir[i].Name(), ".lck") {
						ok = false
						break
					}
				}
				if ok || loop >= 600 {
					break
				}
				time.Sleep(time.Millisecond * 500 * 2)
				loop++
			}
			dst.Close()

			var args []string
			var listContent = ""
			dir, _ := os.ReadDir("temp/" + uniqueDir)
			sort.Slice(dir, func(i, j int) bool {
				return toInt(strings.Replace(dir[i].Name(), "-code.mp4", "", -1)) < toInt(strings.Replace(dir[j].Name(), "-code.mp4", "", -1))
			})
			for i := range dir {
				var fileName = dir[i].Name()
				if strings.Contains(fileName, "code") || !roomConfig.ReEncoding {
					var s = "file '"
					s += fileName
					s += "'\r\n"
					listContent += s
				}
			}
			os.Create("temp/" + uniqueDir + "/list.txt")
			os.WriteFile("temp/"+uniqueDir+"/list.txt", []byte(listContent), 0644)
			args = append(args, "-y")
			args = append(args, "-f")
			args = append(args, "concat")
			args = append(args, "-safe")
			args = append(args, "0")
			args = append(args, "-i")
			args = append(args, "temp/"+uniqueDir+"/list.txt")
			args = append(args, "-c")
			args = append(args, "copy")
			args = append(args, dst.Name())
			cmd := exec.Command("ffmpeg", args...)
			bytes, _ := cmd.CombinedOutput()

			stat, e := os.Stat(dst.Name())

			if e != nil || stat.Size() < 1024 {
				log.Printf("[%s] Merge Error\n", uname)
				log.Printf("[%s] FFmpeg Output\n", uname)
				log.Println(string(bytes))
			} else {
				if !roomConfig.KeepTemp {
					os.RemoveAll("temp/" + uniqueDir)
				}
			}

			var aCodec = roomConfig.AudioCodec

			if aCodec == "" {
				aCodec = "mp3"
			}

			cmd = exec.Command("ffmpeg", "-i",
				roomConfig.Dst.(LocalStorageConfig).Location+"/"+live.UName+"/"+strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-")+"/audio.aac",
				roomConfig.Dst.(LocalStorageConfig).Location+"/"+live.UName+"/"+strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-")+"/audio."+aCodec)
			bytes, _ = cmd.CombinedOutput()
			stat, e = os.Stat(roomConfig.Dst.(LocalStorageConfig).Location + "/" + live.UName + "/" + strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-") + "/audio." + aCodec)
			if e != nil || stat.Size() < 1024 {
				log.Printf("[%s] Auido Code Error Error\n", uname)
				log.Printf("[%s] FFmpeg Output\n", uname)
				log.Println(string(bytes))
			} else {
				if !roomConfig.KeepTemp {
					os.Remove(roomConfig.Dst.(LocalStorageConfig).Location + "/" + live.UName + "/" + strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-") + "/audio.aac")
				}
			}
			if roomConfig.EnableTranscribe {

			}
		}
	}()

	var bytes int64 = 0  //用于标记OneDrive上传的字节数
	var oneDriveUrl = "" //OneDrive上传url
	pr, pw := io.Pipe()  //用于上传到Alist
	if dstType == "alist" {
		token = alistGetToken(roomConfig.Dst.(AlistStorageConfig))
	}
	if !(dstType == "alist") {
		pw.Close()
		pr.Close()
	}

	if dstType == "onedrive" {

		oneDriveId = oneDriveMkDir(oneDrive, oneDrive.RootID, uname)
		oneDriveId = oneDriveMkDir(oneDrive, oneDriveId, strings.ReplaceAll(begin, ":", "-"))
		oneDriveUrl = oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk))+ext)
	}
	go func() {

		for range refreshTicker.C {
			var t0 = stream
			stream = biliClient.GetLiveStream(room, container, false)

			log.Printf("[%s] Last Stream%s\nRefresh Stream: \n"+stream, uname, t0)
		}
	}()
	go func() {
		if roomConfig.Dst.(Storage).Type() == "alist" {
			alistUploadFile(pr, "Microsoft365/小雾uya.mp4", token, roomConfig.Dst.(AlistStorageConfig).Server)
		}
	}()

	go func() {
		if roomConfig.WithEvents {
			var actions []bili.FrontLiveAction

			var avatarMap = make(map[string]bili.FrontLiveAction)

			var mu sync.Mutex
			go func() {
				for {
					select {
					case <-ticker.C:
						{
							if rand.Int()%2 == 1 { //750ms*2，1.5秒
								liveActions := biliClient.GetHistory(room)

								var online = biliClient.GetLiveOnlineRank(room, uid)
								mu.Lock()
								for i := range liveActions {
									avatarMap[liveActions[i].Face] = liveActions[i]
								}
								for _, i := range online {
									var inner = bili.LiveAction{
										FromId:   i.UID,
										FromName: i.UName,
									}
									avatarMap[i.Face] = bili.FrontLiveAction{
										LiveAction: inner,
										Face:       i.Face,
									}
								}

								for i := range actions {
									if actions[i].FromId == 0 {
										v, ok := avatarMap[actions[i].Face]
										if ok {
											actions[i].FromId = v.FromId
											actions[i].FromName = v.FromName
										}
									}
								}
								mu.Unlock()
							}

							//9秒
							if rand.Int()%12 == 1 {
								marshal, _ := json.Marshal(actions)
								if roomConfig.Dst.(Storage).Type() == "local" {
									os.WriteFile(roomConfig.Dst.(LocalStorageConfig).Location+"/"+live.UName+"/"+strings.ReplaceAll(live.Time.Format(time.DateTime), ":", "-")+"/events.json", marshal, 0644)
								}
								if oneDrive != nil {
									//270秒
									if rand.Int()%30 == 1 {
										session := oneDriveCreate(oneDrive, oneDriveId, "events.json")
										oneDriveUpload(oneDrive, 0, int64(len(marshal)-1), int64(len(marshal)), session, marshal)
									}

								}

							}
						}
					}
				}
			}()

			biliClient.TraceLive(toString(int64(room)), func(action bili.FrontLiveAction) {
				if action.FromId == 0 {
					v, ok := avatarMap[action.Face]
					if ok {
						mu.Lock()
						action.FromId = v.FromId
						action.FromName = v.FromName
						mu.Unlock()
					}
				}
				actions = append(actions, action)
			}, nil)
		}
	}()
	var extMap = ""
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			str, err := biliDirectClient.Resty.R().Get(stream)

			var retry = 1
			for {
				if str.StatusCode() != 200 && retry > 0 {
					stream = biliClient.GetLiveStream(room, container, false)
					str, err = client.R().Get(stream)
					retry--
				} else {
					break
				}
			}
			m[room].Stream = str.Request.URL
			if err != nil || str.StatusCode() != 200 || stream == "" || m[room].End {
				log.Println(err)
				log.Println(str.StatusCode())
				refreshTicker.Stop()
				ticker.Stop()
				if roomConfig.Dst.(Storage).Type() == "alist" {
					pw.Close()
				}
				if roomConfig.Dst.(Storage).Type() == "onedrive" {
					log.Printf("[%s] End", live.UName)
					_, r0 := oneDriveUpload(oneDrive, bytes, oneDrive.ChunkSize, oneDrive.ChunkSize, oneDriveUrl, make([]byte, 16))

					mapUrl := oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk))+".json")
					b, _ := json.Marshal(offsetMap)
					oneDriveUpload(oneDrive, 0, int64(len(b)-1), int64(len(b)), mapUrl, b)
					mapUrl = oneDriveCreate(oneDrive, oneDriveId, "metadata.json")
					b, _ = json.Marshal(m[room])
					oneDriveUpload(oneDrive, 0, int64(len(b)-1), int64(len(b)), mapUrl, b)

					if roomConfig.AutoConvert {
						var obj0 map[string]interface{}
						json.Unmarshal(r0.Body(), &obj0)
						b0, _ := json.Marshal(offsetMap)
						var link = oneDriveDownload(oneDrive, getString(obj0, "id"))
						resty.New().R().
							SetQueryParam("link", link).
							SetFormData(map[string]string{
								"mapping": string(b0),
								"dir":     oneDriveId,
								"id":      getString(obj0, "id"),
							}).
							SetQueryParam("fName", title+"-"+toString(int64(oneDriveChunk))+ext).
							Post("http://127.0.0.1:" + toString(int64(config.Port)) + "/convert")
					}
				}
				done <- true
			}
			if strings.Contains(str.String(), ".ts") {
				container = "ts"
			}
			if strings.Contains(str.String(), ".m4s") {
				container = "fmp4"
			}
			for _, s := range strings.Split(str.String(), "\n") {
				path := u.Path
				split := strings.Split(path, "/")
				var d = ""
				for i, s2 := range split {
					if i != len(split)-1 {
						d += s2 + "/"
					}
				}
				if m[room].ChunkBegin.IsZero() {
					m[room].ChunkBegin = time.Now()
				}
				if strings.Contains(s, "#EXTINF") {
					//offsetMap = append(offsetMap, s)
				}

				if strings.Contains(s, "#EXT-X-MAP:URI=") {
					extMap0 := strings.Replace(strings.Replace(s, "#EXT-X-MAP:URI=", "", 1), `"`, "", 2)
					if extMap0 != "" {
						extMap = extMap0
						_, ok := fmp4[extMap]
						if !ok {
							v, _ := biliDirectClient.Resty.R().Get("https://" + u.Host + d + extMap)
							fmp4[extMap] = v.Body()
							log.Printf("[%s] download map %s \n", live.UName, v.Request.URL)
						}
					}
				}
				if !strings.HasPrefix(s, "#") && s != "" {
					_, ok := m0[s]
					if !ok {

						r, err1 := biliDirectClient.Resty.R().Get("https://" + u.Host + d + s)
						if r.StatusCode() != 200 {
							r, err1 = client.R().Get("https://" + u.Host + d + s)
						}
						if err1 != nil {
							log.Println(err1)
						}
						count++
						if roomConfig.Dst.(Storage).Type() == "local" {
							var fsChunk = count / roomConfig.ChunkTime
							name := "temp/" + uniqueDir + "/" + strconv.Itoa(fsChunk) + ".mp4"
							_, err := os.Stat(name)
							if err != nil {
								os.Create(name)
								var f, _ = os.Create(name)
								w = bufio.NewWriter(f)
							}
							if count == 1 {
								log.Printf("[%s] write MAP \n", live.UName)
								w.Write(fmp4[extMap])
							}
							w.Write(r.Body())
							w.Flush()
							if fsChunk*roomConfig.ChunkTime == count {
								if roomConfig.ReEncoding {
									go func() {
										var args []string

										if roomConfig.VADevice != "" && strings.Contains(roomConfig.Encoder, "vaapi") {
											args = append(args, "-vaapi_device")
											args = append(args, roomConfig.VADevice)

										}
										args = append(args, "-i")
										args = append(args, "temp/"+uniqueDir+"/"+strconv.Itoa(fsChunk-1)+".mp4")

										if roomConfig.VADevice != "" && strings.Contains(roomConfig.Encoder, "vaapi") {
											args = append(args, "-vf")
											args = append(args, "format=nv12,hwupload")
										}
										args = append(args, "-c:v")
										args = append(args, roomConfig.Encoder)

										if roomConfig.Bitrate != 0 {
											args = append(args, "-b:v")
											args = append(args, toString(int64(roomConfig.Bitrate))+"k")
										}

										args = append(args, "temp/"+uniqueDir+"/"+strconv.Itoa(fsChunk-1)+"-code.mp4")

										cmd := exec.Command("ffmpeg", args...)
										cmd.Stdout = os.Stdout
										cmd.Stderr = os.Stderr
										var lckFile = uuid.NewString() + ".lck"
										os.Create(lckFile)
										cmd.Run()
										os.Remove(lckFile)

										stat, e := os.Stat("temp/" + uniqueDir + "/" + strconv.Itoa(fsChunk-1) + "-code.mp4")
										if e != nil || stat.Size() < 1024 {

										} else {
											if !roomConfig.KeepTemp {
												os.Remove("temp/" + uniqueDir + "/" + strconv.Itoa(fsChunk-1) + ".mp4")
											}
										}
									}()
								}
							}
						}
						if roomConfig.Dst.(Storage).Type() == "alist" {
							pw.Write(r.Body())
						}
						if dstType == "onedrive" {
							body := r.Body()
							curBytes := bytes
							curUrl := oneDriveUrl
							chunkSize := oneDrive.ChunkSize
							curOneDrive := oneDrive
							m[room].BitRate = (float64(bytes) + float64(len(m[room].BufferBytes))) * 10.0 / time.Now().Sub(m[room].ChunkBegin).Seconds() / 1024.0 / 1024.0
							if len(body) == 0 {
								log.Printf("[%s] Length Error\n", uname)
								log.Printf("[%s] "+r.Request.URL, uname)
								log.Printf("[%s] %d", uname, r.StatusCode())
							} else {
								m[room].AudioBufferBytes = append(m[room].AudioBufferBytes, Extract(body)...)

								var l0st = 0
								if len(offsetMap) > 0 {
									l0st = toInt(strings.Split(offsetMap[len(offsetMap)-1], ",")[1]) + 1
								}
								offsetMap = append(offsetMap, fmt.Sprintf("%d,%d", l0st, l0st+len(body)-1))
								if len(m[room].ChunkRecord)+1 == oneDriveChunk {
									m[room].ChunkRecord = append(m[room].ChunkRecord, time.Now())
								}
								if oneDrive.BufferChunk >= 1 {
									m[room].BufferBytes = append(m[room].BufferBytes, body...)
									//log.Println("append,length=" + toString(int64(len(m[room].BufferBytes))))
									if chunk%oneDrive.BufferChunk == 0 {
										var to int64 = 0
										var broder = false
										if chunkSize-(curBytes+int64(len(m[room].BufferBytes))-1) <= 64*1024*1024 {
											to = chunkSize - 1
											broder = true
										} else {
											to = curBytes + int64(len(m[room].BufferBytes)) - 1
										}
										var t = oneDrive.Retry
										for {
											t--

											code, r0 := oneDriveUpload(curOneDrive, curBytes, to, chunkSize, curUrl, m[room].BufferBytes)
											log.Println(fmt.Sprintf("%d,%d", curBytes, curBytes+int64(len(body))-1))
											if code != 416 {

												m[room].OnedriveOffset = bytes
												bytes = curBytes + int64(len(m[room].BufferBytes))
												//offsetMap = append(offsetMap, fmt.Sprintf("%d,%d", curBytes, curBytes+int64(len(body))-1))

												if broder {
													m[room].ChunkBegin = time.Now()
													oneDriveChunk++
													oneDriveUrl = oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk))+ext)
													offsetMap = append(offsetMap, fmt.Sprintf("%d,%d", curBytes, curBytes+int64(len(body))-1))
													bytes = 0
													m[room].OnedriveOffset = 0
													mapUrl := oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk-1))+".json")
													b, _ := json.Marshal(offsetMap)
													log.Println(oneDriveUpload(curOneDrive, 0, int64(len(b)-1), int64(len(b)), mapUrl, b))

													offsetMap = make([]string, 0)
													if r0 == nil {
														fmt.Println("r0 = nil")
													}
													if r0 != nil && !strings.Contains(r0.String(), "createdBy") {
														fmt.Println(r0.String())
													}
													go func() {
														time.Sleep(time.Second * 60)
														if roomConfig.AutoConvert {
															var obj0 map[string]interface{}
															json.Unmarshal(r0.Body(), &obj0)
															var link = oneDriveDownload(oneDrive, getString(obj0, "id"))
															resty.New().R().
																SetQueryParam("link", link).
																SetFormData(map[string]string{
																	"mapping": string(b),
																	"dir":     oneDriveId,
																	"id":      getString(obj0, "id"),
																}).
																SetQueryParam("fName", title+"-"+toString(int64(oneDriveChunk-1))+ext).
																Post("http://127.0.0.1:" + toString(int64(config.Port)) + "/convert")
														}
													}()

												}
												break
											} else {
												bytes = m[room].OnedriveOffset
											}
											if t <= 0 {
												break
											}
										}
										m[room].BufferBytes = make([]byte, 0)
									}
								} else {
									if chunkSize-curBytes <= 10*1024*1024 {
										// 最后一块
										oneDriveUpload(curOneDrive, curBytes, chunkSize-1, chunkSize, curUrl, body)
										oneDriveChunk++
										oneDriveUrl = oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk))+ext)
										offsetMap = append(offsetMap, fmt.Sprintf("%d,%d", curBytes, curBytes+int64(len(body))-1))
										bytes = 0
										m[room].OnedriveOffset = 0
										fmt.Println("end")

										mapUrl := oneDriveCreate(oneDrive, oneDriveId, title+"-"+toString(int64(oneDriveChunk-1))+".json")

										b, _ := json.Marshal(offsetMap)
										oneDriveUpload(curOneDrive, 0, int64(len(b)-1), int64(len(b)), mapUrl, b)

										offsetMap = make([]string, 0)

									} else {
										end := curBytes + int64(len(body)) - 1

										if code, _ := oneDriveUpload(curOneDrive, curBytes, end, chunkSize, curUrl, body); code != 416 {
											m[room].OnedriveOffset = bytes
											bytes = curBytes + int64(len(body))
											offsetMap = append(offsetMap, fmt.Sprintf("%d,%d", curBytes, end))
										} else {
											bytes = m[room].OnedriveOffset
										}
									}
								}
							}
						}
					}

					m0[s] = true
					chunk++
				}
			}
		}
	}
}

var m = make(map[int]*RoomStatus)

var mutex sync.Mutex

func RefreshStatus(id []int64) {
	for _, int64s := range chunkSlice(id, 40) {
		RefreshStatus0(int64s)
	}
}

var INIT = false

func RefreshStatus0(id []int64) {
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

	if res.Size() == 0 {
		return
	}

	if !strings.Contains(res.String()[0:70], "_ts_rpc_return_") || strings.Contains(res.String()[0:70], "服务调用超时") {
		fmt.Println(res.String())
		time.Sleep(time.Second * 3)
		return
		//RefreshStatus(id)

	}
	for _, i := range m0["_ts_rpc_return_"].(map[string]interface{})["data"].(map[string]interface{}) {
		var room = toInt(getString(i, "roomId"))
		if getString(i, "liveStatus") == "1" {
			//mutex.Lock()
			_, ok := m[room]
			if !ok {
				m[room] = &RoomStatus{} // 先在锁内写入占位
				m[room].AudioBufferBytes = make([]byte, 0)
				//mutex.Unlock()
				go func() {
					TraceStream(biliClient.Resty, room, config.GlobalConfig)
				}()
			} else {
				//mutex.Unlock()
			}
		}
		if getString(i, "liveStatus") == "0" {
			//mutex.Lock()
			v, ok := m[room]
			if ok {
				v.End = true
			}
			//mutex.Unlock()
		}
		for i2 := range config.Livers {
			if config.Livers[i2].UID == toInt64(getString(i, "uid")) {
				config.Livers[i2].UName = getString(i, "uname")
			}
		}
	}
	if !INIT {
		saveConfig()
		INIT = true
	}
}

var client = resty.New()
var oneDriveClient = resty.New()
var cookie, _ = os.ReadFile("cookie.txt")
var biliClient = bili.NewClient(string(cookie), bili.ClientOptions{
	HttpProxy: config.ProxyServer,
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
	bytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Println("[Error]", err)
		debug.PrintStack()
	}
	err = os.WriteFile("config.json", bytes, 0644)
}

var file = time.Now().Format("2006-01-02_15-04-05") + ".log"
var logFile, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)

func main() {

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	c := cron.New()
	loadConfig()
	getDstByLabel(config.GlobalConfig.DstLabel)
	initResty()
	go InitHTTP()

	biliClient = bili.NewClient(config.Cookie, bili.ClientOptions{
		HttpProxy: config.ProxyServer,
		ProxyUser: config.ProxyUser,
		ProxyPass: config.ProxyPass,
	})

	biliDirectClient.Cookie = config.Cookie

	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	RefreshStatus(lo.Map(config.Livers, func(item Liver, index int) int64 {
		return item.UID
	}))
	c.AddFunc("@every 10s", func() {
		RefreshStatus(lo.Map(config.Livers, func(item Liver, index int) int64 {
			return item.UID
		}))
	})
	c.Start()
	select {}

}
