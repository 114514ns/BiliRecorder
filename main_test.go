package main

import (
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
