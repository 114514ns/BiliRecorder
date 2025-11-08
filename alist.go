package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/bytedance/sonic"
)

func alistUploadFile(pr *io.PipeReader, alistPath string, token string, alistServer string) error {

	req, err := http.NewRequest("PUT", alistServer+"/api/fs/put", pr)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("File-Path", alistPath)
	req.Header.Set("Content-Type", "application/octet-stream")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[%s] %d %s\n", alistPath, resp.StatusCode, string(body))
	return nil
}
func alistGetToken(config AlistStorageConfig) string {
	type LoginResponse struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	type LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	var sum = sha256.Sum256([]byte(config.Pass + "-https://github.com/alist-org/alist"))
	var req = LoginRequest{Username: config.User, Password: hex.EncodeToString(sum[:])}
	alist, err := client.R().SetBody(req).Post(config.Server + "/api/auth/login/hash")

	if err != nil {
		log.Println(err)
	}
	var res = LoginResponse{}
	sonic.Unmarshal(alist.Body(), &res)
	return res.Data.Token
}
