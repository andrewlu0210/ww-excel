package document

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"ww-excel/internal/model"
)

func checkIconSet(rule *model.CFRule) error {
	if rule.IconSet == nil {
		return fmt.Errorf("請選擇圖示集")
	}
	set := *rule.IconSet
	count := model.IconSetSize(set.Set)
	if count == 0 {
		return fmt.Errorf("不支援的圖示集「%s」", set.Set)
	}
	set.Thresholds = append([]model.CFThreshold(nil), set.Thresholds...)
	set.Icons = append([]model.CFIcon(nil), set.Icons...)
	if len(set.Thresholds) == 0 {
		set.Thresholds = model.IconThresholds(count)
	}
	if len(set.Thresholds) != count || len(set.Icons) > 0 && len(set.Icons) != count {
		return fmt.Errorf("圖示門檻數量不符")
	}
	for i := range set.Thresholds {
		threshold := &set.Thresholds[i]
		switch threshold.Type {
		case "num", "percent", "percentile":
			value, err := strconv.ParseFloat(strings.TrimSpace(threshold.Value), 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || threshold.Type != "num" && (value < 0 || value > 100) {
				return fmt.Errorf("請輸入有效的圖示門檻")
			}
			if i > 0 && set.Thresholds[i-1].Type == threshold.Type {
				previous, _ := strconv.ParseFloat(set.Thresholds[i-1].Value, 64)
				if value < previous {
					return fmt.Errorf("圖示門檻須由小到大排列")
				}
			}
		case "formula":
			threshold.Value = strings.TrimPrefix(strings.TrimSpace(threshold.Value), "=")
			if _, err := ruleFormula(threshold.Value, nil, 0, 0); err != nil {
				return fmt.Errorf("公式有誤：%v", err)
			}
		case "min", "max", "autoMin", "autoMax":
		default:
			return fmt.Errorf("請輸入有效的圖示門檻")
		}
	}
	for _, icon := range set.Icons {
		if icon.Set == "NoIcons" {
			continue
		}
		if icon.Index < 0 || icon.Index >= model.IconSetSize(icon.Set) {
			return fmt.Errorf("圖示索引無效")
		}
	}
	rule.IconSet = &set
	return nil
}
