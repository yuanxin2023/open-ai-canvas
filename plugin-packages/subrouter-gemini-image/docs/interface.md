# SubRouter Gemini Image 接口字段

## 协议身份

- 插件 ID：`subrouter-gemini-image`。
- Provider ID：`subrouter-gemini-image`。
- 能力：`image`。
- 默认 Base URL：`https://asiasouth.up.railway.app`。
- 鉴权驱动：`google-api-key`。
- 创建：`POST /v1beta/models/{{model}}:generateContent`。
- 生命周期：同步响应。

## 文档依据

- 官方 API 文档：https://subrouter.ai/docs?tab=channels&source=self&model=gemini-3.1-flash-image&provider=4k-image
- 核对日期：2026-09-29

## 配置字段

| 字段 | 类型 | 必填 | 含义 |
| --- | --- | --- | --- |
| `apiKey` | secret | 是 | API Key |

## 统一字段映射

| 统一字段 | 类型 | 必填 | 上游映射 | 说明 |
| --- | --- | --- | --- | --- |
| `model` | string | 是 | `model` | 图片模型 ID。 |
| `prompt` | string | 是 | `prompt` | 图片提示词。 |
| `images` | media[] | 否 | `provider image/reference fields` | 参考图或编辑源图，role 由业务层确定。 |
| `aspectRatio` | string | 否 | `size/aspect_ratio` | 比例或尺寸，语义按协议说明。 |
| `resolution` | string | 否 | `resolution/imageSize` | 分辨率档位。 |
| `quality` | string | 否 | `quality` | 质量档位。 |
| `providerOptions` | object | 否 | `provider-specific fields` | 插件命名空间内的厂商扩展字段。 |

## 上游请求模板逐字段清单

下表由插件请求模板生成，覆盖 body、query、headers 和 multipart 文件声明中的每个字段。

| 上游位置 | 值或转换表达式 |
| --- | --- |
| `create.method` | `"POST"` |
| `create.path` | `"/v1beta/models/{{model}}:generateContent"` |
| `create.contentType` | `"application/json"` |
| `create.body.contents[0].role` | `"user"` |
| `create.body.contents[0].parts` | `{"$concatArrays":[[{"text":{"$ref":"request.prompt"}}],{"$map":{"from":{"$ref":"request.images"},"as":"media","in":{"$if":{"condition":{"$ref":"media.dataUrl"},"then":{"inlineData":{"mimeType":{"$dataMime":{"$ref":"media.dataUrl"}},"data":{"$dataPayload":{"$ref":"media.dataUrl"}}}},"else":{"fileData":{"mimeType":{"$omitEmpty":{"$ref":"media.mimeType"}},"fileUri":{"$ref":"media.url"}}}}}}}]}` |
| `create.body.generationConfig.responseModalities` | `{"$coalesce":[{"$ref":"request.providerOptions.subrouter-gemini-image.responseModalities"},["TEXT","IMAGE"]]}` |
| `create.body.generationConfig.imageConfig.aspectRatio` | `{"$omitEmpty":{"$coalesce":[{"$ref":"request.output.aspectRatio"},{"$ref":"request.aspectRatio"}]}}` |
| `create.body.generationConfig.imageConfig.imageSize` | `{"$omitEmpty":{"$coalesce":[{"$switch":{"cases":[{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.quality"},{"$ref":"request.quality"}]}},["1k","low"]]},"then":"1K"},{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.quality"},{"$ref":"request.quality"}]}},["2k","medium"]]},"then":"2K"},{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.quality"},{"$ref":"request.quality"}]}},["4k","high"]]},"then":"4K"}],"default":null}},{"$switch":{"cases":[{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.resolution"},{"$ref":"request.resolution"}]}},["1k","low"]]},"then":"1K"},{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.resolution"},{"$ref":"request.resolution"}]}},["2k","medium"]]},"then":"2K"},{"when":{"$in":[{"$lower":{"$coalesce":[{"$ref":"request.output.resolution"},{"$ref":"request.resolution"}]}},["4k","high"]]},"then":"4K"}],"default":null}}]}}` |
| `create.body.generationConfig.temperature` | `{"$omitEmpty":{"$ref":"request.providerOptions.subrouter-gemini-image.temperature"}}` |
| `create.body.generationConfig.topP` | `{"$omitEmpty":{"$ref":"request.providerOptions.subrouter-gemini-image.topP"}}` |
| `create.body.generationConfig.topK` | `{"$omitEmpty":{"$ref":"request.providerOptions.subrouter-gemini-image.topK"}}` |
| `create.body.generationConfig.seed` | `{"$omitEmpty":{"$ref":"request.providerOptions.subrouter-gemini-image.seed"}}` |
| `create.body.safetySettings` | `{"$omitEmpty":{"$ref":"request.providerOptions.subrouter-gemini-image.safetySettings"}}` |
| `create.body.systemInstruction` | `{"$omitEmpty":{"$coalesce":[{"$ref":"request.providerOptions.subrouter-gemini-image.systemInstruction"},{"$if":{"condition":{"$ref":"request.instructions"},"then":{"parts":[{"text":{"$ref":"request.instructions"}}]},"else":null}}]}}` |

## Provider 扩展键

- `providerOptions.subrouter-gemini-image.responseModalities`
- `providerOptions.subrouter-gemini-image.safetySettings`
- `providerOptions.subrouter-gemini-image.seed`
- `providerOptions.subrouter-gemini-image.systemInstruction`
- `providerOptions.subrouter-gemini-image.temperature`
- `providerOptions.subrouter-gemini-image.topK`
- `providerOptions.subrouter-gemini-image.topP`

动态模型或工作流允许使用文档声明的完整 `parameters/input/extra_body` 对象；该对象是协议本身的开放 schema，不会被宿主裁剪。

## 响应映射逐字段清单

| 映射位置 | 上游路径或转换表达式 |
| --- | --- |
| `response.status` | `"succeeded"` |
| `response.images` | `{"$concatArrays":[{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$or":[{"$ref":"part.inlineData"},{"$ref":"part.inline_data"}]}}},"as":"part","in":{"dataUrl":{"$concat":["data:",{"$coalesce":[{"$ref":"part.inlineData.mimeType"},{"$ref":"part.inline_data.mime_type"},"image/png"]},";base64,",{"$coalesce":[{"$ref":"part.inlineData.data"},{"$ref":"part.inline_data.data"}]}]}}}},{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$gt":[{"$len":{"$split":[{"$ref":"part.text"},"]("]}},1]}}},"as":"part","in":{"url":{"$trim":{"$at":[{"$split":[{"$at":[{"$split":[{"$ref":"part.text"},"]("]},1]},")"]},0]}}}}}]}` |
| `response.text` | `{"$map":{"from":{"$filter":{"from":{"$ref":"response.candidates.0.content.parts"},"as":"part","where":{"$ref":"part.text"}}},"as":"part","in":{"$ref":"part.text"}}}` |
| `response.usage` | `{"$ref":"response.usageMetadata"}` |
| `response.errorPaths[0]` | `"error.code"` |
| `response.messagePaths[0]` | `"error.message"` |
| `response.resultEphemeral` | `true` |

## 响应与错误

插件把上游 task/status/text/media/usage 映射为统一结果。临时媒体 URL 标记为 ephemeral，由宿主立即下载持久化。HTTP 错误、业务 code 和 error object 保持失败语义，不包装成成功。

## 兼容边界与待确认项

支持同步文生图与图生图，不限制具体模型 ID；模型必须兼容 Gemini Image generateContent 请求结构。参考图从 request.images 写入 contents[0].parts：Data URL 转为 inlineData，远程 URL 转为 fileData。响应兼容标准 inlineData 和 parts[].text 中的 Markdown 图片链接。尚未确认批量出图和异步任务，因此插件不声明这些能力。签名链接标称 5 小时有效，宿主必须立即下载持久化。

<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "open-ai-canvas.plugin/v2",
  "id": "subrouter-gemini-image",
  "name": "SubRouter Gemini Image",
  "version": "2.1.0",
  "author": "SubRouter",
  "description": "SubRouter Gemini Image 独立请求协议插件。",
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>",
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
        "id": "subrouter-gemini-image",
        "label": "SubRouter Gemini Image",
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
        "baseUrl": "https://asiasouth.up.railway.app",
        "requiresPublicMediaUrls": false,
        "auth": {
          "type": "google-api-key",
          "field": "apiKey"
        },
        "parameters": [
          {
            "name": "model",
            "type": "string",
            "required": true,
            "mapping": "model",
            "description": "图片模型 ID。"
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
          "path": "/v1beta/models/{{model}}:generateContent",
          "contentType": "application/json",
          "body": {
            "contents": [
              {
                "role": "user",
                "parts": {
                  "$concatArrays": [
                    [
                      {
                        "text": {
                          "$ref": "request.prompt"
                        }
                      }
                    ],
                    {
                      "$map": {
                        "from": {
                          "$ref": "request.images"
                        },
                        "as": "media",
                        "in": {
                          "$if": {
                            "condition": {
                              "$ref": "media.dataUrl"
                            },
                            "then": {
                              "inlineData": {
                                "mimeType": {
                                  "$dataMime": {
                                    "$ref": "media.dataUrl"
                                  }
                                },
                                "data": {
                                  "$dataPayload": {
                                    "$ref": "media.dataUrl"
                                  }
                                }
                              }
                            },
                            "else": {
                              "fileData": {
                                "mimeType": {
                                  "$omitEmpty": {
                                    "$ref": "media.mimeType"
                                  }
                                },
                                "fileUri": {
                                  "$ref": "media.url"
                                }
                              }
                            }
                          }
                        }
                      }
                    }
                  ]
                }
              }
            ],
            "generationConfig": {
              "responseModalities": {
                "$coalesce": [
                  {
                    "$ref": "request.providerOptions.subrouter-gemini-image.responseModalities"
                  },
                  [
                    "TEXT",
                    "IMAGE"
                  ]
                ]
              },
              "imageConfig": {
                "aspectRatio": {
                  "$omitEmpty": {
                    "$coalesce": [
                      {
                        "$ref": "request.output.aspectRatio"
                      },
                      {
                        "$ref": "request.aspectRatio"
                      }
                    ]
                  }
                },
                "imageSize": {
                  "$omitEmpty": {
                    "$coalesce": [
                      {
                        "$switch": {
                          "cases": [
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.quality"
                                        },
                                        {
                                          "$ref": "request.quality"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "1k",
                                    "low"
                                  ]
                                ]
                              },
                              "then": "1K"
                            },
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.quality"
                                        },
                                        {
                                          "$ref": "request.quality"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "2k",
                                    "medium"
                                  ]
                                ]
                              },
                              "then": "2K"
                            },
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.quality"
                                        },
                                        {
                                          "$ref": "request.quality"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "4k",
                                    "high"
                                  ]
                                ]
                              },
                              "then": "4K"
                            }
                          ],
                          "default": null
                        }
                      },
                      {
                        "$switch": {
                          "cases": [
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.resolution"
                                        },
                                        {
                                          "$ref": "request.resolution"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "1k",
                                    "low"
                                  ]
                                ]
                              },
                              "then": "1K"
                            },
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.resolution"
                                        },
                                        {
                                          "$ref": "request.resolution"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "2k",
                                    "medium"
                                  ]
                                ]
                              },
                              "then": "2K"
                            },
                            {
                              "when": {
                                "$in": [
                                  {
                                    "$lower": {
                                      "$coalesce": [
                                        {
                                          "$ref": "request.output.resolution"
                                        },
                                        {
                                          "$ref": "request.resolution"
                                        }
                                      ]
                                    }
                                  },
                                  [
                                    "4k",
                                    "high"
                                  ]
                                ]
                              },
                              "then": "4K"
                            }
                          ],
                          "default": null
                        }
                      }
                    ]
                  }
                }
              },
              "temperature": {
                "$omitEmpty": {
                  "$ref": "request.providerOptions.subrouter-gemini-image.temperature"
                }
              },
              "topP": {
                "$omitEmpty": {
                  "$ref": "request.providerOptions.subrouter-gemini-image.topP"
                }
              },
              "topK": {
                "$omitEmpty": {
                  "$ref": "request.providerOptions.subrouter-gemini-image.topK"
                }
              },
              "seed": {
                "$omitEmpty": {
                  "$ref": "request.providerOptions.subrouter-gemini-image.seed"
                }
              }
            },
            "safetySettings": {
              "$omitEmpty": {
                "$ref": "request.providerOptions.subrouter-gemini-image.safetySettings"
              }
            },
            "systemInstruction": {
              "$omitEmpty": {
                "$coalesce": [
                  {
                    "$ref": "request.providerOptions.subrouter-gemini-image.systemInstruction"
                  },
                  {
                    "$if": {
                      "condition": {
                        "$ref": "request.instructions"
                      },
                      "then": {
                        "parts": [
                          {
                            "text": {
                              "$ref": "request.instructions"
                            }
                          }
                        ]
                      },
                      "else": null
                    }
                  }
                ]
              }
            }
          }
        },
        "response": {
          "status": "succeeded",
          "images": {
            "$concatArrays": [
              {
                "$map": {
                  "from": {
                    "$filter": {
                      "from": {
                        "$ref": "response.candidates.0.content.parts"
                      },
                      "as": "part",
                      "where": {
                        "$or": [
                          {
                            "$ref": "part.inlineData"
                          },
                          {
                            "$ref": "part.inline_data"
                          }
                        ]
                      }
                    }
                  },
                  "as": "part",
                  "in": {
                    "dataUrl": {
                      "$concat": [
                        "data:",
                        {
                          "$coalesce": [
                            {
                              "$ref": "part.inlineData.mimeType"
                            },
                            {
                              "$ref": "part.inline_data.mime_type"
                            },
                            "image/png"
                          ]
                        },
                        ";base64,",
                        {
                          "$coalesce": [
                            {
                              "$ref": "part.inlineData.data"
                            },
                            {
                              "$ref": "part.inline_data.data"
                            }
                          ]
                        }
                      ]
                    }
                  }
                }
              },
              {
                "$map": {
                  "from": {
                    "$filter": {
                      "from": {
                        "$ref": "response.candidates.0.content.parts"
                      },
                      "as": "part",
                      "where": {
                        "$gt": [
                          {
                            "$len": {
                              "$split": [
                                {
                                  "$ref": "part.text"
                                },
                                "]("
                              ]
                            }
                          },
                          1
                        ]
                      }
                    }
                  },
                  "as": "part",
                  "in": {
                    "url": {
                      "$trim": {
                        "$at": [
                          {
                            "$split": [
                              {
                                "$at": [
                                  {
                                    "$split": [
                                      {
                                        "$ref": "part.text"
                                      },
                                      "]("
                                    ]
                                  },
                                  1
                                ]
                              },
                              ")"
                            ]
                          },
                          0
                        ]
                      }
                    }
                  }
                }
              }
            ]
          },
          "text": {
            "$map": {
              "from": {
                "$filter": {
                  "from": {
                    "$ref": "response.candidates.0.content.parts"
                  },
                  "as": "part",
                  "where": {
                    "$ref": "part.text"
                  }
                }
              },
              "as": "part",
              "in": {
                "$ref": "part.text"
              }
            }
          },
          "usage": {
            "$ref": "response.usageMetadata"
          },
          "errorPaths": [
            "error.code"
          ],
          "messagePaths": [
            "error.message"
          ],
          "resultEphemeral": true
        }
      }
    ]
  }
}
```
<!-- OPEN_AI_CANVAS_MANIFEST_CONTRACT_END -->
