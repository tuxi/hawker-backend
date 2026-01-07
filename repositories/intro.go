package repositories

import (
	"hawker-backend/models"
	"sync"
)

// 仓库接口
type IntroRepository interface {
	GetPathByID(id string, voiceType string) string
	FindByTime(hour int, voiceType string) *models.IntroTemplate
}
type MemIntroRepository struct {
	templates []models.IntroTemplate
	mu        sync.RWMutex
}

func NewMemIntroRepository() *MemIntroRepository {
	return &MemIntroRepository{
		templates: make([]models.IntroTemplate, 0),
	}
}

// GetPathByID 根据 ID 和音色查找
func (r *MemIntroRepository) GetPathByID(id string, voiceType string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.templates {
		if t.ID == id && t.VoiceType == voiceType {
			return t.AudioURL
		}
	}
	return ""
}

// FindByTime 根据当前小时和音色查找匹配的开场白
func (r *MemIntroRepository) FindByTime(hour int, voiceType string) *models.IntroTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.templates {
		// 逻辑：匹配音色 且 小时在范围内
		if t.VoiceType == voiceType && hour >= t.TimeRange[0] && hour < t.TimeRange[1] {
			return &t
		}
	}
	return nil
}

// AddTemplate 手动或自动添加模版
func (r *MemIntroRepository) AddTemplate(t models.IntroTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates = append(r.templates, t)
}
