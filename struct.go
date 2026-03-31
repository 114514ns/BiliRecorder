package main

import (
	"time"
)

type LocalStorageConfig struct {
	Location  string //本地存储
	Label     string
	ForwardTo string
}
type AlistStorageConfig struct {
	Label   string
	Server  string
	User    string //Alist
	Pass    string
	RootDir string
}

type S3StorageConfig struct {
	Label   string
	User    string //S3，支持边录边传,不过成本太高，也许不会实现，先占个位
	Pass    string
	RootDir string
}
type OneDriveStorageConfig struct {
	Label          string
	AccessToken    string
	RefreshToken   string
	ClientID       string
	ClientSecret   string
	RootDir        string
	RedirectURL    string
	RootID         string
	ChunkSize      int64
	AudioChunkSize int64
	BufferChunk    int
	Retry          int
}
type Storage interface {
	Type() string
}

func (s LocalStorageConfig) Type() string {
	return "local"
}
func (s S3StorageConfig) Type() string {
	return "s3"
}
func (s AlistStorageConfig) Type() string {
	return "alist"
}
func (s OneDriveStorageConfig) Type() string {
	return "onedrive"
}

type RoomConfig struct {
	ReEncoding       bool   //重新编码
	Encoder          string //编码器
	VADevice         string //VA API设备，如没有或不需要，留空即可
	Bitrate          int    //目标码率，单位KB，如不需要，留空即可
	ChunkTime        int    //每块到多大的时候开始编码
	KeepTemp         bool   //保留分片，调试用
	Dst              Storage
	DstLabel         string
	WithEvents       bool //记录直播间事件
	EnableTranscribe bool //转录
	AudioCodec       string
	Container        string //容器格式，ts/fmp4
	AutoConvert      bool   //仅OneDrive dst有效。每个分块上传完成后是否自动开始转换
}

type Liver struct {
	UID   int64
	UName string
}
type Config struct {
	Cookie         string
	ProxyServer    string
	ProxyUser      string
	ProxyPass      string
	GlobalConfig   RoomConfig
	OverrideConfig map[int]RoomConfig
	Port           int //api端口
	Livers         []Liver
	Storages       []interface{}
	BiliProxy      string //请求b站api的代理
	StreamProxy    string
	OneDriveProxy  string
}

type Live struct {
	UName         string
	Title         string
	UID           int64
	Time          time.Time
	End           time.Time
	LastChunkSize int64
	Cover         string
}

type RoomStatus struct {
	Stream              string // 视频流
	Title               string
	UName               string
	UID                 int64
	Room                int
	Live                time.Time   //直播开始时间
	Record              time.Time   //录制开始时间
	ChunkRecord         []time.Time //每个分片的开始时间
	End                 bool
	OnedriveOffset      int64
	OnedriveAudioOffset int64
	BufferChunk         int
	BufferBytes         []byte    `json:"-"` //视频流
	AudioBufferBytes    []byte    `json:"-"`
	ChunkBegin          time.Time `json:"-"`
	BitRate             float64
}

type MetaData struct {
	RoomConfig
}

var cacheDst = make(map[string]Storage)

func getDstByLabel(s string) Storage {
	v, ok := cacheDst[s]
	if ok {
		return v
	}
	for _, i := range config.Storages {
		if getString(i, "Label") == s {
			if getString(i, "Type") == "local" {
				var storage = &LocalStorageConfig{
					Location: getString(i, "Location"),
				}
				cacheDst[s] = storage
				return storage
			}
			if getString(i, "Type") == "onedrive" {
				var storage = &OneDriveStorageConfig{
					AccessToken:    getString(i, "AccessToken"),
					AudioChunkSize: getInt64(i, "AudioChunkSize"),
					ChunkSize:      getInt64(i, "ChunkSize"),
					ClientID:       getString(i, "ClientID"),
					ClientSecret:   getString(i, "ClientSecret"),
					RedirectURL:    getString(i, "RedirectURL"),
					RefreshToken:   getString(i, "RefreshToken"),
					Retry:          getInt(i, "Retry"),
					RootDir:        getString(i, "RootDir"),
					RootID:         getString(i, "RootID"),
					BufferChunk:    getInt(i, "BufferChunk"),
				}
				cacheDst[s] = storage
				oneDriveInit(storage)
				return storage
			}
			return nil
		}
	}
	return nil
}
