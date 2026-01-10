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
	for i := range r.templates {
		// 通过索引访问，或者在循环内定义局部变量 t := r.templates[i]
		if r.templates[i].VoiceType == voiceType && hour >= r.templates[i].TimeRange[0] && hour < r.templates[i].TimeRange[1] {
			return &r.templates[i]
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
