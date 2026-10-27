package protocol

import (
	"fmt"
	"maps"
	"strings"
)

// NormalizeLegacyImageOptions projects only shipped, equivalent field mappings.
// Callers retain the saved configuration and reject unsupported projected values.
func NormalizeLegacyImageOptions(providerID string, options map[string]any) (map[string]any, error) {
	result := maps.Clone(options)
	if result == nil {
		result = map[string]any{}
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "openai-image" {
		if quality, ok := result["quality"].(string); ok {
			if mapped := map[string]string{"1k": "low", "2k": "medium", "4k": "high"}[strings.ToLower(strings.TrimSpace(quality))]; mapped != "" {
				result["quality"] = mapped
			}
		}
		return result, nil
	}
	var tiers map[string]bool
	aliases := map[string]string{}
	switch providerID {
	case "grok-image":
		tiers = map[string]bool{"1k": true, "2k": true}
		aliases = map[string]string{"low": "1k", "standard": "1k", "medium": "2k", "hd": "2k", "high": "2k", "4k": "2k"}
	case "gemini-image", "subrouter-gemini-image":
		tiers = map[string]bool{"1k": true, "2k": true, "4k": true}
		aliases = map[string]string{"low": "1k", "medium": "2k", "high": "4k"}
	case "agnes-image":
		tiers = map[string]bool{"1k": true, "2k": true, "4k": true}
	case "km-kemei-seedream":
		tiers = map[string]bool{"1k": true, "1.5k": true, "2k": true}
	default:
		return result, nil
	}
	move := func(name string, ignoreAuto bool) error {
		raw, exists := result[name]
		if !exists || raw == nil || raw == "" {
			return nil
		}
		value, ok := raw.(string)
		if !ok {
			return fmt.Errorf("旧图片参数 %s 类型无效，请核对渠道配置", name)
		}
		value = strings.ToLower(strings.TrimSpace(value))
		if mapped := aliases[value]; mapped != "" {
			value = mapped
		}
		if ignoreAuto && (value == "auto" || value == "any") {
			if current, ok := result["imageResolution"]; ok && current != nil && current != "" {
				delete(result, name)
				return nil
			}
			value = "auto"
		}
		if !tiers[value] && value != "*" && value != "auto" {
			return fmt.Errorf("旧图片参数 %s=%s 无法等价转换，请核对渠道配置", name, value)
		}
		if current, exists := result["imageResolution"]; exists && current != nil && current != "" {
			if text, ok := current.(string); !ok || strings.ToLower(strings.TrimSpace(text)) != value {
				return fmt.Errorf("旧质量与图片分辨率条件冲突，请核对渠道配置")
			}
		}
		result["imageResolution"] = value
		delete(result, name)
		return nil
	}
	if err := move("quality", true); err != nil {
		return nil, err
	}
	if err := move("resolution", false); err != nil {
		return nil, err
	}
	return result, nil
}
