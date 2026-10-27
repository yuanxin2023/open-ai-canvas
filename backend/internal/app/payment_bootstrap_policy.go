package app

import (
	"errors"

	"infinite-canvas/backend/internal/protocol"
)

type paymentPluginOrderCounter interface {
	ActivePaymentOrderCountForPlugin(string, string) (int64, error)
}

// A changed payment artifact cannot replace the package needed by unresolved
// orders. An unavailable counter also defers replacement; it never grants it.
func deferInstalledPaymentUpdate(current protocol.Manifest, currentHash, nextHash string, counter paymentPluginOrderCounter) (bool, error) {
	if len(current.Contributes.PaymentProviders) == 0 || currentHash != "" && currentHash == nextHash {
		return false, nil
	}
	if counter == nil {
		return true, errors.New("未配置支付插件订单检查")
	}
	if current.Metadata.ID == "" {
		return true, errors.New("已安装支付插件缺少身份")
	}
	count, err := counter.ActivePaymentOrderCountForPlugin(current.Metadata.ID, current.Metadata.Version)
	if err != nil {
		return true, err
	}
	if count < 0 {
		return true, errors.New("支付插件未决订单计数无效")
	}
	return count > 0, nil
}
