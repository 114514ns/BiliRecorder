package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type LocalStorageConfig struct {
	Location string //本地存储
}
type AlistStorageConfig struct {
	Server  string
	User    string //Alist
	Pass    string
	RootDir string
}
type S3StorageConfig struct {
	User    string //S3，支持边录边传,不过成本太高，也许不会实现，先占个位
	Pass    string
	RootDir string
}
type OneDriveStorageConfig struct {
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
	ReEncoding bool   //重新编码
	Encoder    string //编码器
	VADevice   string //VA API设备，如没有或不需要，留空即可
	Bitrate    int    //目标码率，单位KB，如不需要，留空即可
	ChunkTime  int    //每块到多大的时候开始编码
	KeepTemp   bool   //保留分片，调试用
	Dst        Storage
	WithEvents bool //记录直播间事件
}
type Config struct {
	Cookie         string
	ProxyServer    string
	ProxyUser      string
	ProxyPass      string
	GlobalConfig   RoomConfig
	OverrideConfig map[int]RoomConfig
	Port           int //api端口
	Livers         []int64
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

func (r *RoomConfig) UnmarshalJSON(data []byte) error {
	// 先解析到临时结构体
	type Alias RoomConfig
	aux := &struct {
		Dst json.RawMessage `json:"Dst"` // 暂不解析 Dst
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 解析 Dst 字段，需要先判断类型
	if len(aux.Dst) > 0 {
		var typeCheck struct {
			Type string `json:"type"` // 假设 JSON 中有 type 字段标识类型
		}
		if err := json.Unmarshal(aux.Dst, &typeCheck); err != nil {
			return err
		}

		// 根据 type 字段选择具体类型
		switch typeCheck.Type {
		case "local":
			var local LocalStorageConfig

			if err := json.Unmarshal(aux.Dst, &local); err != nil {
				return err
			}
			r.Dst = local
		case "s3":
			var s3 S3StorageConfig
			if err := json.Unmarshal(aux.Dst, &s3); err != nil {
				return err
			}
			r.Dst = s3
		case "alist":
			var alist AlistStorageConfig
			if err := json.Unmarshal(aux.Dst, &alist); err != nil {
				return err
			}
			r.Dst = alist
		case "onedrive":
			var onedrive *OneDriveStorageConfig
			if err := json.Unmarshal(aux.Dst, &onedrive); err != nil {
				return err
			}
			onedrive.Type()
			r.Dst = onedrive
		default:
			return fmt.Errorf("unknown storage type: %s", typeCheck.Type)
		}
	}

	return nil
}

func (r RoomConfig) MarshalJSON() ([]byte, error) {
	type Alias RoomConfig
	aux := &struct {
		Dst any `json:"Dst"`
		*Alias
	}{
		Alias: (*Alias)(&r),
	}

	// 序列化 Dst，补上 type 字段
	if r.Dst != nil {
		// d 是原本的配置结构体
		d, err := json.Marshal(r.Dst)
		if err != nil {
			return nil, err
		}

		// 转为 map，加上 type 字段
		var m map[string]any
		if err := json.Unmarshal(d, &m); err != nil {
			return nil, err
		}
		m["Type"] = r.Dst.Type()

		aux.Dst = m
	}

	return json.Marshal(aux)
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
