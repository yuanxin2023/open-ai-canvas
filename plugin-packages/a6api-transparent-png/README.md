# A6API PNG 透明背景 1.1.1

这是可从管理员后台上传安装的独立图片协议插件，不覆盖系统内置 A6API。插件 ID 为 `a6api-transparent-png`，图片协议 ID 为 `a6api-transparent-image`，协议显示名称为「A6API Image · PNG 透明背景」。

## 为什么使用独立 ID

工作台禁止通过上传覆盖内置协议插件。此前同 ID 的 A6API 1.1.0 包虽然能通过清单与请求测试，但无法替换内置 `a6api`，安装界面只显示通用错误。1.1.1 同时隔离插件 ID、Provider ID 和扩展参数命名空间；原 A6API 图片、文本协议均保留。

## 安装与配置

1. 在插件管理上传 `a6api-transparent-png-1.1.1.yingce-plugin`，不要上传外层 ZIP，也不要卸载内置 A6API。
2. 在管理员「渠道与模型」中复制目标图片模型用于验证，或编辑指定模型，将请求协议改为「A6API Image · PNG 透明背景」。API Key、Base URL 和上游模型 ID 沿用渠道实际配置。
3. 在「能力与参数」核对尺寸、质量、张数及价格，启用「透明背景」，默认值保持关闭。使用前台模型目录时，核对它关联的是新协议模型及其公开能力。
4. 保存后刷新用户端，选择新协议对应的模型，开启透明背景并创建新任务。

只安装插件不会自动把原模型从 `a6api-image` 切换到新协议。切换协议时宿主可能重置能力参数，应逐项核对，不能直接跳过保存前检查。

## 功能与边界

- 文生图走 JSON `/images/generations`；参考图与蒙版编辑走 multipart `/images/edits`。
- 动态参数包含尺寸、分辨率、质量、数量、透明背景和输出格式；首页与画布使用同一套声明。
- 开启透明背景时发送 `background=transparent`，关闭或未传时省略；输出始终请求 PNG，原样保留 Base64 图片及 Alpha 通道。
- 插件不执行代码、不提供抠图后处理、不新增异步任务，不修改系统内置协议。
- 生成数量合同上限 100 是网关范围，管理员须按模型收窄；GPT Image 2 可设 10 或更低。分辨率档位由宿主换算为像素尺寸，不发送独立 `resolution`。
- PNG 不代表一定有透明像素，具体商家线路是否执行 `background` 需真实验收。

## 资料来源

- [官方工作台协议插件格式](https://github.com/ddcat-ai/open-ai-canvas/blob/main/plugin-packages/openai-images/manifest.json)
- [A6API 生图文档](https://a6api.vip/#/docs?topic=api-reference&section=image-generation)：`a6api.ai` 入口指向该站点；DTO 可接收 `background`，是否转发由具体适配器和商家决定。
