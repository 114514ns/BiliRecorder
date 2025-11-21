package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func accessToken(config *OneDriveStorageConfig) string {
	res, err := biliClient.Resty.R().
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
		panic(err)
	}
	var obj interface{}
	json.Unmarshal(res.Body(), &obj)
	return getString(obj, "access_token")
}

func oneDriveDict(config *OneDriveStorageConfig, name string) string {
	res, _ := client.R().SetHeader("authorization", "Bearer "+config.AccessToken).
		Get(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/root:/%s", name))

	var obj interface{}
	json.Unmarshal(res.Body(), &obj)

	return getString(obj, "id")
}
func oneDriveCreate(config *OneDriveStorageConfig, item string, fileName string) string {
	fileName = strings.Replace(fileName, ":", "-", -1)
	res, _ := client.R().SetHeader("authorization", "Bearer "+config.AccessToken).SetHeader("Content-Type", "application/json").SetBody("{}").
		Post(fmt.Sprintf("https://graph.microsoft.com/v1.0/me/drive/items/%s:/%s:/createUploadSession", item, fileName))
	var obj interface{}
	json.Unmarshal(res.Body(), &obj)
	return getString(obj, "uploadUrl")
}

// 创建文件夹，如果存在直接返回item id，不存在就创建再返回
func oneDriveMkDir(config *OneDriveStorageConfig, item, name string) string {
	// 查询当前目录下的所有子项
	res, err := client.R().
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

	res, err = client.R().
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
func oneDriveUpload(config *OneDriveStorageConfig, from, to, max int64, path string, content []byte) {
	expectedLen := to - from + 1
	contentLen := int64(len(content))

	// 如果内容大小符合预期，直接上传
	if contentLen == expectedLen {
		client.R().
			SetHeader("authorization", "Bearer "+config.AccessToken).
			SetHeader("Content-Length", strconv.Itoa(len(content))).
			SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to, max)).
			SetBody(content).
			Put(path)
		return
	}

	// 内容小于预期，需要填充
	if contentLen < expectedLen {
		paddingNeeded := expectedLen - contentLen

		// 如果填充量小于等于32MB，直接创建完整数组一次上传
		if paddingNeeded <= int64(32*1024*1024) {
			fullContent := make([]byte, expectedLen)
			copy(fullContent, content)

			client.R().
				SetHeader("authorization", "Bearer "+config.AccessToken).
				SetHeader("Content-Length", strconv.FormatInt(expectedLen, 10)).
				SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to, max)).
				SetBody(fullContent).
				Put(path)
			return
		}

		// 填充量大于32MB，先上传实际内容
		actualTo := from + contentLen - 1
		client.R().
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
			client.R().
				SetHeader("authorization", "Bearer "+config.AccessToken).
				SetHeader("Content-Length", strconv.FormatInt(size, 10)).
				SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", currentFrom, currentTo, max)).
				SetBody(chunk).
				Put(path)

			if last {
				return
			}

			currentFrom = currentTo + 1
		}
	}
}
func oneDriveInit(config *OneDriveStorageConfig) {
	config.AccessToken = accessToken(config)
	config.RootID = oneDriveDict(config, "Live")
	config.ChunkSize = (config.ChunkSize + 187) / 188 * 188
	log.Printf("[Onedrive] root_id %v]\n", config.RootID)

	go func() {
		for {
			config.AccessToken = accessToken(config)
			time.Sleep(30 * time.Minute)
		}
	}()
}
