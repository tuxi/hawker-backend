package services

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
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
	VoiceType   string
	StaticDir   string
}

// 对应官方的 defaultHeader: version=1, head_size=4, full_request, json, gzip
var volcHeader = []byte{0x11, 0x10, 0x11, 0x00}

func NewDoubaoAudioService(appID, token, cluster, voice, staticDir string) *DoubaoAudioService {
	return &DoubaoAudioService{
		AppID:       appID,
		AccessToken: token,
		ClusterID:   cluster,
		VoiceType:   voice,
		StaticDir:   staticDir,
	}
}

func (s *DoubaoAudioService) GenerateAudio(ctx context.Context, text string, identifier string) (string, error) {
	fileName := fmt.Sprintf("%s.mp3", identifier)
	fullPath := filepath.Join(s.StaticDir, fileName)

	// 1. 准备 JSON 请求体并进行 Gzip 压缩 (官方 Demo 要求)
	inputJSON := s.makeRequestJSON(text)
	compressedJSON := s.gzipCompress(inputJSON)

	// 2. 构造完整的二进制包: Header(4B) + PayloadSize(4B) + Payload
	payloadSize := len(compressedJSON)
	clientRequest := make([]byte, 0, 8+payloadSize)
	clientRequest = append(clientRequest, volcHeader...)

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, uint32(payloadSize))
	clientRequest = append(clientRequest, sizeBytes...)
	clientRequest = append(clientRequest, compressedJSON...)

	// 3. 建立连接并发送
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

	// 4. 循环读取响应
	file, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	err = s.processResponse(conn, file)
	if err != nil {
		return "", err
	}
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

func (s *DoubaoAudioService) makeRequestJSON(text string) []byte {
	reqID := uuid.New().String()
	req := map[string]interface{}{
		"app": map[string]interface{}{
			"appid":   s.AppID,
			"token":   s.AccessToken,
			"cluster": s.ClusterID,
		},
		"user": map[string]interface{}{"uid": "hawker_go_cli"},
		"audio": map[string]interface{}{
			"voice_type":   s.VoiceType,
			"encoding":     "mp3",
			"speed_ratio":  1.0,
			"volume_ratio": 1.0,
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
