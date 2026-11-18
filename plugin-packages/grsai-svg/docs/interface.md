# GrsAI SVG 接口说明

## 身份与请求

- 清单：`yingce.plugin/v2`，插件 `grsai-svg` 1.0.0，协议 `grsai-svg-image`。
- 权限：`generation.run`、`media.read`；支持后台系统渠道、用户自定义渠道、画布、创作和 Agent。
- 鉴权：`Authorization: Bearer <API Key>`，密钥使用宿主 secret 配置。
- 创建：`POST /v1/api/generate`，JSON；查询：`GET /v1/api/result?id=<任务ID>`。两条路径均从渠道域名根路径解析，不重复拼接 /v1。

| 参数 | 上游映射及限制 |
| --- | --- |
| 模型 | 固定 `model=gpt-image-2.6`，请求模型不匹配时拒绝 |
| 提示词 | `prompt` |
| 参考图 | `images[]`，按 order 排序，使用 URL 或完整 Base64 data URL；无参考图时为空数组 |
| 画幅 / 尺寸 | `aspectRatio`，默认 `1024x1024`，允许文档中的比例或 1K 像素值；不提供 2K/4K 和自定义像素 |
| 质量 | 固定 `quality=auto` |
| 数量 | 单张；不发送文档未声明的 n/count |
| 输出格式 | 固定 `outputFormat=svg`，不受扩展参数覆盖 |
| 回复类型 | 固定 `replyType=async` |

拒绝其他模型、其他格式、多张输出、其他质量、未声明尺寸、缺少地址的参考图和蒙版。上游文档没有提供 SVG 专用蒙版接口或其他模型的 SVG 支持，因此不作推断。

## 响应与恢复

创建和查询共用 `id`、`status`、`results[].url`、`error` 映射。宿主规范化 pending/queued、running/processing、succeeded、failed 等状态；violation 显式映射为 failed，保留上游错误信息。创建接口提前返回完整结果时也可直接保存。

每个图片引用声明 `mimeType=image/svg+xml`。SVG 下载、静态 XML 校验、尺寸识别、原字节保存、归属检查及保存失败恢复由宿主执行。下载地址应返回 `Content-Type: image/svg+xml`。不添加未记录的 Base64 输出字段解析。

查询复用已持久化的上游任务 ID，不重新调用创建接口；查询有效期是出最终结果后两小时。插件未声明上游取消或幂等创建支持，不把本地停止等待描述为上游取消。

<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v2",
  "id": "grsai-svg",
  "name": "GrsAI GPT Image · SVG 矢量图",
  "version": "1.0.0",
  "author": "本地定制",
  "description": "GrsAI gpt-image-2.6 SVG 专用协议，支持参考图和异步查询；需要宿主 SVG 保存支持。",
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
        "id": "grsai-svg-image",
        "label": "GrsAI GPT Image · SVG 矢量图",
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
        "baseUrl": "https://grsaiapi.com",
        "requiresPublicMediaUrls": false,
        "detachedPolling": true,
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
            "values": [
              "auto",
              "1:1",
              "16:9",
              "9:16",
              "4:3",
              "3:4",
              "3:2",
              "2:3",
              "5:4",
              "4:5",
              "21:9",
              "9:21",
              "1:2",
              "2:1",
              "1024x1024",
              "1672x941",
              "941x1672",
              "1443x1090",
              "1090x1443",
              "1536x1024",
              "1024x1536",
              "1408x1120",
              "1120x1408",
              "1920x832",
              "832x1920",
              "896x1792",
              "1792x896"
            ],
            "default": "1024x1024",
            "pricing": true
          },
          {
            "name": "quality",
            "label": "质量",
            "type": "string",
            "binding": "quality",
            "values": [
              "auto"
            ],
            "default": "auto"
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
            "binding": "providerOptions.grsai-svg-image.outputFormat",
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
            "values": [
              "gpt-image-2.6"
            ],
            "description": "仅支持精确上游模型 ID gpt-image-2.6。"
          },
          {
            "name": "prompt",
            "type": "string",
            "required": true,
            "mapping": "prompt",
            "description": "描述 SVG 主体、构图、颜色和矢量风格的提示词。"
          },
          {
            "name": "images",
            "type": "media[]",
            "mapping": "images",
            "description": "可选参考图，支持 URL 或完整 Base64 data URL，不支持蒙版。"
          }
        ],
        "validations": [
          {
            "assert": {
              "$eq": [
                {
                  "$ref": "request.model"
                },
                "gpt-image-2.6"
              ]
            },
            "message": "GrsAI SVG 仅支持 gpt-image-2.6，不支持 flare、sunburst 或其他模型。"
          },
          {
            "assert": {
              "$in": [
                {
                  "$ref": "request.imageCount"
                },
                [
                  0,
                  1
                ]
              ]
            },
            "message": "GrsAI SVG 每次只能生成一张图。"
          },
          {
            "assert": {
              "$in": [
                {
                  "$coalesce": [
                    {
                      "$ref": "request.output.count"
                    },
                    0
                  ]
                },
                [
                  0,
                  1
                ]
              ]
            },
            "message": "GrsAI SVG 每次只能生成一张图。"
          },
          {
            "assert": {
              "$in": [
                {
                  "$ref": "request.quality"
                },
                [
                  "",
                  "auto"
                ]
              ]
            },
            "message": "GrsAI SVG 专用协议仅支持 auto 质量。"
          },
          {
            "assert": {
              "$in": [
                {
                  "$ref": "request.aspectRatio"
                },
                [
                  "",
                  "auto",
                  "1:1",
                  "16:9",
                  "9:16",
                  "4:3",
                  "3:4",
                  "3:2",
                  "2:3",
                  "5:4",
                  "4:5",
                  "21:9",
                  "9:21",
                  "1:2",
                  "2:1",
                  "1024x1024",
                  "1672x941",
                  "941x1672",
                  "1443x1090",
                  "1090x1443",
                  "1536x1024",
                  "1024x1536",
                  "1408x1120",
                  "1120x1408",
                  "1920x832",
                  "832x1920",
                  "896x1792",
                  "1792x896"
                ]
              ]
            },
            "message": "GrsAI SVG 仅支持文档列出的比例和 1K 尺寸。"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.images"
                      },
                      "as": "media",
                      "where": {
                        "$eq": [
                          {
                            "$lower": {
                              "$ref": "media.role"
                            }
                          },
                          "mask"
                        ]
                      }
                    }
                  }
                },
                0
              ]
            },
            "message": "GrsAI SVG 不支持蒙版编辑，请移除蒙版。"
          },
          {
            "assert": {
              "$eq": [
                {
                  "$len": {
                    "$filter": {
                      "from": {
                        "$ref": "request.images"
                      },
                      "as": "media",
                      "where": {
                        "$not": {
                          "$ref": "media.value"
                        }
                      }
                    }
                  }
                },
                0
              ]
            },
            "message": "GrsAI SVG 参考图缺少可用 URL 或 Base64。"
          }
        ],
        "create": {
          "method": "POST",
          "path": "/v1/api/generate",
          "originPath": true,
          "contentType": "application/json",
          "body": {
            "model": "gpt-image-2.6",
            "prompt": {
              "$ref": "request.prompt"
            },
            "images": {
              "$map": {
                "from": {
                  "$sortByOrder": {
                    "$ref": "request.images"
                  }
                },
                "as": "media",
                "in": {
                  "$ref": "media.value"
                }
              }
            },
            "aspectRatio": {
              "$coalesce": [
                {
                  "$ref": "request.aspectRatio"
                },
                "1024x1024"
              ]
            },
            "quality": "auto",
            "outputFormat": "svg",
            "replyType": "async"
          }
        },
        "poll": {
          "method": "GET",
          "path": "/v1/api/result",
          "originPath": true,
          "query": {
            "id": {
              "$ref": "taskId"
            }
          }
        },
        "response": {
          "taskIdPaths": [
            "id"
          ],
          "status": {
            "$if": {
              "condition": {
                "$eq": [
                  {
                    "$ref": "response.status"
                  },
                  "violation"
                ]
              },
              "then": "failed",
              "else": {
                "$ref": "response.status"
              }
            }
          },
          "statusPaths": [
            "status"
          ],
          "errorPaths": [
            "error"
          ],
          "messagePaths": [
            "error",
            "message"
          ],
          "images": {
            "$map": {
              "from": {
                "$ref": "response.results"
              },
              "as": "item",
              "in": {
                "url": {
                  "$omitEmpty": {
                    "$ref": "item.url"
                  }
                },
                "mimeType": "image/svg+xml"
              }
            }
          }
        }
      }
    ]
  },
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>"
}
```
<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_END -->
