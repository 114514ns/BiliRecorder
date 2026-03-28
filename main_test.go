package main

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)
import _ "net/http/pprof"

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

func TestM(t *testing.T) {
	main()
}
