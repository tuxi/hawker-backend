package logic

import (
	"fmt"
	"hawker-backend/models"
)

// GenerateHawkingScript 根据手动设置的模式生成文案
func GenerateHawkingScript(p models.Product) string {
	price := p.Price
	if p.CustomPrice > 0 {
		price = p.CustomPrice
	}

	switch p.HawkingMode {
	case models.ModeAbundant:
		return fmt.Sprintf("好消息！新鲜的%s到货啦，货源充足，随您挑选！只要%v块一%s！", p.Name, price, p.Unit)
	case models.ModeLowStock:
		return fmt.Sprintf("注意啦！%s最后几%s，清仓甩卖，卖完就收摊！只要%v块，先到先得！", p.Name, p.Unit, price)
	case models.ModePromotion:
		return fmt.Sprintf("疯啦！全场大促销，%s原价%v，现在只要%v块！划算到家了！", p.Name, p.Price, price)
	case models.ModeNormal:
		return fmt.Sprintf("走过路过不要错过，优质%s，%v块一%s，欢迎选购！", p.Name, price, p.Unit)
	default:
		return ""
	}
}
