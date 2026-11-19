# A6API PNG 透明背景接口

## 身份与鉴权

- 插件 ID：`a6api-transparent-png`；Provider ID：`a6api-transparent-image`。
- 默认 Base URL：`https://api.a6api.com`，实际调用沿用管理员渠道配置。
- 鉴权：`Authorization: Bearer <API_KEY>`。
- 无参考图：JSON `POST /images/generations`；有参考图或蒙版：multipart `POST /images/edits`。

## 参数

| 工作台参数 | 上游字段/用途 |
| --- | --- |
| `size` | `size`，像素尺寸编码 |
| `imageResolution` | 宿主尺寸预设换算、计价，不单独发送 |
| `quality` | `quality`，范围按精确模型配置 |
| `count` | `n`，默认 1，合同范围 1–100，模型应进一步收窄 |
| `transparentBackground` | 开启发送 `background=transparent`；关闭省略 |
| `outputFormat` | 固定 `output_format=png` |

`model`、`prompt` 必填；蒙版与参考图沿用原 A6API 编辑格式。`response_format` 沿用 A6API 网关的 `b64_json` 默认约定，不代表 OpenAI 官方直连接口支持此字段。

响应读取 `data[].url` 或 `data[].b64_json`；后者原样添加 `data:image/png;base64,` 前缀，不压平或重新编码图片。错误读取 `error.code` 与 `error.message`。

扩展参数命名空间为 `providerOptions.a6api-transparent-image`，与系统内置 `a6api-image` 完全隔离。上游真实透明效果仍需检查返回图片的 Alpha 通道及透明像素。

<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "a6api-transparent-png",
  "name": "A6API PNG 透明背景",
  "version": "1.1.1",
  "author": "A6API / 本地定制",
  "description": "独立可上传的 A6API 图片协议：动态参数、PNG 输出和透明背景。",
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
        "id": "a6api-transparent-image",
        "label": "A6API Image · PNG 透明背景",
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
        "baseUrl": "https://api.a6api.com",
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
            "name": "imageResolution",
            "label": "分辨率",
            "type": "string",
            "binding": "resolution",
            "enabled": false,
            "pricing": true
          },
          {
            "name": "quality",
            "label": "图片质量",
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
            "max": 100,
            "step": 1,
            "default": 1
          },
          {
            "name": "transparentBackground",
            "label": "透明背景",
            "description": "请求保留 Alpha 通道的 PNG；仅为已确认支持的模型和商家线路开启。",
            "type": "boolean",
            "binding": "transparentBackground",
            "default": false,
            "enabled": false
          },
          {
            "name": "outputFormat",
            "label": "输出格式",
            "description": "固定为 PNG，避免透明通道丢失及 Base64 MIME 类型不一致。",
            "type": "string",
            "binding": "providerOptions.a6api-transparent-image.output_format",
            "values": [
              "png"
            ],
            "default": "png"
          }
        ],
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "A6api 模型市场中的图像模型 ID。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": true,
            "mapping": "prompt",
            "description": "图片提示词。"
          },
          {
            "name": "images",
            "type": "media[]",
            "required": false,
            "mapping": "provider image/reference fields",
            "description": "参考图或编辑源图，role 由业务层确定。"
          },
          {
            "name": "imageCount",
            "type": "integer",
            "required": false,
            "mapping": "n/sample_count",
            "description": "输出数量。"
          },
          {
            "name": "aspectRatio",
            "type": "string",
            "required": false,
            "mapping": "size/aspect_ratio",
            "description": "比例或尺寸，语义按协议说明。"
          },
          {
            "name": "resolution",
            "type": "string",
            "required": false,
            "mapping": "resolution/imageSize",
            "description": "分辨率档位。"
          },
          {
            "name": "quality",
            "type": "string",
            "required": false,
            "mapping": "quality",
            "description": "质量档位。"
          },
          {
            "name": "providerOptions",
            "type": "object",
            "required": false,
            "mapping": "provider-specific fields",
            "description": "插件命名空间内的厂商扩展字段。"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/images/generations",
          "pathTemplate": {
            "$if": {
              "condition": {
                "$gt": [
                  {
                    "$len": {
                      "$ref": "request.images"
                    }
                  },
                  0
                ]
              },
              "then": "/images/edits",
              "else": "/images/generations"
            }
          },
          "contentType": "application/json",
          "contentTypeTemplate": {
            "$if": {
              "condition": {
                "$gt": [
                  {
                    "$len": {
                      "$ref": "request.images"
                    }
                  },
                  0
                ]
              },
              "then": "multipart/form-data",
              "else": "application/json"
            }
          },
          "body": {
            "model": {
              "$ref": "request.model"
            },
            "prompt": {
              "$ref": "request.prompt"
            },
            "n": {
              "$omitEmpty": {
                "$if": {
                  "condition": {
                    "$gt": [
                      {
                        "$ref": "request.imageCount"
                      },
                      0
                    ]
                  },
                  "then": {
                    "$ref": "request.imageCount"
                  },
                  "else": 1
                }
              }
            },
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
            "background": {
              "$omitEmpty": {
                "$if": {
                  "condition": {
                    "$eq": [
                      {
                        "$ref": "request.extra.transparentBackground"
                      },
                      "true"
                    ]
                  },
                  "then": "transparent",
                  "else": null
                }
              }
            },
            "output_format": "png",
            "response_format": {
              "$omitEmpty": {
                "$coalesce": [
                  {
                    "$ref": "request.providerOptions.a6api-transparent-image.response_format"
                  },
                  "b64_json"
                ]
              }
            },
            "user": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.a6api-transparent-image.user"
              }
            }
          },
          "files": [
            {
              "name": "image",
              "source": {
                "$filter": {
                  "from": {
                    "$ref": "request.images"
                  },
                  "as": "media",
                  "where": {
                    "$ne": [
                      {
                        "$ref": "media.role"
                      },
                      "mask"
                    ]
                  }
                }
              },
              "filename": "source.png"
            },
            {
              "name": "mask",
              "source": {
                "$filter": {
                  "from": {
                    "$ref": "request.images"
                  },
                  "as": "media",
                  "where": {
                    "$eq": [
                      {
                        "$ref": "media.role"
                      },
                      "mask"
                    ]
                  }
                }
              },
              "filename": "mask.png"
            }
          ]
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
                "dataUrl": {
                  "$if": {
                    "condition": {
                      "$ref": "item.b64_json"
                    },
                    "then": {
                      "$concat": [
                        "data:image/png;base64,",
                        {
                          "$ref": "item.b64_json"
                        }
                      ]
                    },
                    "else": null
                  }
                }
              }
            }
          },
          "usage": {
            "$ref": "response.usage"
          },
          "errorPaths": [
            "error.code"
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
