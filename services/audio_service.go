package services

import (
	"context"
)

// AudioService 定义语音合成的标准接口
type AudioService interface {
	// GenerateAudio 返回音频文件的本地路径或 URL
	GenerateAudio(ctx context.Context, text string, identifier string, voiceType string) (string, error)
}
