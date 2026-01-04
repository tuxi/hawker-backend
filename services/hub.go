package services

import (
	"encoding/json"
	"hawker-backend/models"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Client 是连接与 Hub 之间的桥梁
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte // 每个客户端独立的待发送消息队列
}

// Hub 负责维护所有活跃客户端并处理消息广播
type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte  // 待广播的消息管道
	Register   chan *Client // 注册请求管道
	Unregister chan *Client // 注销请求管道
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
			log.Println("📱 新客户端已连接")
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				log.Println("👋 客户端已断开")
			}
		case message := <-h.Broadcast:
			// 异步分发给所有客户端，不阻塞广播管道
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}
		}
	}
}

// BroadcastHawking 暴露给调度器使用的结构化广播方法
func (h *Hub) BroadcastHawking(audioURL string, text string, productID string) {
	payload := map[string]string{
		"type":       "HAWKING_TASK",
		"audio_url":  audioURL,
		"text":       text,
		"product_id": productID,
	}
	message, _ := json.Marshal(payload)
	h.Broadcast <- message
}

func (h *Hub) BroadcastTaskBundle(tasks []*models.HawkingTask) {
	bundle := models.TaskBundle{
		Type: "TASK_CONF_UPDATE",
		Data: tasks,
	}
	payload, _ := json.Marshal(bundle)
	h.Broadcast <- payload
}

// --- Client 相关方法 ---

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	// 此处主要用于监听心跳或客户端主动关闭信号
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}
