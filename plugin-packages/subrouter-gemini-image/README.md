# SubRouter Gemini Image

- 插件 ID：`subrouter-gemini-image`
- Provider ID：`subrouter-gemini-image`
- 网站/供应商：SubRouter
- 模型范围：同类 Gemini Image generateContent 模型（使用渠道配置的模型 ID）
- 能力：`image`
- 默认 Base URL：`https://asiasouth.up.railway.app`
- 鉴权方式：`google-api-key`
- 生命周期：同步响应
- 安装包：`subrouter-gemini-image.canvas-plugin`
- 官方 API 文档：https://subrouter.ai/docs?tab=channels&source=self&model=gemini-3.1-flash-image&provider=4k-image
- 文档核对日期：2026-09-29

## 已知限制

支持同步文生图与图生图，不限制具体模型 ID；模型必须兼容 Gemini Image generateContent 请求结构。参考图从 request.images 写入 contents[0].parts：Data URL 转为 inlineData，远程 URL 转为 fileData。响应兼容标准 inlineData 和 parts[].text 中的 Markdown 图片链接。尚未确认批量出图和异步任务，因此插件不声明这些能力。签名链接标称 5 小时有效，宿主必须立即下载持久化。

完整字段和响应映射见 [docs/interface.md](docs/interface.md)。
