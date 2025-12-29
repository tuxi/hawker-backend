package edge_tts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// 定义音色常量，方便调用
const (
	VoiceBoy    = "zh-CN-YunxiNeural"    // 活泼男声（首选）
	VoiceGirl   = "zh-CN-XiaoxiaoNeural" // 清脆女声（超市感）
	VoiceStrong = "zh-CN-YunjianNeural"  // 浑厚男声（力度感）
)

// 查找edge-tts执行路径
func getEdgeTTSPath() string {
	// 1. 尝试直接从系统 PATH 寻找 (适合 Ubuntu 部署后已经配置好 PATH 的情况)
	path, err := exec.LookPath("edge-tts")
	if err == nil {
		return path
	}

	// 2. 如果找不到，尝试常见的 Mac Python 脚本路径 (适合你本地开发)
	macPath := "/Users/xiaoyuan/Library/Python/3.9/bin/edge-tts"
	if _, err := os.Stat(macPath); err == nil {
		return macPath
	}

	// 3. 默认返回，让系统报错，或者根据你的 Ubuntu 实际安装路径再加一个候选
	return "edge-tts"
}

// GenerateAudio 调用 edge-tts 生成音频文件
// text: 文案, fileName: 文件名, voice: 音色, rate: 语速(如 +15%)
func GenerateAudio(text string, fileName string, voice string, rate string) (string, error) {
	outputDir := "./static/audio"
	// 1. 自动创建目录
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.MkdirAll(outputDir, 0755)
	}

	outputPath := filepath.Join(outputDir, fileName+".mp3")

	// 2. 自动判定 edge-tts 执行路径
	binPath := getEdgeTTSPath()

	// 3. 构造命令
	cmd := exec.Command(binPath,
		"--text", text,
		"--voice", voice,
		"--rate", rate,
		"--write-media", outputPath)

	// 4. 执行并捕获错误
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("TTS 生成失败: %v, 详情: %s", err, string(output))
	}

	return "/static/audio/" + fileName + ".mp3", nil
}
