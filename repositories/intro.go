package repositories

import (
	"hawker-backend/models"
	"sync"
)

// 仓库接口
type IntroRepository interface {
	FindByID(id string, voiceType string) *models.IntroTemplate
	FindByTime(hour int, voiceType string) *models.IntroTemplate
	FindAllByVoice(voiceType string) []*models.IntroTemplate
	FindAllByTime(hour int, voiceType string) []*models.IntroTemplate
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
func (r *MemIntroRepository) FindByID(id string, voiceType string) *models.IntroTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, t := range r.templates {
		if t.ID == id && t.VoiceType == voiceType {
			return &t
		}
	}
	return nil
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

func (r *MemIntroRepository) FindAllByVoice(voiceType string) []*models.IntroTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	templates := make([]*models.IntroTemplate, 0)
	for i := range r.templates {
		if r.templates[i].VoiceType == voiceType {
			// 直接存入指针
			templates = append(templates, &r.templates[i])
		}
	}
	return templates
}

// AddTemplate 手动或自动添加模版
func (r *MemIntroRepository) AddTemplate(t models.IntroTemplate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates = append(r.templates, t)
}

func (r *MemIntroRepository) FindAllByTime(hour int, voiceType string) []*models.IntroTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make([]*models.IntroTemplate, 0)

	for i := range r.templates {
		if r.templates[i].VoiceType == voiceType && hour >= r.templates[i].TimeRange[0] && hour < r.templates[i].TimeRange[1] {
			templates = append(templates, &r.templates[i])
		}
	}
	return templates
}
