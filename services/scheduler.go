package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"hawker-backend/logic"
	"hawker-backend/models"
	"hawker-backend/repositories"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type HawkingSession struct {
	ID        string
	VoiceType string
	// 该 Session 下的任务列表，key 是 ProductID
	ActiveTasks  map[string]*models.HawkingTask
	currentIntro *models.HawkingIntro
	mu           sync.RWMutex

	// 控制该 Session 的开关
	SessionCtx    context.Context // Session 的总开关（只有关闭 Session 时才取消）
	SessionCancel context.CancelFunc

	BatchCancel context.CancelFunc // 🌟 专门用于取消“当前这一波”合成任务

	taskNotify chan struct{}
	IsRunning  int32

	VoiceVersion int // 音色版本
}

// 建议的消息结构
type PlayEventData struct {
	SessionID string `json:"session_id"`
	ProductID string `json:"product_id"`
	// 🌟 只有在音色变更后的第一个任务，或者 Pool 发生变化时才携带，平时为 nil
	IntroPool []*models.HawkingIntro `json:"intro_pool,omitempty"`
	Product   *models.HawkingTask    `json:"product"`    // 商品叫卖任务
	VoiceType string                 `json:"voice_type"` // 全局同步音色
}

type HawkingScheduler struct {
	productRepo  repositories.ProductRepository
	introRepo    repositories.IntroRepository // 👈 新增：开场白仓库
	audioService AudioService
	Hub          *Hub

	sessions  map[string]*HawkingSession // 👈 管理多个 Session
	sessionMu sync.RWMutex
}

func NewHawkingScheduler(repo repositories.ProductRepository, introRepo repositories.IntroRepository, audio AudioService, hub *Hub) *HawkingScheduler {
	return &HawkingScheduler{
		productRepo:  repo,
		introRepo:    introRepo,
		audioService: audio,
		Hub:          hub,
		sessions:     make(map[string]*HawkingSession, 2),
	}
}

func (s *HawkingScheduler) StartSession(sessionID string, voiceType string) {
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	// 1. 如果 Session 已存在且在运行，则跳过
	if sess, ok := s.sessions[sessionID]; ok && atomic.LoadInt32(&sess.IsRunning) == 1 {
		return
	}

	// 2. 初始化新 Session
	ctx, cancel := context.WithCancel(context.Background())
	sess := &HawkingSession{
		ID:            sessionID,
		VoiceType:     voiceType,
		ActiveTasks:   make(map[string]*models.HawkingTask),
		taskNotify:    make(chan struct{}, 1),
		SessionCtx:    ctx,
		SessionCancel: cancel,
		IsRunning:     1,
	}
	s.sessions[sessionID] = sess

	// 3. 启动该 Session 的独立叫卖协程
	go s.runSessionLoop(sess)
}

func (s *HawkingScheduler) runSessionLoop(sess *HawkingSession) {
	defer func() {
		atomic.StoreInt32(&sess.IsRunning, 0)
		log.Printf("🛑 Session [%s] 已停止", sess.ID)
	}()

	for {
		// --- 1. 等待信号 ---
		// 我们不再主动轮询，只有在 AddTask 或是手动唤醒时才继续
		select {
		case <-sess.SessionCtx.Done():
			return
		case <-sess.taskNotify: // 只有收到 AddTask 信号才往下走
			log.Printf("🔔 Session [%s] 被唤醒，开始检查新任务", sess.ID)
		}

		// --- 2. 处理任务 ---
		sess.mu.RLock()
		// 提取还没合成的任务（按需处理）
		var pendingTasks []*models.HawkingTask
		for _, t := range sess.ActiveTasks {
			if !t.IsSynthesized { // 关键：只处理未合成的
				pendingTasks = append(pendingTasks, t)
			}
		}
		sess.mu.RUnlock()

		if len(pendingTasks) == 0 {
			continue
		}

		for _, task := range pendingTasks {
			// 增加一层校验：如果任务要求的音色和 Session 当前音色不符，说明是旧信号，跳过
			if task.VoiceType != sess.VoiceType {
				continue
			}
			product, err := s.productRepo.FindByID(task.ProductID)
			if err != nil {
				continue
			}

			// 执行合成
			audioURL, script, err := s.executeHawking(sess.SessionCtx, product, task)
			if err != nil {
				log.Printf("❌ 合成失败: %v", err)
				continue
			}

			// 更新状态
			sess.mu.Lock()
			task.IsSynthesized = true
			task.AudioURL = audioURL
			task.Text = script
			sess.mu.Unlock()

			// 匹配开场白
			//intro := s.pickIntroForSession(sess)
			// 🌟 获取该音色对应的完整开场白池
			introPool := s.GetIntroPoolByVoice(sess.VoiceType)

			log.Printf("📡 广播新资源: %s (带全量开场白池)", product.Name)
			// 📢 仅在此时广播：合成好了，告诉客户端“加菜了”
			s.broadcastPlayEventToSession(sess.ID, product, task, introPool)
		}
	}
}

func (s *HawkingScheduler) broadcastPlayEventToSession(sessionID string, p *models.Product, task *models.HawkingTask, introPool []*models.HawkingIntro) {
	data := PlayEventData{
		SessionID: sessionID, // 👈 关键：标识所属会话
		ProductID: p.ID.String(),
		IntroPool: introPool,
		Product:   task,
		VoiceType: task.VoiceType,
	}
	s.Hub.Broadcast(models.WSMessage{Type: "HAWKING_PLAY_EVENT", Data: data})
}

// 匹配 Session 对应的开场白
func (s *HawkingScheduler) pickIntroForSession(sess *HawkingSession) *models.HawkingIntro {
	hour := time.Now().Hour()
	// 从 Repo 找符合该 Session 音色和当前时间的模版
	templates := s.introRepo.FindAllByTime(hour, sess.VoiceType)
	if len(templates) == 0 {
		return nil
	}

	// 随机选一个实现“多样性”
	target := templates[rand.Intn(len(templates))]
	return &models.HawkingIntro{
		AudioURL:  target.AudioURL,
		Text:      target.Text,
		Scene:     target.SceneTag,
		VoiceType: target.VoiceType,
	}
}

func (s *HawkingScheduler) getOrRefreshIntro(sess *HawkingSession, task *models.HawkingTask) *models.HawkingIntro {
	now := time.Now().Hour()

	// 逻辑：检查该 Session 当前持有的 intro 是否失效
	// 注意：s.currentIntro 应该移到 HawkingSession 结构体中
	if sess.currentIntro == nil ||
		now < sess.currentIntro.StartHour ||
		now >= sess.currentIntro.EndHour ||
		sess.currentIntro.VoiceType != task.VoiceType {

		log.Printf("🔄 Session [%s] 正在刷新开场白 (当前小时: %d)", sess.ID, now)
		sess.currentIntro = s.getIntroTask(task)
	}

	return sess.currentIntro
}

// executeHawking 封装具体的执行步骤，保持 Start 方法简洁
func (s *HawkingScheduler) executeHawking(ctx context.Context, p *models.Product, task *models.HawkingTask) (audioURL string, script string, err error) {
	if task == nil {
		return
	}

	// 🌟 检查点 1：进入时检查
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	// 1. 生成文案
	script = task.Text
	// 生成文件名
	newFileName, currentHash := s.generateFileName(task, task.VoiceType)

	// 3. 缓存校验
	// 如果文案没变，且对应的音频文件确实存在于磁盘上
	// 3. 再次校验缓存（防止 runSynthesisBatch 过程中别的线程下好了）
	if s.checkAudioExists(newFileName) {
		audioURL = fmt.Sprintf("/static/audio/%s.mp3", newFileName)
		log.Printf("♻️ 文案未变，复用缓存音频: %s", p.Name)
		return audioURL, script, nil
	}

	// 🌟 检查点 2：调用外部 SDK 前检查
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	// 4. 文案变了或文件丢失，调用火山引擎合成
	log.Printf("🎙️ 文案已更新，正在调用火山引擎合成音频: %s", p.Name)
	audioURL, err = s.audioService.GenerateAudio(ctx, script, newFileName, task.VoiceType)
	if err != nil {
		log.Printf("❌ 语音合成失败 [%s]: %v", p.Name, err)
		// 这里如果是 context canceled，不应该将状态设为 idle
		if ctx.Err() == nil {
			s.productRepo.UpdateHawkingStatus(p.ID.String(), map[string]interface{}{"hawking_status": "idle"})
		}
		return
	}

	log.Printf("✅ 音频合成成功! 文件路径: %s", audioURL) // 👈 新增：确认合成完成

	// 5. 清理当前音色下的旧文案版本
	// 为了防止磁盘被同一个商品的各种历史版本占满，可以异步删掉该商品旧 Hash 的文件
	go s.cleanupOldVersions(p.ID.String(), task.VoiceType, newFileName)

	// 🌟 检查点 3：写入数据库前检查
	// 如果此时用户切换了音色，那么之前的合成结果虽然已经落盘，但不需要更新到这个 session 的 DB 任务状态中
	if err := ctx.Err(); err != nil {
		return "", "", err
	}

	// 更新哈希值准备存入数据库
	p.LastScriptHash = currentHash

	updates := map[string]interface{}{
		"last_script_hash": p.LastScriptHash,
		"last_hawked_at":   time.Now(),
		"priority":         0,
		"hawking_status":   "idle",
	}
	s.productRepo.UpdateHawkingStatus(p.ID.String(), updates)
	return
}
func (s *HawkingScheduler) generateFileName(task *models.HawkingTask, voiceID string) (fileName string, hash string) {
	// 统一使用 task.Text，它是 AddTask 时锁定的唯一真理
	script := task.Text
	hash = fmt.Sprintf("%x", md5.Sum([]byte(script)))[:8]
	return fmt.Sprintf("%s_%s_%s", task.ProductID, voiceID, hash), hash
}

// 辅助方法：检查本地文件是否还在（防止被手动删了）
func (s *HawkingScheduler) checkAudioExists(identifier string) bool {
	filePath := filepath.Join("./static/audio", identifier+".mp3")
	_, err := os.Stat(filePath)
	return err == nil
}

func (s *HawkingScheduler) AddTask(product *models.Product, req models.AddTaskReq) {
	s.sessionMu.Lock()
	sess, exists := s.sessions[req.SessionID]
	if !exists {
		// 1. 懒加载：创建并启动新 Session
		ctx, cancel := context.WithCancel(context.Background())
		sess = &HawkingSession{
			ID:            req.SessionID,
			VoiceType:     req.VoiceType,
			ActiveTasks:   make(map[string]*models.HawkingTask),
			taskNotify:    make(chan struct{}, 1),
			SessionCtx:    ctx,
			SessionCancel: cancel,
		}
		s.sessions[req.SessionID] = sess
		go s.runSessionLoop(sess) // 启动该 Session 的独立循环
		log.Printf("✨ 自动启动 Session [%s]", req.SessionID)
	}
	s.sessionMu.Unlock()

	finalText := req.Text

	// 2. 确定文案场景
	scene := "custom"
	if finalText == "" {
		// 构造一个临时 Task 传给文案生成逻辑
		tempTask := &models.HawkingTask{
			Price:         req.Price,
			OriginalPrice: req.OriginalPrice,
			Unit:          req.Unit,
			MinQty:        req.MinQty,
			ConditionUnit: req.ConditionUnit,
			VoiceType:     req.VoiceType,
			ProductID:     req.ProductID,
			PromotionTag:  req.PromotionTag,
			UseRepeatMode: req.UseRepeatMode,
		}
		finalText = logic.GenerateScript(*product, tempTask)
		scene = "smart_generated" // 标记是生成的
	}

	// 3. 在 Session 内部添加任务
	sess.mu.Lock()
	key := strings.ToLower(product.ID.String())
	sess.ActiveTasks[key] = &models.HawkingTask{
		ProductID:     req.ProductID,
		CustomText:    req.Text,
		Text:          finalText, // 锁定文案，后续音色切换全部基于此 Text
		Price:         req.Price,
		OriginalPrice: req.OriginalPrice,
		Unit:          req.Unit,
		MinQty:        req.MinQty,
		ConditionUnit: req.ConditionUnit,
		PromotionTag:  req.PromotionTag,
		UseRepeatMode: req.UseRepeatMode,
		VoiceType:     req.VoiceType,
		Scene:         scene,
		IsSynthesized: false, // 确保进入循环后被识别为 pendingTasks
	}
	sess.mu.Unlock()

	// 4. 唤醒信号
	// 触发信号唤醒 Start 中的 for 循环
	select {
	case sess.taskNotify <- struct{}{}:
		log.Println("✅ 唤醒信号发送成功")
	default:
		// 如果信号没发进去，说明上一次唤醒的任务还在处理中，
		// 处理完后它会自动重新检查 mu.ActiveTasks，所以不用担心丢失。
		log.Println("ℹ️ 调度器忙碌中，新任务已排队")
	}
}

func (s *HawkingScheduler) RemoveTask(sessionID string, productID string) {
	s.sessionMu.Lock()
	sess, exists := s.sessions[sessionID]
	if !exists {
		s.sessionMu.Unlock()
		return
	}

	sess.mu.Lock()
	delete(sess.ActiveTasks, strings.ToLower(productID))
	remaining := len(sess.ActiveTasks)
	sess.mu.Unlock()

	// ⚠️ 核心逻辑：如果任务空了，停止并移除 Session
	if remaining == 0 {
		sess.SessionCancel() // 停止 runSessionLoop 协程
		delete(s.sessions, sessionID)
		log.Printf("🗑️ Session [%s] 无任务，已自动停止并销毁", sessionID)
	}
	s.sessionMu.Unlock()
}

func (s *HawkingScheduler) GetActiveTasksSnapshot(sessionID string) *models.TasksSnapshotData {
	s.sessionMu.RLock()
	sess, exists := s.sessions[sessionID]
	s.sessionMu.RUnlock()

	if !exists {
		return &models.TasksSnapshotData{Products: []*models.HawkingTask{}, IntroPool: []*models.HawkingIntro{}}
	}

	sess.mu.RLock()
	defer sess.mu.RUnlock()

	var products = make([]*models.HawkingTask, 0)
	for _, task := range sess.ActiveTasks {
		products = append(products, task)
	}

	// 仅针对该 Session 所使用的音色下发开场白池
	introPool := s.GetIntroPoolByVoice(sess.VoiceType)

	return &models.TasksSnapshotData{
		Products:  products,
		IntroPool: introPool,
	}
}

// 场景 B：单次播放指令
func (s *HawkingScheduler) broadcastPlayEvent(p *models.Product, task *models.HawkingTask, introPool []*models.HawkingIntro) {

	data := PlayEventData{
		ProductID: p.ID.String(),
		IntroPool: introPool,
		Product:   task,
		VoiceType: task.VoiceType,
	}
	payload := models.WSMessage{
		Type: "HAWKING_PLAY_EVENT",
		Data: data,
	}
	s.Hub.Broadcast(payload)
}

func (s *HawkingScheduler) cleanupOldVersions(productID string, voiceType string, currentFullFileName string) {
	// 1. 更加精准的匹配模式：ProductID_VoiceType_*.mp3
	// 这样只会找到【当前商品】在【当前音色】下的历史版本
	pattern := filepath.Join("static/audio", fmt.Sprintf("%s_%s_*.mp3", productID, voiceType))

	files, _ := filepath.Glob(pattern)
	for _, f := range files {
		// 2. 只有文件名完全不匹配当前最新文件时才删除
		// 这样可以保留该商品在 其它音色 下的缓存文件
		if !strings.Contains(f, currentFullFileName) {
			log.Printf("🧹 清理旧版本缓存: %s", f)
			os.Remove(f)
		}
	}
}

// 辅助方法：匹配逻辑
func (s *HawkingScheduler) getIntroTask(task *models.HawkingTask) *models.HawkingIntro {
	// 逻辑核心：必须传入 task.VoiceType
	// 确保开场白与商品语音的人声完全统一

	// 1. 获取当前小时
	hour := time.Now().Hour()

	// 2. 从仓库查找：匹配 (时间段 + 任务指定的音色)
	template := s.introRepo.FindByTime(hour, task.VoiceType)
	if template == nil {
		// 3. 如果该音色没有对应时段的开场白，回退到默认
		template = s.introRepo.FindByID("default_01", task.VoiceType)
		fmt.Printf("模版中没有对应时段的开场白，使用默认开场白音频：%s", template.AudioURL)
	} else {
		fmt.Printf("从模版中匹配到了开场白音频：%s", template.AudioURL)
	}

	return &models.HawkingIntro{
		AudioURL:  template.AudioURL,
		Text:      template.Text,
		Scene:     template.SceneTag,
		IntroID:   template.ID,
		StartHour: template.TimeRange[0],
		EndHour:   template.TimeRange[1],
		VoiceType: template.VoiceType,
	}

}

func (s *HawkingScheduler) getOrCreateSession(sessionID string) *HawkingSession {
	s.sessionMu.RLock()
	defer s.sessionMu.RUnlock()
	sess, exists := s.sessions[sessionID]
	if !exists {
		sess = &HawkingSession{}
	}
	return sess
}
func (s *HawkingScheduler) ChangeSessionVoice(sessionID string, newVoiceID string, targetProductIDs []string) {
	sess := s.getOrCreateSession(sessionID)
	sess.mu.Lock()

	// 1. 取消旧批次
	if sess.BatchCancel != nil {
		sess.BatchCancel()
	}

	batchCtx, cancel := context.WithCancel(context.Background())
	sess.BatchCancel = cancel
	sess.VoiceVersion++
	currentVersion := sess.VoiceVersion
	sess.VoiceType = newVoiceID

	// 2. 准备判断 Map
	targetMap := make(map[string]bool)
	for _, id := range targetProductIDs {
		targetMap[strings.ToLower(id)] = true
	}

	hasPendingTask := false // 标记是否真的需要跑后台合成

	// 3. 必须遍历所有任务，确保内存里的元数据 100% 准确
	for _, task := range sess.ActiveTasks {
		task.VoiceType = newVoiceID // 统一音色标识

		// 基于已锁定的 task.Text 计算哈希，不再重新生成文案
		predictedName, _ := s.generateFileName(task, newVoiceID)
		// 第一步：先看服务端磁盘到底有没有
		existsOnServer := s.checkAudioExists(predictedName)

		if existsOnServer {
			// 只要服务端有，无论客户端传没传，都直接复用
			task.IsSynthesized = true
			task.AudioURL = fmt.Sprintf("/static/audio/%s.mp3", predictedName)
			log.Printf("♻️ 命中服务端缓存 [音色: %s]: %s", newVoiceID, predictedName)
		} else {
			// 如果服务端磁盘没有：
			// 无论客户端本地有没有，都必须重新合成，否则必然 404
			task.IsSynthesized = false
			task.AudioURL = ""
			hasPendingTask = true
			log.Printf("⚡️ 无缓存，准备合成新音色 [%s]: %s", newVoiceID, predictedName)
		}
	}
	sess.mu.Unlock()

	// 4. 只有存在真正需要合成的任务时，才启动协程
	if hasPendingTask {
		go s.runSynthesisBatch(sess, batchCtx, currentVersion)
	} else {
		log.Printf("✅ 所有任务均命中间缓存，无需发起 TTS 合成请求")
	}
}

// 重新合成音频
func (s *HawkingScheduler) runSynthesisBatch(sess *HawkingSession, ctx context.Context, version int) {
	// 批量抓取待处理任务
	sess.mu.RLock()
	var tasks []*models.HawkingTask
	for _, t := range sess.ActiveTasks {
		tasks = append(tasks, t) // 拿到当前所有任务
	}
	sess.mu.RUnlock()

	for _, task := range tasks {
		// 🌟 检查点 1: Context 是否被取消（音色是否又换了）
		select {
		case <-ctx.Done():
			return
		default:
		}

		product, err := s.productRepo.FindByID(task.ProductID)
		if err != nil {
			continue
		}

		// 执行合成，传入带取消功能的 ctx
		audioURL, script, err := s.executeHawking(ctx, product, task)
		if err != nil {
			continue
		}

		sess.mu.Lock()
		// 🌟 检查点 2: 双重校验版本号
		if sess.VoiceVersion != version {
			sess.mu.Unlock()
			return // 版本不一致，说明切换了，直接丢弃本次合成结果
		}

		task.IsSynthesized = true
		task.AudioURL = audioURL
		task.Text = script

		introPool := s.GetIntroPoolByVoice(sess.VoiceType)
		s.broadcastPlayEventToSession(sess.ID, product, task, introPool)
		sess.mu.Unlock()
	}
}

func (s *HawkingScheduler) GetIntroPoolByVoice(voiceType string) []*models.HawkingIntro {
	// 仅针对该 Session 所使用的音色下发开场白池
	templates := s.introRepo.FindAllByVoice(voiceType)
	var introPool = make([]*models.HawkingIntro, 0)
	for _, t := range templates {
		introPool = append(introPool, &models.HawkingIntro{
			AudioURL:  t.AudioURL,
			Text:      t.Text,
			Scene:     t.SceneTag,
			IntroID:   t.ID,
			StartHour: t.TimeRange[0],
			EndHour:   t.TimeRange[1],
			VoiceType: t.VoiceType,
		})
	}
	return introPool
}
