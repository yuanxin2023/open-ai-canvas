# A6API SVG 接口说明

`POST {channel_base_url}/images/generations`，Bearer 鉴权，JSON 请求；宿主按既有规则处理 `/v1` 前缀。

| 参数 | 映射与约束 |
| --- | --- |
| 模型 | `model`，使用渠道实际模型 ID，不维护模型名称白名单 |
| 提示词 | `prompt`，描述主体、颜色、构图与矢量路径要求 |
| 画幅 / 尺寸 | `size`，沿用模型配置的实际可用像素尺寸 |
| 参考图质量 | `quality`，未设置时省略 |
| 数量 | `n=1`，其他数量拒绝 |
| 输出格式 | `output_format=svg`，无法由扩展选项覆盖 |

不发送 `response_format`、`stream`、`partial_images`、`output_compression` 或 `background`，不请求编辑接口。支持 SVG 的官方接口默认返回 Base64，A6API 商家也可能返回下载 URL。

响应读取 `data[].b64_json` 或 `data[].url`。声明期望 MIME 为 `image/svg+xml`，URL 的下载仍受宿主出站校验。SVG 要有正数的宽高或有效 `viewBox`，仅接受静态矢量内容；原文件逐字节保存，尺寸和 `.svg` 扩展名由宿主识别。

失败时保留上游错误。上游返回非 SVG、无效 XML、脚本、外部资源或内嵌位图时明确失败，不生成伪矢量资源。URL 响应应提供 `image/svg+xml`；类型丢失或错误时不靠文件名猜测。

<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "a6api-svg",
  "name": "A6API SVG 矢量图",
  "version": "1.0.0",
  "author": "A6API / 本地定制",
  "description": "通过 A6API 请求原生 SVG 文生图；需要商家线路支持 SVG 输出及宿主 SVG 保存支持。",
  "permissions": [
    "generation.run",
    "media.read"
  ],
  "configuration": {
    "fields": [
      {
        "name": "apiKey",
        "type": "secret",
        "label": "API Key",
        "required": true
      }
    ]
  },
  "contributes": {
    "providers": [
      {
        "id": "a6api-svg-image",
        "label": "A6API Image · SVG 矢量图",
        "capabilities": [
          "image"
        ],
        "scopes": [
          "admin.system-channel",
          "user.custom-channel",
          "canvas",
          "creation",
          "agent"
        ],
        "baseUrl": "https://api.a6api.vip",
        "auth": {
          "type": "bearer",
          "field": "apiKey"
        },
        "imageParameters": [
          {
            "name": "size",
            "label": "画幅 / 尺寸",
            "type": "string",
            "binding": "aspectRatio",
            "encoding": "pixels",
            "pricing": true
          },
          {
            "name": "quality",
            "label": "参考图质量",
            "description": "控制用于制作 SVG 的参考图质量。",
            "type": "string",
            "binding": "quality",
            "pricing": true
          },
          {
            "name": "count",
            "label": "生成数量",
            "type": "integer",
            "binding": "imageCount",
            "min": 1,
            "max": 1,
            "default": 1
          },
          {
            "name": "outputFormat",
            "label": "输出格式",
            "type": "string",
            "binding": "providerOptions.a6api-svg-image.output_format",
            "values": [
              "svg"
            ],
            "default": "svg"
          }
        ],
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "使用已确认支持 SVG 的 A6API 精确模型 ID。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": true,
            "mapping": "prompt"
          }
        ],
        "validations": [
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$ref": "request.images"
                  }
                },
                0
              ]
            },
            "message": "SVG 协议仅支持文生图，不支持参考图或蒙版编辑。"
          },
          {
            "assert": {
              "$or": [
                {
                  "$eq": [
                    {
                      "$ref": "request.imageCount"
                    },
                    0
                  ]
                },
                {
                  "$eq": [
                    {
                      "$ref": "request.imageCount"
                    },
                    1
                  ]
                }
              ]
            },
            "message": "SVG 协议每次只能生成一张图。"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/images/generations",
          "contentType": "application/json",
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "prompt": {
              "$ref": "request.prompt"
            },
            "n": 1,
            "size": {
              "$omitEmpty": {
                "$ref": "request.aspectRatio"
              }
            },
            "quality": {
              "$omitEmpty": {
                "$ref": "request.quality"
              }
            },
            "output_format": "svg"
          }
        },
        "response": {
          "status": "succeeded",
          "images": {
            "$map": {
              "from": {
                "$ref": "response.data"
              },
              "as": "item",
              "in": {
                "url": {
                  "$omitEmpty": {
                    "$ref": "item.url"
                  }
                },
                "b64_json": {
                  "$omitEmpty": {
                    "$ref": "item.b64_json"
                  }
                },
                "mimeType": "image/svg+xml"
              }
            }
          },
          "usage": {
            "$ref": "response.usage"
          },
          "errorPaths": [
            "error.code",
            "error.message"
          ],
          "messagePaths": [
            "error.message"
          ]
        }
      }
    ]
  },
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>"
}
```
<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_END -->
