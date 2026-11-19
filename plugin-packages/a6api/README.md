# A6api

A6api 模型网关协议插件。统一入口 `https://api.a6api.com`，OpenAI 兼容（`Authorization: Bearer <api_key>`）。

## Providers

| Provider | 能力 | 端点 |
|---|---|---|
| a6api-chat | text | POST /chat/completions |
| a6api-image | image | POST /images/generations、POST /images/edits |

## 配置

在渠道中填写 API Key（A6api 控制台创建，形如 `sk-xxxxxxxx`），baseUrl 使用 `https://api.a6api.com`。

## 说明

- 模型名以 A6api 模型市场当前列表为准。
- 图像编辑（带参考图）走 `/images/edits` multipart 请求。
- 错误体为 OpenAI 风格，携带 `request_id`。

## 1.1.0：PNG 透明背景

- 保留插件 ID `a6api` 及 `a6api-chat` / `a6api-image` 协议 ID，更新包不创建另一套渠道。
- 图片协议声明完整动态参数，首页和画布可按同一合同显示尺寸、质量、数量与透明背景；现有模型的启用状态、范围和默认值由宿主导入后仍须管理员核对。
- 管理员在「渠道与模型 → 对应模型 → 能力与参数 → 协议参数」启用「透明背景」，默认值建议关闭；使用前台模型目录时同步核对其公开能力与路由。
- 用户开启时，文生图 JSON 和图片编辑 multipart 请求均发送 `background=transparent`；关闭时省略该字段。输出固定请求 `output_format=png`，Base64 结果保留原始 PNG 字节，不做解码重绘或背景合成。
- `imageResolution` 是宿主尺寸预设换算与计价档位，不额外发送名为 `resolution` 的上游字段。质量值由管理员按精确模型收窄，GPT Image 2 可配置 `auto / low / medium / high`；数量上限也须按模型和商家收窄。
- PNG 格式本身不保证透明：本插件负责正确传参并保留 Alpha，具体商家是否执行透明背景需通过真实生成检查。未提供抠图后处理或异步任务功能。

## 安装与验收

本目录是源码构建时随工作台分发的内置 A6API 协议；系统禁止上传覆盖内置插件，因此不能通过管理员插件上传把它替换为 1.1.0。需要从后台安装时，使用独立的 `a6api-transparent-png` 插件及 `a6api-transparent-image` 协议，安装与模型切换步骤见该插件 README。不要删除内置插件或渠道；运行中的旧任务不作为新版本验收依据。

依次检查：后台可启用透明背景，首页与画布出现开关，关闭时不发送 `background`，开启时发送 `transparent` 且格式为 PNG。最后检查新生成图片存在 Alpha 通道及透明像素；本地模拟验证不能替代具体商家线路的真实生成验收。

## 参考文档

- [A6API 生图使用说明](https://a6api.vip/#/docs?topic=api-reference&section=image-generation)：`a6api.ai` 当前入口指向该站点；文档声明可接收 `background`、`output_format`，实际转发取决于适配器和商家。
- [官方工作台 OpenAI Images 插件](https://github.com/ddcat-ai/open-ai-canvas/blob/main/plugin-packages/openai-images/manifest.json)：动态参数、透明背景绑定及 PNG Base64 响应格式参考。
