package services

import (
	"context"
	"fmt"
	"hawker-backend/conf"
	"testing"
)

func TestDoubaoTTS(t *testing.T) {
	cfg, err := config.LoadConfig("../config/config.yaml")
	if err != nil {
		panic(err)
	}

	svc := NewDoubaoAudioService(cfg.Volcengine.AppID, cfg.Volcengine.AccessToken, cfg.Volcengine.ClusterID, cfg.Volcengine.VoiceType, "./static")
	url, err := svc.GenerateAudio(context.Background(), "走过路过不要错过，五花肉降价啦，快来买呀！", "test_voice")
	if err != nil {
		t.Fatalf("API 调通失败: %v", err)
	}
	fmt.Println("API 调用成功，音频路径:", url)
}
