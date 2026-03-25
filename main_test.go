package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)
import _ "net/http/pprof"

func TestExtract(t *testing.T) {
	bytes1, _ := os.ReadFile("input.ts")
	var start = time.Now()
	fmt.Println(len(Extract(bytes1)))
	fmt.Println(time.Now().Sub(start))
}

func TestHttp(t *testing.T) {
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	loadConfig()
	config.Port++

	InitHTTP()

}

func TestTranscribe(t *testing.T) {
	bytes, _ := os.ReadFile("sample/2.json")
	var obj map[string]interface{}
	json.Unmarshal(bytes, &obj)
	parseTranscribe(obj)
}

func TestOneDrive(t *testing.T) {
	loadConfig()
	var oneDrive OneDriveStorageConfig
	for _, i := range config.Storages {
		if getString(i, "Type") == "onedrive" {
			oneDrive = (getDstByLabel(getString(i, "Label")).(OneDriveStorageConfig))
		}
	}
	fmt.Println(oneDriveDownload(&oneDrive, "01TNAFOAYTED2DBQDJC5FIOFCGHVYF6CNT"))

}
