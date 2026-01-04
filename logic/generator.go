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

	// 针对不同关键字的属性描述
	traits = map[string][]string{
		"猪肉": {"本地黑猪肉，", "早起刚宰的猪，", "肉质红润，", "一点注水都没有，"},
		"牛肉": {"正宗黄牛肉，", "纹路清晰可见，", "肉质紧实，", "口感扎实，"},
		"五花": {"肥瘦层层分明，", "这层色看这就漂亮，", "肥而不腻，"},
		"瘦肉": {"纯瘦里脊，", "一点肥膘不带，", "肉质鲜嫩，"},
		"排骨": {"排骨匀称，", "肉厚骨头小，", "全是精排小排，"},
		"禽类": {"现杀的老鸡老鸭，", "炖汤大补，", "肉质一点不柴，"},
	}

	// 针对不同关键字的烹饪建议
	advices = map[string][]string{
		"五花": {"红烧、小炒都喷香！", "做个红烧肉全家抢着吃！"},
		"瘦肉": {"包饺子、做肉丸最合适！", "给小孩炒肉丝特别嫩！"},
		"排骨": {"炖个汤、做个糖醋那是绝了！", "清炖红烧都好吃！"},
		"牛肉": {"炖个土豆，那叫一个香！", "切片炒辣椒，绝好的下酒菜！"},
		"副产": {"洗得干干净净，回家一炒就能吃！", "当下酒菜再合适不过了！"},
	}

	closings = []string{"快来带一点！", "先到先得啊！", "晚了就卖光了！", "欢迎选购！"}
)

func GenerateSmartScript(p models.Product, promoPrice float64, anchorPrice float64) string {
	rand.Seed(time.Now().UnixNano())

	// 1. 随机选开场
	script := openings[rand.Intn(len(openings))]

	// 2. 识别商品属性并添加描述 (智能匹配)
	hasTrait := false
	for key, list := range traits {
		if strings.Contains(p.Name, key) || strings.Contains(p.Category.Name, key) {
			script += list[rand.Intn(len(list))]
			hasTrait = true
			break // 匹配到一个核心属性就够了
		}
	}
	if !hasTrait {
		script += "优质生鲜，品质看得见，"
	}

	// 3. 嵌入商品名
	script += fmt.Sprintf("咱家的%s，", p.Name)

	// 4. 寻找烹饪建议 (智能关联)
	for key, list := range advices {
		if strings.Contains(p.Name, key) {
			script += list[rand.Intn(len(list))]
			break
		}
	}
	
	// 5. 【核心改进】组合价格逻辑
	if promoPrice > 0 {
		if anchorPrice > promoPrice {
			// 模式 A: 强调折扣力度（锚点价）
			script += fmt.Sprintf("平时都要 %.2f 的%s，今天摊位搞活动，", anchorPrice, p.Name)
			script += fmt.Sprintf("只要 %.2f 块一%s！", promoPrice, p.Unit)
			script += "真正的亏本处理，走过路过千万别错过！"
		} else {
			// 模式 B: 单纯强调现价
			script += fmt.Sprintf("咱家的%s，今天只要 %.2f 块一%s！", p.Name, promoPrice, p.Unit)
		}
	} else {
		// 模式 C: 兜底使用数据库价格
		script += fmt.Sprintf("咱家的%s，现在只要 %.2f 块一%s！", p.Name, p.Price, p.Unit)
	}

	// 6. 加上结尾和模式后缀
	if p.HawkingMode == models.ModeLowStock {
		script += "最后最后一点了，便宜处理！"
	} else {
		script += closings[rand.Intn(len(closings))]
	}

	return script
}
