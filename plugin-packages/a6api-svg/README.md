# A6API SVG 矢量图 1.0.0

独立可上传的 SVG 文生图协议，插件 ID 为 `a6api-svg`，Provider ID 为 `a6api-svg-image`。不覆盖内置 A6API，也不替换现有透明 PNG 协议。

## 安装和使用

1. 先更新宿主后端至包含本次 SVG 支持的源码构建。仅安装插件到旧后端不能补齐 SVG 内容校验和作品保存。
2. 管理后台上传 `a6api-svg-1.0.0.yingce-plugin`。
3. 在渠道与模型中复制一个已确认支持 SVG 的模型，选择「A6API Image · SVG 矢量图」协议。保留实际渠道地址、密钥和精确上游模型 ID。
4. 保存前逐项核对尺寸、质量、单张数量、价格和公开模型关联；不要把旧协议参数直接复制为新协议的动态参数。
5. 刷新前台后选择新模型，用文字描述生成图标、Logo 或插画，下载原文件得到 `.svg`。

此协议固定请求 SVG，参数只提供 SVG 格式和单张数量。参考图和蒙版编辑会明确拒绝。普通 PNG、透明 PNG 和已能生成的 GIF 继续使用原模型协议。

## 支持边界

- A6API 生图文档只说明 `output_format` 可接收，是否转发由适配器和商家决定，没有承诺所有线路支持 SVG。安装成功不等于上游支持；尚未调用真实收费模型验证。
- URL 和 Base64 两种响应均接受。宿主验证完整 SVG XML、静态元素和本地引用，保留原始字节；禁止脚本、事件处理器、外部资源、内嵌位图和动画。不是 PNG 改后缀，也不提供位图转矢量后处理。
- 上游忽略 SVG 参数并返回 PNG/GIF 时明确失败，不把位图标成 SVG。不自动换模型或重新提交生成。
- SVG 原文件沿用附件与 sandbox 分发，不放宽普通上传文件的安全边界。预览由浏览器图片组件处理，不提供路径编辑器；原文件可交给矢量编辑软件继续编辑。
- SVG 费用可能包括参考图生成和 SVG 编写；本次不改变系统报价、结算或历史订单。真实商家的用量和计价需管理员另行核对。

资料：[A6API 生图说明](https://a6api.vip/#/docs?topic=api-reference&section=image-generation)、[OpenAI SVG 输出](https://developers.openai.com/api/docs/guides/image-generation#generate-svg-output)。
