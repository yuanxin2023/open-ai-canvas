package paymentplugins

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultZPayAPIBaseURL = "https://zpayz.cn"
	maxZPayResponseBytes  = 1 << 20
)

type ZPayProvider struct {
	client      *http.Client
	paymentType string
	now         func() time.Time
}

func NewZPayProvider(client *http.Client, paymentType string) *ZPayProvider {
	if client == nil {
		client = NewZPayHTTPClient()
	}
	return &ZPayProvider{client: client, paymentType: strings.TrimSpace(paymentType), now: time.Now}
}

func NewZPayHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	resolver := net.DefaultResolver
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		addresses, err := resolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		if len(addresses) == 0 {
			return nil, errors.New("ZPAY 网关域名没有可用地址")
		}
		for _, address := range addresses {
			if !isPublicZPayAddress(address) {
				return nil, errors.New("ZPAY 网关解析到了非公网地址")
			}
		}
		var lastErr error
		for _, address := range addresses {
			connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.String(), port))
			if dialErr == nil {
				return connection, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
	client := &http.Client{Transport: transport, Timeout: 25 * time.Second}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("ZPAY 网关重定向次数过多")
		}
		if len(via) == 0 || !strings.EqualFold(request.URL.Host, via[0].URL.Host) {
			return errors.New("ZPAY 网关不允许跨主机重定向")
		}
		return validateZPayAPIBaseURL(request.URL.Scheme + "://" + request.URL.Host)
	}
	return client
}

func (p *ZPayProvider) Descriptor() Descriptor {
	id, name, icon := ProviderZPayAlipay, "支付宝支付", "brand:alipay"
	if p.paymentType == "wxpay" {
		id, name, icon = ProviderZPayWeChat, "微信支付", "brand:wechat-pay"
	}
	return Descriptor{
		ID: id, PluginID: PluginZPay, PluginVersion: "1.0.0", Name: name, Icon: icon, CheckoutMode: "qr_code",
		IdentityFields:      []string{"apiBaseUrl", "pid"},
		NotificationSuccess: NotificationResponse{Status: 200, ContentType: "text/plain; charset=utf-8", Body: "success"},
		NotificationFailure: NotificationResponse{Status: 400, ContentType: "text/plain; charset=utf-8", Body: "failure"},
	}
}

func (p *ZPayProvider) ValidateConfig(config Config) error {
	if p.paymentType != "alipay" && p.paymentType != "wxpay" {
		return errors.New("ZPAY 支付类型无效")
	}
	pid := strings.TrimSpace(config["pid"])
	if pid == "" || len(pid) > 64 || strings.ContainsAny(pid, "\r\n&=") {
		return errors.New("ZPAY 商户 ID 无效")
	}
	merchantKey := strings.TrimSpace(config["merchantKey"])
	if merchantKey == "" || len(merchantKey) > 512 || strings.ContainsAny(merchantKey, "\r\n") {
		return errors.New("ZPAY 商户密钥无效")
	}
	if cid := strings.TrimSpace(config["cid"]); len(cid) > 500 || strings.ContainsAny(cid, "\r\n&=") {
		return errors.New("ZPAY 支付渠道 ID 无效")
	}
	return validateZPayAPIBaseURL(zpayAPIBaseURL(config))
}

func (p *ZPayProvider) CreateOrder(ctx context.Context, config Config, request CreateRequest) (Checkout, error) {
	if err := p.ValidateConfig(config); err != nil {
		return Checkout{}, err
	}
	if request.AmountFen <= 0 || request.Currency != "CNY" || request.MerchantOrderNo == "" || len(request.MerchantOrderNo) > 32 || request.NotifyURL == "" || net.ParseIP(strings.TrimSpace(request.ClientIP)) == nil {
		return Checkout{}, errors.New("ZPAY 扫码支付下单参数无效")
	}
	if err := validateZPayNotifyURL(request.NotifyURL); err != nil {
		return Checkout{}, err
	}
	values := url.Values{
		"pid":          {strings.TrimSpace(config["pid"])},
		"type":         {p.paymentType},
		"out_trade_no": {request.MerchantOrderNo},
		"notify_url":   {request.NotifyURL},
		"name":         {truncateUTF8(strings.TrimSpace(request.Description), 100)},
		"money":        {formatFen(request.AmountFen)},
		"clientip":     {strings.TrimSpace(request.ClientIP)},
	}
	if cid := strings.TrimSpace(config["cid"]); cid != "" {
		values.Set("cid", cid)
	}
	values.Set("sign", signZPayValues(values, config["merchantKey"]))
	values.Set("sign_type", "MD5")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, key := range sortedZPayKeys(values) {
		if err := writer.WriteField(key, values.Get(key)); err != nil {
			return Checkout{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Checkout{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, zpayEndpoint(config, "/mapi.php"), &body)
	if err != nil {
		return Checkout{}, err
	}
	httpRequest.Header.Set("Content-Type", writer.FormDataContentType())
	var response zpayCreateResponse
	if err := p.do(httpRequest, &response); err != nil {
		return Checkout{}, err
	}
	if response.Code.String() != "1" {
		return Checkout{}, zpayRejected("zpay_create_rejected", response.Message)
	}
	checkoutValue := firstNonEmptyZPay(response.QRCode, response.PayURL, response.PayURL2)
	if checkoutValue == "" {
		return Checkout{}, errors.New("ZPAY 下单成功但未返回二维码内容")
	}
	checkoutValue = resolveZPayCheckoutValue(zpayAPIBaseURL(config), checkoutValue)
	return Checkout{Mode: "qr_code", Value: checkoutValue, ExpiresAt: request.ExpiresAt}, nil
}

func (p *ZPayProvider) QueryOrder(ctx context.Context, config Config, request QueryRequest) (Result, error) {
	if err := p.ValidateConfig(config); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(request.MerchantOrderNo) == "" {
		return Result{}, errors.New("ZPAY 查单缺少商户订单号")
	}
	query := url.Values{
		"act":          {"order"},
		"pid":          {strings.TrimSpace(config["pid"])},
		"key":          {config["merchantKey"]},
		"out_trade_no": {strings.TrimSpace(request.MerchantOrderNo)},
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, zpayEndpoint(config, "/api.php")+"?"+query.Encode(), nil)
	if err != nil {
		return Result{}, err
	}
	var response zpayQueryResponse
	if err := p.do(httpRequest, &response); err != nil {
		return Result{}, err
	}
	if response.Code.String() != "1" {
		if strings.Contains(response.Message, "不存在") || strings.Contains(strings.ToLower(response.Message), "not found") {
			return Result{}, ErrOrderNotFound
		}
		return Result{}, zpayRejected("zpay_query_rejected", response.Message)
	}
	if response.PID.String() != strings.TrimSpace(config["pid"]) || response.Type != p.paymentType || response.OutTradeNo != request.MerchantOrderNo {
		return Result{}, errors.New("ZPAY 查单结果与商户或支付渠道不匹配")
	}
	amountFen, err := parseYuanToFen(response.Money)
	if err != nil {
		return Result{}, errors.New("ZPAY 查单金额无效")
	}
	paid := response.Status.String() == "1"
	if paid && strings.TrimSpace(response.TradeNo) == "" {
		return Result{}, errors.New("ZPAY 已支付订单缺少交易号")
	}
	paidAt, _ := time.ParseInLocation("2006-01-02 15:04:05", response.EndTime, time.FixedZone("CST", 8*60*60))
	providerStatus := "WAIT_BUYER_PAY"
	if paid {
		providerStatus = "TRADE_SUCCESS"
	}
	return Result{
		MerchantOrderNo: response.OutTradeNo, ProviderTradeNo: response.TradeNo, ProviderStatus: providerStatus,
		AmountFen: amountFen, Currency: "CNY", Paid: paid, PaidAt: paidAt,
	}, nil
}

func (p *ZPayProvider) CloseOrder(context.Context, Config, CloseRequest) (Result, error) {
	return Result{}, &ProviderError{Code: "zpay_close_unsupported", Message: "ZPAY 不支持主动关单"}
}

func (p *ZPayProvider) VerifyNotification(_ context.Context, config Config, _ http.Header, rawBody []byte) (Notification, error) {
	if err := p.ValidateConfig(config); err != nil {
		return Notification{}, err
	}
	values, err := url.ParseQuery(string(rawBody))
	if err != nil {
		return Notification{}, errors.New("ZPAY 通知参数无效")
	}
	for key, entries := range values {
		if len(entries) != 1 {
			return Notification{}, fmt.Errorf("ZPAY 通知参数 %s 重复", key)
		}
	}
	providedSignature := strings.ToLower(strings.TrimSpace(values.Get("sign")))
	if len(providedSignature) != md5.Size*2 || !strings.EqualFold(values.Get("sign_type"), "MD5") {
		return Notification{}, errors.New("ZPAY 通知签名参数缺失")
	}
	expectedSignature := signZPayValues(values, config["merchantKey"])
	if subtle.ConstantTimeCompare([]byte(providedSignature), []byte(expectedSignature)) != 1 {
		return Notification{}, errors.New("ZPAY 通知签名无效")
	}
	if values.Get("pid") != strings.TrimSpace(config["pid"]) || values.Get("type") != p.paymentType || values.Get("trade_status") != "TRADE_SUCCESS" {
		return Notification{}, errors.New("ZPAY 通知商户、支付渠道或交易状态不匹配")
	}
	merchantOrderNo := strings.TrimSpace(values.Get("out_trade_no"))
	tradeNo := strings.TrimSpace(values.Get("trade_no"))
	if merchantOrderNo == "" || tradeNo == "" {
		return Notification{}, errors.New("ZPAY 通知订单号无效")
	}
	amountFen, err := parseYuanToFen(values.Get("money"))
	if err != nil {
		return Notification{}, errors.New("ZPAY 通知金额无效")
	}
	return Notification{EventID: tradeNo + ":TRADE_SUCCESS", Result: Result{
		MerchantOrderNo: merchantOrderNo, ProviderTradeNo: tradeNo, ProviderStatus: "TRADE_SUCCESS",
		AmountFen: amountFen, Currency: "CNY", Paid: true,
	}}, nil
}

func (p *ZPayProvider) DownloadTradeBill(context.Context, Config, time.Time) ([]BillRecord, error) {
	return nil, ErrTradeBillNotFound
}

func (p *ZPayProvider) do(request *http.Request, output any) error {
	response, err := p.client.Do(request)
	if err != nil {
		return &ProviderError{Code: "zpay_network_error", Message: "请求 ZPAY 网关失败", Temporary: true, Cause: err}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &ProviderError{Code: "zpay_http_error", Message: "ZPAY 网关返回异常状态", Temporary: response.StatusCode >= 500}
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxZPayResponseBytes+1))
	if err != nil {
		return &ProviderError{Code: "zpay_read_error", Message: "读取 ZPAY 网关响应失败", Temporary: true, Cause: err}
	}
	if len(raw) > maxZPayResponseBytes {
		return errors.New("ZPAY 网关响应超过安全限制")
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return &ProviderError{Code: "zpay_invalid_response", Message: "ZPAY 网关返回了无效响应", Cause: err}
	}
	return nil
}

type zpayScalar string

func (value *zpayScalar) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*value = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = zpayScalar(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*value = zpayScalar(number.String())
	return nil
}

func (value zpayScalar) String() string { return strings.TrimSpace(string(value)) }

type zpayCreateResponse struct {
	Code    zpayScalar `json:"code"`
	Message string     `json:"msg"`
	PayURL  string     `json:"payurl"`
	PayURL2 string     `json:"payurl2"`
	QRCode  string     `json:"qrcode"`
}

type zpayQueryResponse struct {
	Code       zpayScalar `json:"code"`
	Message    string     `json:"msg"`
	PID        zpayScalar `json:"pid"`
	Status     zpayScalar `json:"status"`
	Type       string     `json:"type"`
	TradeNo    string     `json:"trade_no"`
	OutTradeNo string     `json:"out_trade_no"`
	Money      string     `json:"money"`
	EndTime    string     `json:"endtime"`
}

func zpayAPIBaseURL(config Config) string {
	if value := strings.TrimRight(strings.TrimSpace(config["apiBaseUrl"]), "/"); value != "" {
		return value
	}
	return defaultZPayAPIBaseURL
}

func zpayEndpoint(config Config, endpointPath string) string {
	return zpayAPIBaseURL(config) + endpointPath
}

func validateZPayAPIBaseURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return errors.New("ZPAY API 地址必须是公网 HTTPS 根地址")
	}
	if port := parsed.Port(); port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return errors.New("ZPAY API 地址端口无效")
		}
	}
	if address, err := netip.ParseAddr(parsed.Hostname()); err == nil && !isPublicZPayAddress(address) {
		return errors.New("ZPAY API 地址不能使用非公网 IP")
	}
	return nil
}

func validateZPayNotifyURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("ZPAY 通知地址无效或包含查询参数")
	}
	return nil
}

func isPublicZPayAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	blocked := []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001:db8::/32"),
	}
	for _, prefix := range blocked {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func signZPayValues(values url.Values, merchantKey string) string {
	parts := make([]string, 0, len(values))
	for _, key := range sortedZPayKeys(values) {
		if key == "sign" || key == "sign_type" || values.Get(key) == "" {
			continue
		}
		parts = append(parts, key+"="+values.Get(key))
	}
	digest := md5.Sum([]byte(strings.Join(parts, "&") + merchantKey))
	return hex.EncodeToString(digest[:])
}

func sortedZPayKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstNonEmptyZPay(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func resolveZPayCheckoutValue(baseURL, value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() {
		return value
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil {
		return value
	}
	return base.ResolveReference(parsed).String()
}

func zpayRejected(code, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "ZPAY 网关拒绝了请求"
	}
	return &ProviderError{Code: code, Message: truncateUTF8(message, 300)}
}
