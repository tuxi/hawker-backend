package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hawker-backend/models"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type DoubaoAudioService struct {
	AppID       string
	AccessToken string
	ClusterID   string
	StaticDir   string
}

// 对应官方的 defaultHeader: version=1, head_size=4, full_request, json, gzip
var volcHeader = []byte{0x11, 0x10, 0x11, 0x00}

func NewDoubaoAudioService(appID, token, cluster, staticDir string) *DoubaoAudioService {
	return &DoubaoAudioService{
		AppID:       appID,
		AccessToken: token,
		ClusterID:   cluster,
		StaticDir:   staticDir,
	}
}
func (s *DoubaoAudioService) GenerateAudio(ctx context.Context, text string, identifier string, voiceType string) (string, error) {
	// 1. 处理路径：支持 "intros/morning_sunny" 这种格式
	fileName := fmt.Sprintf("%s.mp3", identifier)
	fullPath := filepath.Join(s.StaticDir, fileName)

	// 🌟 核心改进：自动创建子目录 (例如 static/audio/intros/)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	// 2. 准备数据包 (保持原有逻辑)
	inputJSON := s.makeRequestJSON(text, voiceType)
	compressedJSON := s.gzipCompress(inputJSON)
	payloadSize := len(compressedJSON)
	clientRequest := make([]byte, 0, 8+payloadSize)
	clientRequest = append(clientRequest, volcHeader...)
	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(payloadSize))
	clientRequest = append(clientRequest, sizeBytes...)
	clientRequest = append(clientRequest, compressedJSON...)

	// 3. 建立连接
	header := http.Header{"Authorization": []string{fmt.Sprintf("Bearer;%s", s.AccessToken)}}
	addr := "wss://openspeech.bytedance.com/api/v1/tts/ws_binary"
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, addr, header)
	if err != nil {
		return "", fmt.Errorf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.BinaryMessage, clientRequest); err != nil {
		return "", fmt.Errorf("write failed: %v", err)
	}

	// 🌟 核心改进：使用临时文件防止残缺文件被 App 缓存
	tempPath := fullPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return "", err
	}

	// 4. 读取响应并写入
	err = s.processResponse(conn, file)
	file.Close() // 必须先关闭句柄才能重命名

	if err != nil {
		os.Remove(tempPath) // 出错则清理临时文件
		return "", err
	}

	// 🌟 将临时文件原子性地重命名为最终文件
	if err := os.Rename(tempPath, fullPath); err != nil {
		return "", fmt.Errorf("failed to finalize audio file: %v", err)
	}

	// 返回给前端的相对 URL
	// 注意：如果是 intros/xxx，这里拼接出来的也是 /static/audio/intros/xxx.mp3
	return "/static/audio/" + fileName, nil
}

// gzipCompress 实现官方 Demo 的压缩逻辑
func (s *DoubaoAudioService) gzipCompress(input []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(input)
	w.Close()
	return b.Bytes()
}

func (s *DoubaoAudioService) makeRequestJSON(text string, voiceType string) []byte {
	realVoiceID := s.GetRealVoiceID(voiceType)

	reqID := uuid.New().String()
	req := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   s.AppID,
			"token":   s.AccessToken,
			"cluster": s.ClusterID,
		},
		"user": map[string]interface{}{"uid": "hawker_go_cli"},
		"audio": map[string]interface{}{
			"voice_type":   realVoiceID,
			"encoding":     "mp3",
			"speed_ratio":  1.0,
			"volume_ratio": 1.0,
			"pitch_ratio":  1.0,
		},
		"request": map[string]interface{}{
			"reqid":     reqID,
			"text":      text,
			"text_type": "plain",
			"operation": "query",
		},
	}
	data, _ := json.Marshal(req)
	return data
}

func (s *DoubaoAudioService) processResponse(conn *websocket.Conn, w io.Writer) error {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return nil
		}

		if len(message) < 8 {
			continue
		}

		messageType := message[1] >> 4
		// 检查第 2 字节（索引为 2）的低 4 位是否为 1 (代表有压缩)
		isCompressed := (message[2] & 0x0f) == 1

		if messageType == 0xb { // 音频数据
			// 官方 Demo 逻辑：音频数据从 8 字节开始
			w.Write(message[8:])
			seq := int32(binary.BigEndian.Uint32(message[4:8]))
			if seq < 0 {
				return nil
			}
		} else if messageType == 0xf { // 错误信息
			rawPayload := message[8:]

			if isCompressed {
				// 【核心修正】在 Payload 中寻找 Gzip 的起始标志 0x1f 0x8b
				startIndex := bytes.Index(rawPayload, []byte{0x1f, 0x8b})
				if startIndex != -1 {
					decoded, err := s.gzipDecompress(rawPayload[startIndex:])
					if err == nil {
						return fmt.Errorf("火山引擎明文报错: %s", string(decoded))
					}
				}
			}
			return fmt.Errorf("无法解压的错误消息(Hex): %X", rawPayload)
		}
	}
}

// 增加解压辅助函数
func (s *DoubaoAudioService) gzipDecompress(input []byte) ([]byte, error) {
	if len(input) < 2 {
		return input, nil
	}
	// Gzip 魔数检查: 1f 8b
	if input[0] != 0x1f || input[1] != 0x8b {
		return input, nil
	}

	r, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out, err := io.ReadAll(r)
	return out, err
}

func (s *DoubaoAudioService) GetRealVoiceID(voiceType string) string {
	// 映射业务标识到火山引擎真实 ID
	realVoiceID := "zh_male_M392_conversation_wvae_bigtts" // 默认阳光青年

	//switch voiceType {
	//case VoiceSunnyBoy:
	//	realVoiceID = "bv001_streaming" // 灿烂阳光青年
	//case VoiceSoftGirl:
	//	realVoiceID = "bv051_streaming" // 亲切邻居大姐
	//case VoicePromoBoss:
	//	realVoiceID = "bv700_streaming" // 热血卖货大叔
	//case VoiceSweetGirl:
	//	realVoiceID = "bv002_streaming" // 甜美温柔少女
	//}

	switch voiceType {
	case models.VoiceSunnyBoy:
		realVoiceID = "zh_male_M392_conversation_wvae_bigtts" // 灿烂阳光青年
	case models.VoiceSoftGirl:
		realVoiceID = "zh_female_vv_uranus_bigtts" // 亲切邻居大姐
	case models.VoicePromoBoss:
		realVoiceID = "zh_male_yuanboxiaoshu_moon_bigtts" // 热血卖货大叔
	case models.VoiceSweetGirl:
		realVoiceID = "zh_female_xiaohe_uranus_bigtts" // 甜美温柔少女
	}

	return realVoiceID
}
