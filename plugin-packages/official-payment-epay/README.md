# 易支付

官方格式的 `yingce.payment/v1` 支付插件。支持 V1/MD5 `mapi.php` 接口下单、`submit.php` 页面跳转和 V2/RSA `/api/pay/create`。本地人民币业务通过 `payType` 配置单一支付方式，保留套餐、优惠、邀请奖励、关单与对账合同，不接入汇率。协议实现参考 [acg-faka](https://github.com/lizhipay/acg-faka) 与 [new-api](https://github.com/QuantumNous/new-api)。
