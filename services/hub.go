package services

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// 定义一个简单的连接池
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 允许跨域
}

type Hub struct {
	clients map[*websocket.Conn]bool
	mu      sync.Mutex
}

var GlobalHub = &Hub{
	clients: make(map[*websocket.Conn]bool),
}

// HandleWS 处理 App 的连接请求
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

// Broadcast 发送叫卖指令给所有连接的设备
func (h *Hub) Broadcast(audioURL string, text string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	payload := map[string]string{
		"type":      "HAWKING_TASK",
		"audio_url": audioURL,
		"text":      text,
	}
	for client := range h.clients {
		err := client.WriteJSON(payload)
		if err != nil {
			client.Close()
			delete(h.clients, client)
		}
	}
}
