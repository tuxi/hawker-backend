package models

type HawkingTask struct {
	ProductID     string  `json:"product_id"`
	AudioURL      string  `json:"audio_url"`
	Text          string  `json:"text"` // 如果用户传了全文，优先用这个
	Scene         string  `json:"scene"`
	Price         float64 `json:"price"`          // 👈 新增：临时现价
	OriginalPrice float64 `json:"original_price"` // 👈 新增：临时原价
}

// 定义推送给 Swift 的包装结构
type TaskBundle struct {
	Type string         `json:"type"` // 例如 "TASK_CONF_UPDATE"
	Data []*HawkingTask `json:"data"`
}

type AddTaskReq struct {
	ProductID     string  `json:"product_id" binding:"required"`
	Text          string  `json:"text"`           // 用户完全自定义的文案
	Price         float64 `json:"price"`          // 现价
	OriginalPrice float64 `json:"original_price"` // 原价
}
