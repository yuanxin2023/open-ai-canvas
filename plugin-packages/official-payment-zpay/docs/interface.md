## yingce.payment/v1

支持 `validate_config`、`create_order`、`query_order` 和 `verify_notification`。ZPAY 没有可用的主动关单与日账单下载接口，因此插件不声明对应权限；订单到期和晚到账补查由宿主处理。

支付宝和微信是同一插件包的两个独立 Provider，分别配置和启停。前台名称为“支付宝支付”和“微信支付”，后台插件名称为“ZPAY 聚合支付”。

<!-- YINGCE_MANIFEST_CONTRACT_START -->
## Manifest 完整接口定义

以下 JSON 与插件包内实际 `manifest.json` 逐字段一致，覆盖插件身份、权限、配置、鉴权、参数、校验、创建、Agent、查询、取消、结果下载、响应和 Agent 响应映射。`documentation` 字段的值就是当前完整文档；为避免文档在自身内部无限递归，JSON 中仅用等义占位文本表示正文。

```json
{
  "apiVersion": "yingce.plugin/v1",
  "id": "official-payment-zpay",
  "name": "ZPAY 聚合支付",
  "version": "1.0.0",
  "author": "ZPAY",
  "description": "通过 ZPAY/EasyPay API 提供支付宝和微信扫码充值。",
  "enabled": true,
  "installable": true,
  "runtime": {
    "backend": "rpc",
    "backendEntry": "backend/provider"
  },
  "surfaces": [
    "wallet",
    "settings"
  ],
  "permissions": [
    "payment.create",
    "payment.query"
  ],
  "configuration": {
    "fields": [
      {
        "name": "publicBaseUrl",
        "type": "url",
        "label": "服务器公网地址",
        "required": true,
        "description": "用于生成 ZPAY 异步通知地址，必须可被 ZPAY 访问。"
      },
      {
        "name": "apiBaseUrl",
        "type": "url",
        "label": "ZPAY API 地址",
        "required": true,
        "default": "https://zpayz.cn",
        "description": "只允许公网 HTTPS 根地址。"
      },
      {
        "name": "pid",
        "type": "string",
        "label": "商户 ID",
        "required": true
      },
      {
        "name": "merchantKey",
        "type": "password",
        "label": "商户密钥",
        "required": true,
        "secret": true
      },
      {
        "name": "cid",
        "type": "string",
        "label": "支付渠道 ID",
        "description": "可选；多个渠道 ID 使用英文逗号分隔。"
      }
    ]
  },
  "contributes": {
    "paymentProviders": [
      {
        "id": "zpay-alipay-qr",
        "label": "支付宝支付",
        "icon": "brand:alipay",
        "checkoutMode": "qr_code",
        "identityFields": [
          "apiBaseUrl",
          "pid"
        ],
        "expiryPolicy": {
          "defaultMinutes": 30,
          "minMinutes": 5,
          "maxMinutes": 1440
        },
        "notificationSuccess": {
          "status": 200,
          "contentType": "text/plain; charset=utf-8",
          "body": "success"
        },
        "notificationFailure": {
          "status": 400,
          "contentType": "text/plain; charset=utf-8",
          "body": "failure"
        }
      },
      {
        "id": "zpay-wechat-qr",
        "label": "微信支付",
        "icon": "brand:wechat-pay",
        "checkoutMode": "qr_code",
        "identityFields": [
          "apiBaseUrl",
          "pid"
        ],
        "expiryPolicy": {
          "defaultMinutes": 30,
          "minMinutes": 5,
          "maxMinutes": 1440
        },
        "notificationSuccess": {
          "status": 200,
          "contentType": "text/plain; charset=utf-8",
          "body": "success"
        },
        "notificationFailure": {
          "status": 400,
          "contentType": "text/plain; charset=utf-8",
          "body": "failure"
        }
      }
    ]
  },
  "documentation": "<当前插件的完整 documentation，由 README.md 与 docs/interface.md 拼接而成；为避免 JSON 递归，此处不重复展开正文。>"
}
```
<!-- YINGCE_MANIFEST_CONTRACT_END -->
