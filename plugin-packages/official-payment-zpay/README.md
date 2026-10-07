# ZPAY 聚合支付

官方 `open-ai-canvas.payment/v1` RPC 支付插件，通过兼容易支付的 ZPAY API 提供支付宝和微信扫码充值。

插件只负责协议转换、签名和验签。商户密钥由AI 创作工作台支付渠道配置加密保存，不写入 Manifest、日志或订单正文。
