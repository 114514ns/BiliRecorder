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
	if config.GlobalConfig.Dst.Type() == "onedrive" {
		var c = config.GlobalConfig.Dst.(*OneDriveStorageConfig)
		oneDrive = c
		oneDriveInit(oneDrive)
	}
	config.Port++

	InitHTTP()
}

func TestTranscribe(t *testing.T) {
	bytes, _ := os.ReadFile("sample/2.json")
	var obj map[string]interface{}
	json.Unmarshal(bytes, &obj)
	parseTranscribe(obj)
}
