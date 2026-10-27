package app

import (
	"fmt"
	"strings"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/protocol"
)

func projectLegacyImageCapability(provider string, image *ImageCapabilityConfig) error {
	if image == nil || image.Parameters != nil || !image.Quality.Supported {
		return nil
	}
	quality := image.Quality
	if len(quality.Values) == 0 && strings.TrimSpace(provider) == "openai-image" {
		projected, err := protocol.NormalizeLegacyImageOptions(provider, map[string]any{"quality": quality.Default})
		if err != nil {
			return BadAuthRequest(err.Error())
		}
		image.Quality.Default = fmt.Sprint(projected["quality"])
		return nil
	}
	var field string
	values := make([]string, 0, len(quality.Values))
	for _, value := range quality.Values {
		projected, err := protocol.NormalizeLegacyImageOptions(provider, map[string]any{"quality": value})
		if err != nil {
			return BadAuthRequest(err.Error())
		}
		name := "quality"
		if _, ok := projected["imageResolution"]; ok {
			name = "imageResolution"
		}
		mapped, ok := projected[name].(string)
		if !ok || mapped == "" || field != "" && field != name {
			return BadAuthRequest("旧图片质量声明无法等价转换，请核对渠道配置")
		}
		field = name
		if !containsCapabilityString(values, mapped) {
			values = append(values, mapped)
		}
	}
	projected, err := protocol.NormalizeLegacyImageOptions(provider, map[string]any{"quality": quality.Default})
	if err != nil {
		return BadAuthRequest(err.Error())
	}
	initial, ok := projected[field].(string)
	if !ok || !containsCapabilityString(values, initial) {
		return BadAuthRequest("旧图片质量默认值无法等价转换，请核对渠道配置")
	}
	if field == "imageResolution" {
		if image.Resolution.Supported {
			if image.Resolution.Default != initial || len(image.Resolution.Values) != len(values) {
				return BadAuthRequest("旧质量与图片分辨率配置冲突，请核对渠道配置")
			}
			for _, value := range values {
				if !containsCapabilityString(image.Resolution.Values, value) {
					return BadAuthRequest("旧质量与图片分辨率配置冲突，请核对渠道配置")
				}
			}
		}
		image.Resolution = ImageResolutionConfig{Supported: true, Values: values, Default: initial}
		image.Quality = ImageQualityConfig{Supported: false, Values: []string{}, Default: "auto"}
	} else {
		image.Quality = ImageQualityConfig{Supported: true, Values: values, Default: initial}
	}
	return nil
}

func normalizedImagePriceSelector(channel model.ChannelModel, parameters []protocol.ImageParameter, selector map[string]string) (map[string]string, error) {
	values := make(map[string]any, len(selector))
	for key, value := range selector {
		values[key] = value
	}
	projected, err := protocol.NormalizeLegacyImageOptions(string(channel.Protocol), values)
	if err != nil {
		return nil, BadAuthRequest(err.Error())
	}
	// Renaming preserves a condition; dropping it would broaden the price rule.
	for key, value := range selector {
		if _, ok := projected[key]; !ok && projected["imageResolution"] == nil && key == "quality" && (strings.EqualFold(value, "auto") || strings.EqualFold(value, "any")) {
			return nil, BadAuthRequest("旧价格质量条件无法等价转换，请核对价格档")
		}
	}
	result := make(map[string]string, len(projected))
	for key, value := range projected {
		result[key] = fmt.Sprint(value)
	}
	if err := validateImageParameterPriceSelector(parameters, result); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizedImageIntent(channel model.ChannelModel, intent ModelRequestIntent) (ModelRequestIntent, error) {
	if normalizeCapability(intent.Capability) != "image" {
		return intent, nil
	}
	options, err := protocol.NormalizeLegacyImageOptions(string(channel.Protocol), intent.Options)
	if err != nil {
		return intent, BadAuthRequest(err.Error())
	}
	intent.Options = options
	return intent, nil
}

func validateImageChannelPrices(channel model.ChannelModel, parameters []protocol.ImageParameter) error {
	for _, tier := range channel.PriceTiers {
		if !tier.Enabled || !tier.PriceConfigured {
			continue
		}
		if _, err := normalizedImagePriceSelector(channel, parameters, model.DecodeSKUSelector(tier.SelectorJSON)); err != nil {
			return fmt.Errorf("图片价格档 %s 需核对：%w", tier.ID, err)
		}
	}
	return nil
}
