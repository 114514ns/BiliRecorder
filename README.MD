# BiliRecorder

# Features
- 录制视频流
- 录制音频流
- 录制结束后自动转码
- 上传到Alist
- 上传到OneDrive，数据不落盘
  - 如果选择fmp4模式，主播pk部分会花屏（除非分辨率不变)，需要登录，否则拿不到高画质
  - 如果选择ts模式，在浏览器和手机上无法拖到进度条，pc端本地播放正常。可以选择上传完成后再ffmpeg转封装。可免登录获取高画质的流

# TODO
- 弹幕
- webui
- DOC
- 语音转录

# 配置示例
```json
{
  "Cookie": "", //Cookie
  "ProxyServer": "****", // 代理配置，即将废弃
  "ProxyUser": "****",
  "ProxyPass": "****",
  "GlobalConfig": {
    "ReEncoding": false, //是否重新编码，仅本地dst生效
    "Encoder": "libsvtav1", //编码器
    "VADevice": "/dev/dri/renderD128", //是否使用vaapi，如不需要留空即可
    "Bitrate": 4500, //目标码率
    "ChunkTime": 3600, //每隔多久开始编码
    "KeepTemp": true, //保留编码前的文件
    "Dst": null,
    "DstLabel": "onedrive", //要保存到的位置
    "WithEvents": false, //记录直播间弹幕礼物SC上舰信息
    "EnableTranscribe": false, //启用语音转录
    "AudioCodec": "", //音频编码器，如不需要重新编码，留空即可
    "Container": "ts", //使用的容器格式。
    "AutoConvert": true //对于保存到OneDrive的情况，是否在每一块上传完成后自动转封装为mp4
  },
  "OverrideConfig": {},
  "Port": 8099, //api端口
  "Livers": [ //要录制的主播。只需填写UID字段即可，UName会自动填充和修正
    {
      "UID": 1298779265,
      "UName": "灰泽满Hazel"
    },
    {
      "UID": 3546757543758795,
      "UName": "铃见Suzumi"
    },
    {
      "UID": 3493260125276695,
      "UName": "ハイルラプラス"
    },
    {
      "UID": 1789460279,
      "UName": "弥月Mizuki"
    }
  ],
  "Storages": [ //存储
    {
      "Label": "local",
      "Location": "/root/dst",
      "Type": "local"
    },
    {
      "AccessToken": "*********",
      "AudioChunkSize": 268435456, //每一块音频的大小
      "BufferChunk": 2, //积攒到多少个ts分片再上传。如果录制的主播较多，需适当调高这个值
      "ChunkSize": 1073741884, //每一块视频的大小
      "ClientID": "***********",
      "ClientSecret": "*************",
      "Label": "onedrive",
      "RedirectURL": "https://api.oplist.org/onedrive/callback",
      "RefreshToken": "**************",
      "Retry": 3,
      "RootDir": "Live",
      "RootID": "***************",
      "Type": "onedrive"
    }
    
  ],
  "BiliProxy": "",
  "StreamProxy": "",
  "OneDriveProxy": ""
}
```