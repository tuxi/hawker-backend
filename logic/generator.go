package logic

import (
	"fmt"
	"hawker-backend/models"
	"math/rand"
	"strings"
	"time"
)

// 定义组件池
var (
	openings = []string{"快来看啊，", "各位街坊邻居，", "新鲜到货了！", "瞧一瞧看一看，", "买好肉找咱家，"}
	closings = []string{"快来带一点！", "先到先得啊！", "晚了就卖光了！", "欢迎选购！"}

	// 针对不同关键字的属性描述
	// 更加口语化、带卖点的描述
	traits = map[string][]string{
		"猪肉": {"本地黑猪肉，当天现宰，", "肥膘少、瘦肉多，", "肉色红润，没打过水的，", "这一块肉看着就舒坦，"},
		"牛肉": {"鲜嫩黄牛肉，不打水不压秤，", "纹路漂亮，拿回家怎么炒都不老，", "现切的腱子肉，炖着吃最香，", "正宗黄牛肉，", "纹路清晰可见，", "肉质紧实，", "口感扎实，"},
		"五花": {"三层五花，肥瘦均匀，", "这层色，做红烧肉简直绝了，", "肥的不腻，瘦的不柴，"},
		"排骨": {"全是精选小排，不带大脊骨，", "骨头小、肉厚实，", "回家糖醋或者炖汤都行，", "排骨匀称，", "肉厚骨头小，", "全是精排小排，"},
		"瘦肉": {"纯瘦里脊，", "一点肥膘不带，", "肉质鲜嫩，"},
		"禽类": {"农家土鸡土鸭，炖汤一层油，", "肉质紧实，不是那种饲料鸡，", "现杀现卖，新鲜看得见，"},
		"副产": {"洗得干干净净，回家直接下锅，", "新鲜的猪肝猪心，补铁补血最好了，", "没味儿，拿回家随便炒炒都好吃，"},
		"羊肉": {"正宗山羊肉，一点不膻，", "冬天炖个萝卜，热乎乎的太补了，"},
	}

	// 更有生活气息的建议
	advices = map[string][]string{
		"五花": {"做个扣肉或者红烧肉，全家都爱吃！", "切片煸个油，炒青菜香死个人！", "红烧、小炒都喷香！", "做个红烧肉全家抢着吃！"},
		"瘦肉": {"切个肉丝炒辣椒，那是绝配！", "剁碎了包饺子，汁水特别多！", "包饺子、做肉丸最合适！", "给小朋友炒肉丝特别嫩！"},
		"排骨": {"炖个冬瓜汤，清甜又好喝！", "炸个排骨，小孩能抢着吃完！", "炖个汤、做个糖醋那是绝了！", "清炖红烧都好吃！"},
		"牛肉": {"逆着纹路切，炒出来比豆腐还嫩！", "加点土豆块，焖一锅全家香！"},
		"大肠": {"配点尖椒一爆炒，下酒神器啊！", "卤着吃更香，越嚼越有味儿！"},
	}
)

// GenerateScript 叫卖文案生成核心入口
func GenerateScript(p models.Product, task *models.HawkingTask) string {
	// 每次生成重新播种，确保真随机
	rand.Seed(time.Now().UnixNano())

	// 1. 口语化价格转换
	oralPrice := formatPriceToOral(task.Price, task.Unit)
	oralOriginalPrice := ""
	// 🌟 只有当原价确实存在且大于现价时，才生成原价口语
	if task.OriginalPrice > task.Price {
		oralOriginalPrice = formatPriceToOral(task.OriginalPrice, task.Unit)
	}

	// 2. 确定时间语境
	timeContext := "今天"
	if time.Now().Hour() >= 17 {
		timeContext = "晚上"
	}

	// 3. 策略选择：如果开启复读机模式
	if task.UseRepeatMode {
		label := p.MarketingLabel
		if label == "" {
			label = "新鲜的"
		}

		promo := task.PromotionTag
		if promo == "" {
			promo = "活动价"
		}

		// --- 🌟 优化后的复读机模板 ---
		// 情况 A: 有原价时，加入对比逻辑
		if oralOriginalPrice != "" {
			return fmt.Sprintf("%s %s，%s%s，平时都要卖 %s，%s%s 只要 %s！",
				p.Name, oralPrice, // 第一遍报盘
				label, p.Name, // 第二遍开始：定语+品名
				oralOriginalPrice,             // 抛出原价做对比
				timeContext, promo, oralPrice, // 给出现在的促销理由和价格
			)
		}

		// 情况 B: 无原价时，保持原来的简洁有力
		return fmt.Sprintf("%s %s，%s%s，%s%s 只要 %s！",
			p.Name, oralPrice,
			label, p.Name,
			timeContext, promo, oralPrice,
		)
	}

	// 4. 智能描述模式逻辑
	return generateSmartScriptExtended(p, task, oralPrice, oralOriginalPrice)
}

// formatPriceToOral 将数字价格和单位转化为富有烟火气的口语
func formatPriceToOral(price float64, unit string) string {
	if price <= 0 {
		return "价格面议"
	}

	// 1. 拆解元、角、分
	// 加上 0.5 解决 float64 精度丢失问题（如 19.9 变成 19.89999）
	totalFen := int(price*100 + 0.5)
	yuan := totalFen / 100
	jiao := (totalFen % 100) / 10
	fen := totalFen % 10

	var priceStr string
	priceStr = fmt.Sprintf("%d块", yuan)

	if jiao > 0 && fen > 0 {
		// 场景：11.99 -> 11块9毛9
		priceStr += fmt.Sprintf("%d毛%d", jiao, fen)
	} else if jiao > 0 && fen == 0 {
		// 场景：11.9 -> 11块9 (口语习惯省略“毛”)
		priceStr += fmt.Sprintf("%d", jiao)
	} else if jiao == 0 && fen > 0 {
		// 场景：11.05 -> 11块零5分
		priceStr += fmt.Sprintf("零%d分", fen)
	}

	// 2. 单位处理逻辑保持不变
	if unit == "" {
		return priceStr
	}

	// 检查单位是否包含数字 (如 "3个")
	hasNumber := false
	for _, r := range unit {
		if r >= '0' && r <= '9' {
			hasNumber = true
			break
		}
	}

	if hasNumber {
		// 10块钱3个
		return fmt.Sprintf("%s钱%s", priceStr, unit)
	}

	// 11块9毛9一斤
	return fmt.Sprintf("%s一%s", priceStr, unit)
}

// 扩展后的智能脚本生成逻辑
func generateSmartScriptExtended(p models.Product, req *models.HawkingTask, oralPrice string, oralOriginalPrice string) string {
	// 随机选开场
	script := openings[rand.Intn(len(openings))]

	// 识别商品属性卖点
	matched := false
	for key, list := range traits {
		if strings.Contains(p.Name, key) || (p.Category.Name != "" && strings.Contains(p.Category.Name, key)) {
			script += list[rand.Intn(len(list))]
			matched = true
			break
		}
	}
	if !matched {
		script += "优质生鲜，品质看得见，"
	}

	script += fmt.Sprintf("咱家的%s，", p.Name)

	// 烹饪建议
	for key, list := range advices {
		if strings.Contains(p.Name, key) {
			script += list[rand.Intn(len(list))]
			break
		}
	}

	// 🌟 价格组合逻辑优化
	promo := req.PromotionTag
	if promo == "" {
		promo = "今天搞活动"
	}

	if oralOriginalPrice != "" {
		// 场景：平时都要卖 15块，现在秒杀价只要 11块9毛9！
		script += fmt.Sprintf("平时都要卖 %s，现在%s，只要 %s！", oralOriginalPrice, promo, oralPrice)
	} else {
		script += fmt.Sprintf("现在%s，只要 %s！", promo, oralPrice)
	}

	// 结尾增加稀缺感
	if p.HawkingMode == models.ModeLowStock {
		script += "最后最后一点了，便宜处理！"
	} else {
		script += closings[rand.Intn(len(closings))]
	}

	return script
}
