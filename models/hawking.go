package models

type HawkingTask struct {
	ProductID     string  `json:"product_id"`
	AudioURL      string  `json:"audio_url"`
	Text          string  `json:"text"`        // 生成的、锁定的、用于合成的最终文本
	CustomText    string  `json:"custom_text"` // 用户手动输入的原始文本
	Scene         string  `json:"scene"`
	Price         float64 `json:"price"`          // 临时现价
	OriginalPrice float64 `json:"original_price"` // 临时原价
	Unit          string  `json:"unit"`           // 存储本次叫卖的特定单位
	VoiceType     string  `json:"voice_type"`

	// --- 新增条件促销字段 ---
	MinQty        float64 `json:"min_qty"`        // 触发优惠的门槛数量，如 2
	ConditionUnit string  `json:"condition_unit"` // 门槛单位，如 "斤" 或 "条"

	// 关键：标记该任务是否已经完成合成并下发过
	IsSynthesized bool

	PromotionTag  string `json:"promotion_tag"` // "特价", "秒杀"
	UseRepeatMode bool   `json:"use_repeat_mode"`
}

type HawkingIntro struct {
	AudioURL string `json:"audio_url"`
	Text     string `json:"text"`
	Scene    string `json:"scene"`
	// 可以增加 ID 方便客户端缓存
	IntroID   string `json:"intro_id"`
	StartHour int    `json:"start_hour"`
	EndHour   int    `json:"end_hour"`
	VoiceType string `json:"voice_type"`
}

// 定义推送给 Swift 的包装结构
type TaskBundle struct {
	Type string             `json:"type"` // 例如 "TASK_CONF_UPDATE"
	Data *TasksSnapshotData `json:"data"`
}

type AddTaskReq struct {
	SessionID     string  `json:"session_id" binding:"required"` // 👈 必须
	ProductID     string  `json:"product_id" binding:"required"`
	Text          string  `json:"text"`           // 用户完全自定义的文案
	Price         float64 `json:"price"`          // 现价
	OriginalPrice float64 `json:"original_price"` // 原价
	Unit          string  `json:"unit"`           // 👈 接收前端传来的 "3个" 或 "斤"

	// --- 新增条件促销字段 ---
	MinQty        float64 `json:"min_qty"`        // 触发优惠的门槛数量，如 2
	ConditionUnit string  `json:"condition_unit"` // 门槛单位，如 "斤" 或 "条"

	VoiceType string `json:"voice_type"` // 👈 用户选定的音色，如 "sunny_boy"
	IntroID   string `json:"intro_id"`   // 👈 用户指定的开场白 ID，"none" 表示不要

	PromotionTag string `json:"promotion_tag"` // "特价", "秒杀"

	// UseRepeatMode: 是否默认开启“复读机”喊法
	UseRepeatMode bool `gorm:"default:true" json:"use_repeat_mode"`
}

type SyncIntroReq struct {
	Text      string `json:"text"`
	VoiceType string `json:"voice_type"`
}

// 定义一个统一的消息外壳
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// 开场白模版
type IntroTemplate struct {
	ID        string
	Text      string
	VoiceType string // 音色标识
	SceneTag  string // 如: "default", "morning", "evening", "flash_sale"
	TimeRange [2]int // 适用小时段，如 [17, 20] 表示下午 5点到 8点
	AudioURL  string // 预合成好的音频路径
}

// 定义音色映射常量
const (
	VoiceSunnyBoy  = "sunny_boy"  // 阳光青年：适合水果、蔬菜，听起来新鲜有朝气
	VoiceSoftGirl  = "soft_girl"  // 亲切大姐：适合熟食、肉类，听起来靠谱、像邻居
	VoicePromoBoss = "promo_boss" // 卖货老板：适合海鲜、大促，嗓门大，有张力
	VoiceSweetGirl = "sweet_girl" // 甜美客服：适合零食、甜品，声音细腻
)

type TasksSnapshotData struct {
	// 候选开场白池：客户端根据当前正在播的任务音色从这里面选
	IntroPool []*HawkingIntro `json:"intro_pool"`
	// 所有的任务
	Products []*HawkingTask `json:"products"`
}
