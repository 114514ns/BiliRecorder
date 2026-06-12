package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
)

const (
	File int = iota
	Dict
)

type OneDriveItem struct {
	Name        string
	ID          string
	DownloadURL string
	Type        int
}

func accessToken(config *OneDriveStorageConfig) string {
	res, err := oneDriveClient.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": config.RefreshToken,
			"client_id":     config.ClientID,
			"client_secret": config.ClientSecret,
			"redirect_uri":  config.RedirectURL,
		}).
		Post("https://login.microsoftonline.com/common/oauth2/v2.0/token")
	if err != nil {
		return config.AccessToken
	}
	var obj interface{}
	json.Unmarshal(res.Body(), &obj)
	return getString(obj, "access_token")
}

func oneDriveDict(config *OneDriveStorageConfig, name string) string {
	res, _ := oneDriveClient.R().SetHeader("authorization", "Bearer "+config.AccessToken).
		Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s", name))

	var obj interface{}
	json.Unmarshal(res.Body(), &obj)

	return getString(obj, "id")
}
func oneDriveCreate(config *OneDriveStorageConfig, item string, fileName string) string {
	fileName = strings.Replace(fileName, ":", "-", -1)
	res, _ := oneDriveClient.R().SetHeader("authorization", "Bearer "+config.AccessToken).
		SetHeader("Content-Type", "application/json").
		SetBody(`{"@microsoft.graph.conflictBehavior":"replace"}`).
		Post(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s:/%s:/createUploadSession", item, fileName))
	var obj interface{}
	json.Unmarshal(res.Body(), &obj)
	return getString(obj, "uploadUrl")
}

// 创建文件夹，如果存在直接返回item id，不存在就创建再返回
func oneDriveMkDir(config *OneDriveStorageConfig, item, name string) string {
	// 查询当前目录下的所有子项
	res, err := oneDriveClient.R().
		SetHeader("Authorization", "Bearer "+config.AccessToken).
		Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/children", item))
	if err != nil {
		return ""
	}

	var result struct {
		Value []map[string]interface{} `json:"value"`
	}
	json.Unmarshal(res.Body(), &result)

	// 如果存在同名文件夹，直接返回它的 id
	for _, v := range result.Value {
		if getString(v, "name") == name {
			if _, ok := v["folder"]; ok { // 确认是文件夹
				return getString(v, "id")
			}
		}
	}

	// 否则创建新目录
	body := fmt.Sprintf(`{
		"name": "%s",
		"folder": {},
		"@microsoft.graph.conflictBehavior": "fail"
	}`, name)

	res, err = oneDriveClient.R().
		SetHeader("Authorization", "Bearer "+config.AccessToken).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/children", item))
	if err != nil {
		return ""
	}

	var obj map[string]interface{}
	json.Unmarshal(res.Body(), &obj)
	return getString(obj, "id")
}
func oneDriveUploadReader(config *OneDriveStorageConfig, from, to, max int, path string, body io.Reader) int {
	request, _ := http.NewRequest("PUT", path, body)
	request.Header.Add("Authorization", "Bearer "+config.AccessToken)
	request.Header.Add("Content-Type", "application/octet-stream")
	request.Header.Add("Content-Length", toString(int64(to-from)))

	do, e := http.DefaultClient.Do(request)
	if e != nil {
		time.Sleep(1 * time.Second)
		do, e = http.DefaultClient.Do(request)
	}
	if e != nil {
		time.Sleep(1 * time.Second)
		do, e = http.DefaultClient.Do(request)
	}
	if e != nil {
		time.Sleep(1 * time.Second)
		do, e = http.DefaultClient.Do(request)
	}

	defer do.Body.Close()
	readAll, _ := io.ReadAll(do.Body)
	log.Println(string(readAll))

	return do.StatusCode

}
func oneDriveUpload(config *OneDriveStorageConfig, from, to, max int64, path string, content []byte) (int, *resty.Response) {

	var resp *resty.Response
	expectedLen := to - from + 1
	contentLen := int64(len(content))

	if contentLen == expectedLen {
		r, e := oneDriveClient.R().
			SetHeader("authorization", "Bearer "+config.AccessToken).
			SetHeader("Content-Length", strconv.Itoa(len(content))).
			SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to, max)).
			SetBody(content).
			Put(path)

		if e != nil {
			log.Println(e)
			time.Now()
		}
		if r == nil {
			time.Now()
		} else {
			//log.Println(r.String())
		}

		return r.StatusCode(), r

	}

	// 内容小于预期，需要填充
	if contentLen < expectedLen {
		paddingNeeded := expectedLen - contentLen

		// 如果填充量小于等于32MB，直接创建完整数组一次上传
		if paddingNeeded <= int64(32*1024*1024) {
			log.Println("hit")
			fullContent := make([]byte, expectedLen)
			copy(fullContent, content)

			r, e := oneDriveClient.R().
				SetHeader("authorization", "Bearer "+config.AccessToken).
				SetHeader("Content-Length", strconv.FormatInt(expectedLen, 10)).
				SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to, max)).
				SetBody(fullContent).
				Put(path)
			if e != nil {
				log.Println(e)
				time.Now()
			}
			if r == nil {
				time.Now()
			} else {
				log.Println(r.String())
			}
			return r.StatusCode(), r
		}

		// 填充量大于32MB，先上传实际内容
		actualTo := from + contentLen - 1
		oneDriveClient.R().
			SetHeader("authorization", "Bearer "+config.AccessToken).
			SetHeader("Content-Length", strconv.FormatInt(contentLen, 10)).
			SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, actualTo, max)).
			SetBody(content).
			Put(path)

		// 再分块填充零字节
		currentFrom := actualTo + 1

		for currentFrom <= to {
			size := int64(32 * 1024 * 1024)
			remaining := to - currentFrom + 1
			if remaining < size {
				size = remaining
			}

			currentTo := currentFrom + size - 1

			chunk := make([]byte, size)
			var last = false
			if max == currentTo {
				currentTo--
				chunk = chunk[:len(chunk)-1]
				last = true
			}
			r, err := oneDriveClient.R().
				SetHeader("authorization", "Bearer "+config.AccessToken).
				SetHeader("Content-Length", strconv.FormatInt(size, 10)).
				SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", currentFrom, currentTo, max)).
				SetBody(chunk).
				Put(path)

			if err != nil {
				fmt.Println(err.Error())
			}
			if r == nil {
				time.Now()
			} else {
				resp = r
				log.Println(r.String())
			}

			if last {

				return r.StatusCode(), r
			}

			currentFrom = currentTo + 1
		}
	}
	return 200, resp
}
func oneDriveInit(config *OneDriveStorageConfig) {
	config.AccessToken = accessToken(config)
	config.RootID = oneDriveDict(config, "Live")
	config.ChunkSize = (config.ChunkSize + 187) / 188 * 188
	log.Printf("[Onedrive] root_id %v]\n", config.RootID)

	go func() {
		for {
			time.Sleep(30 * time.Minute)
			config.AccessToken = accessToken(config)

		}
	}()
}

func oneDriveDownload(config *OneDriveStorageConfig, item string) string {
	var t = resty.New()
	t.SetRedirectPolicy(resty.NoRedirectPolicy())
	res, _ := t.R().SetHeader("authorization", "Bearer "+config.AccessToken).Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/content", item))
	return res.Header().Get("Location")
}

func oneDriveDelete(config *OneDriveStorageConfig, item string) {
	response, e0 := oneDriveClient.SetHeader("authorization", "Bearer "+config.AccessToken).R().Delete(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s", item))
	if e0 != nil {
		log.Println(e0)
	}
	log.Println(response.String())
}

func oneDriveList(config *OneDriveStorageConfig, item string) []OneDriveItem {
	res, _ := oneDriveClient.R().SetHeader("authorization", "Bearer "+config.AccessToken).Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s/children", item))
	var obj map[string]interface{}
	json.Unmarshal(res.Body(), &obj)
	var dst []OneDriveItem
	for _, i := range getArray(obj, "value") {
		dst = append(dst, OneDriveItem{
			ID:   getString(i, "id"),
			Name: getString(i, "name"),
		})
	}
	return dst
}

type OneDriveSession struct {
	Offset         int64
	URL            string
	OneDrive       *OneDriveStorageConfig
	live           Live
	onedriveFolder string
	Interval       int
	dataCh         chan []byte
	done           chan struct{}
	exited         chan struct{}
	retry          int
	fileName       string
	ext            string
	max            int64
}

func (session *OneDriveSession) Append(r io.ReadCloser) error {
	defer r.Close()
	buf := make([]byte, 64*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])

			session.dataCh <- chunk
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (session *OneDriveSession) Shutdown() {
	close(session.done)
	<-session.exited // 等待 goroutine 退出
}

func NewOneDriveSession(url string, oneDrive *OneDriveStorageConfig, live Live, id string, fileName, ext string, max int64) *OneDriveSession {
	session := &OneDriveSession{
		dataCh:         make(chan []byte, 16),
		done:           make(chan struct{}),
		exited:         make(chan struct{}),
		OneDrive:       oneDrive,
		URL:            url,
		Interval:       20,
		retry:          3,
		live:           live,
		onedriveFolder: id,
		fileName:       fileName,
		ext:            ext,
		max:            max,
	}
	go func() {
		defer close(session.exited)
		var buf []byte
		ticker := time.NewTicker(time.Duration(session.Interval) * time.Second)
		defer ticker.Stop()
		flush := func() {
			if len(buf) == 0 {
				return
			}
			session.upload(buf)
			buf = buf[:0]
		}
		for {
			select {
			case data := <-session.dataCh:
				buf = append(buf, data...)
			case <-ticker.C:
				flush()
			case <-session.done:
				for {
					select {
					case data := <-session.dataCh:
						buf = append(buf, data...)
					default:
						goto drainComplete
					}
				}
			drainComplete:
				flush()
				expectedSize := session.max
				if session.Offset < expectedSize {
					//paddingSize := expectedSize - session.Offset
					//log.Printf("需要填充 %d 字节的 0 以达到目标大小 %d", paddingSize, expectedSize)

					const maxPaddingChunk = 32 * 1024 * 1024
					for session.Offset+1 < expectedSize {
						remaining := expectedSize - session.Offset
						chunkSize := remaining
						if chunkSize > maxPaddingChunk {
							chunkSize = maxPaddingChunk
						}
						padding := make([]byte, chunkSize)
						session.upload(padding)
					}

					//log.Printf("填充完成，最终 Offset: %d", session.Offset)
				}

				return
			}
		}
	}()
	return session
}
func (session *OneDriveSession) upload(data []byte) {

	//var start = time.Now()

	var to = session.Offset + int64(len(data)) - 1

	if session.max-to < 1024*1024*15 {
		to = session.max - 1
	}

	var tried = 0

	var code = 0
	var r0 *resty.Response
	//重试是必要的，OneDrive的api经常不稳定。。。

	for {
		code, r0 = oneDriveUpload(session.OneDrive, session.Offset, to, session.max, session.URL, data)
		//log.Println(fmt.Sprintf("%d,%d,%d,%d", code, session.Offset, to, time.Now().Sub(start).Milliseconds()))

		if (code >= 200 && code <= 299) || tried >= session.retry {
			break
		}
		tried++
	}

	//code, r0 := oneDriveUpload(session.OneDrive, session.Offset, to, session.OneDrive.ChunkSize, session.URL, data)
	//log.Println(fmt.Sprintf("%d,%d,%d,%d", code, session.Offset, to, time.Now().Sub(start).Milliseconds()))
	session.Offset = to
	if r0 == nil {
		fmt.Println("r0 = nil")
	}
	if r0 != nil && !strings.Contains(r0.String(), "createdBy") {
		//fmt.Println(r0.String())
	}

	if code == 201 && session.ext == ".mp4" {
		go func() {
			time.Sleep(time.Second * 60)
			if session.OneDrive.AutoConvert {
				var obj0 map[string]interface{}
				json.Unmarshal(r0.Body(), &obj0)
				var link = oneDriveDownload(session.OneDrive, getString(obj0, "id"))
				taskPool.Submit(func() {
					res, e := client.R().SetHeader("Range", fmt.Sprintf("bytes=0-%d", session.Offset)).SetDoNotParseResponse(true).Get(link)

					defer res.RawBody().Close()
					out, _ := os.Create(session.fileName)
					defer out.Close()
					_, err = io.Copy(out, res.RawBody())

					if e != nil {
						log.Println(e)
						return
					}

					cmd := exec.Command("ffmpeg", "-y", "-i", session.fileName, "-vcodec", "copy", "-acodec", "copy", strings.Replace(session.fileName, ".mp4", "-COVERT.mp4", 1))
					//output, _ := cmd.CombinedOutput()

					e = cmd.Run()

					if e != nil {
						//log.Println(string(output))
						log.Println(e)
						return
					} else {
						defer func() {
							os.Remove(strings.Replace(session.fileName, ".mp4", "-COVERT.mp4", 1))
							os.Remove(session.fileName)
						}()
					}

					open, e := os.Open(strings.Replace(session.fileName, ".mp4", "-COVERT.mp4", 1))

					if e != nil {
						log.Println(e)
						return
					}

					stat, e := os.Stat(open.Name())

					var forward0 = getDstByLabel(session.OneDrive.ForwardTo)

					if forward0 != nil && forward0.Type() == "139" {
						var forward = forward0.(*CMStorageConfig)
						var instance = forward.instance
						var filtered = lo.Filter(instance.ListFiles(forward.RootDir), func(item DriveItem, index int) bool {
							return item.FileName == session.live.UName
						})
						var folderId = ""
						if len(filtered) == 0 {
							folderId = instance.MkDir(forward.RootDir, session.live.UName)
						} else {
							folderId = filtered[0].FileID
						}
						filtered = lo.Filter(instance.ListFiles(folderId), func(item DriveItem, index int) bool {
							return item.FileName == strings.ReplaceAll(session.live.Time.Format(time.DateTime), ":", "-")
						})
						if len(filtered) == 0 {
							folderId = instance.MkDir(folderId, strings.ReplaceAll(session.live.Time.Format(time.DateTime), ":", "-"))
						} else {
							folderId = filtered[0].FileID
						}
						instance.UploadFile(open.Name(), folderId, open.Name())

						time.Sleep(30 * time.Second)

						for _, i := range instance.ListFiles(folderId) {
							if i.FileName == open.Name() {
								oneDriveDelete(session.OneDrive, getString(obj0, "id"))
								break
							}
						}

						return
					}

					var u = oneDriveCreate(session.OneDrive, session.onedriveFolder, strings.Replace(session.fileName, ".mp4", "-CONVERT.mp4", 1))

					var from int64 = 0
					var CHUNK_SIZE int64 = 1024 * 1024 * 100

					var remain = 3

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
							log.Println(e0)
							remain--
							if remain <= 0 {
								break
							}
							time.Sleep(time.Second - 3)
							continue
						}

						respBytes, _ := io.ReadAll(res0.Body)
						res0.Body.Close()
						log.Println(string(respBytes))

						if res0.StatusCode == 200 || res0.StatusCode == 201 {
							oneDriveDelete(session.OneDrive, getString(obj0, "id"))
							break
						} else if res0.StatusCode == 202 {
							from = to
							remain = 3
						} else {
							//log.Printf(strconv.Itoa(res0.StatusCode))
							remain--
							if remain <= 0 {
								break
							}
						}

						if to >= stat.Size() {
							break
						}
					}

				})
			}
		}()
	}

}
