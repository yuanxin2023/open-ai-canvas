// 充值商品：前台可购买的积分档位与管理端增改。

package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type PaymentCatalog struct {
	Products   []model.TopupProduct   `json:"products"`
	Promotion  PublicPaymentPromotion `json:"promotion"`
	ServerTime time.Time              `json:"serverTime"`
}

func (s *Service) TopupProducts(actor *model.User) (*PaymentCatalog, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	if err := s.RequireFeature(FeatureCredits); err != nil {
		return nil, err
	}
	products, err := s.repo.TopupProducts(false)
	if err != nil {
		return nil, err
	}
	setting, promotion, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	publicPromotion := s.publicPaymentPromotion(setting, promotion, now)
	active := publicPromotion.Visible && paymentPromotionActive(promotion, now)
	for index := range products {
		products[index] = publicTopupProduct(products[index], active)
	}
	products = decorateTopupProducts(products, now)
	return &PaymentCatalog{Products: products, Promotion: publicPromotion, ServerTime: now}, nil
}

func (s *Service) AdminTopupProducts(actor *model.User) ([]model.TopupProduct, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	products, err := s.repo.TopupProducts(true)
	if err != nil {
		return nil, err
	}
	return decorateTopupProducts(products, time.Now()), nil
}

func decorateTopupProducts(products []model.TopupProduct, now time.Time) []model.TopupProduct {
	for index := range products {
		product := &products[index]
		if product.SaleStrategy == "" {
			product.SaleStrategy = model.TopupSaleStrategyUnlimited
		}
		product.SaleStatus, product.CanPurchase = product.SaleAvailability(now)
	}
	return products
}

func publicTopupProduct(product model.TopupProduct, promotionActive bool) model.TopupProduct {
	if product.CompareAmountFen <= 0 {
		return product
	}
	if promotionActive {
		return product
	}
	product.AmountFen = product.CompareAmountFen
	product.CompareAmountFen = 0
	return product
}

// The resolver captures promotion configuration before the product transaction;
// computing the amount under the product lock performs no additional I/O.
func (s *Service) topupAmountResolver() (func(model.TopupProduct, time.Time) int64, error) {
	_, promotion, err := s.readPaymentPromotion()
	if err != nil {
		return nil, err
	}
	imageReady := false
	if promotion.ImageResourceID != "" {
		_, imageErr := s.paymentPromotionImageResource(promotion.ImageResourceID)
		imageReady = imageErr == nil
	}
	return func(product model.TopupProduct, now time.Time) int64 {
		return publicTopupProduct(product, imageReady && paymentPromotionActive(promotion, now)).AmountFen
	}, nil
}

func (s *Service) effectiveTopupProductAmount(product model.TopupProduct, now time.Time) (int64, error) {
	amount, err := s.topupAmountResolver()
	if err != nil {
		return 0, err
	}
	return amount(product, now), nil
}

func (s *Service) CreateTopupProduct(actor *model.User, request TopupProductRequest) (*model.TopupProduct, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	product, err := topupProductFromRequest(newID(), actor.ID, request)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateTopupProduct(product); err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "topup_product.create", "topup_product", product.ID, "创建积分充值商品", map[string]any{"amountFen": product.AmountFen, "creditsMicrocredits": product.CreditsMicrocredits, "saleStrategy": product.SaleStrategy}); err != nil {
		return nil, err
	}
	return product, nil
}

func (s *Service) UpdateTopupProduct(actor *model.User, id string, request TopupProductRequest) (*model.TopupProduct, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionProducts); err != nil {
		return nil, err
	}
	id = strings.TrimSpace(id)
	var product *model.TopupProduct
	// 已售数量必须在锁住商品行之后计算，否则会覆盖并发下单扣减或关单退回的库存。
	err := s.repo.UpdateTopupProduct(id, func(existing *model.TopupProduct) (*model.TopupProduct, error) {
		consumed := existing.StockTotal - existing.StockRemaining
		built, buildErr := topupProductFromRequest(id, actor.ID, request, existing.StockTotal, consumed)
		if buildErr != nil {
			return nil, buildErr
		}
		if built.SaleStrategy != model.TopupSaleStrategyInventory {
			built.StockTotal, built.StockRemaining = existing.StockTotal, existing.StockRemaining
		}
		product = built
		return built, nil
	})
	if errors.Is(err, repository.ErrTopupReservationsPending) {
		return nil, NewAppError(http.StatusConflict, err.Error())
	}
	if err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "topup_product.update", "topup_product", product.ID, "更新积分充值商品", map[string]any{"enabled": product.Enabled, "saleStrategy": product.SaleStrategy}); err != nil {
		return nil, err
	}
	return s.repo.TopupProduct(product.ID)
}

func topupProductFromRequest(id, actorID string, request TopupProductRequest, stockState ...int64) (*model.TopupProduct, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" || len([]rune(name)) > 120 {
		return nil, BadAuthRequest("充值商品名称不能为空且不能超过 120 个字符")
	}
	if request.AmountFen <= 0 || request.AmountFen > 100_000_000 {
		return nil, BadAuthRequest("充值金额必须为 1 分至 100 万元")
	}
	if request.CreditsMicrocredits <= 0 || request.CreditsMicrocredits > maxTopupCreditsMicrocredits {
		return nil, BadAuthRequest("充值积分必须为 0.01 至 10 亿积分")
	}
	if !validCreditPrecision(request.CreditsMicrocredits) {
		return nil, BadAuthRequest("充值积分最多保留 2 位小数")
	}
	if request.CompareAmountFen < 0 || request.CompareAmountFen > 100_000_000 || (request.CompareAmountFen > 0 && request.CompareAmountFen <= request.AmountFen) {
		return nil, BadAuthRequest("对比价必须高于售价且不超过 100 万元")
	}
	for _, field := range []struct {
		value string
		limit int
	}{{request.RibbonText, 120}, {request.BadgeText, 80}, {request.PriceCaption, 240}, {request.QuotaCaption, 120}, {request.QuotaDetail, 240}, {request.ActionText, 80}} {
		if len([]rune(strings.TrimSpace(field.value))) > field.limit {
			return nil, BadAuthRequest("套餐卡片文案超过长度限制")
		}
	}
	accentColor := strings.ToUpper(strings.TrimSpace(request.AccentColor))
	if accentColor == "" {
		accentColor = "#D8FF4F"
	}
	if len(accentColor) != 7 || accentColor[0] != '#' || strings.IndexFunc(accentColor[1:], func(value rune) bool {
		return !((value >= '0' && value <= '9') || (value >= 'A' && value <= 'F'))
	}) >= 0 {
		return nil, BadAuthRequest("套餐主题颜色必须为 6 位十六进制颜色")
	}
	var previousTotal, consumed int64
	if len(stockState) > 0 {
		previousTotal = stockState[0]
	}
	if len(stockState) > 1 {
		consumed = stockState[1]
	}
	strategy := request.SaleStrategy
	if strategy == "" {
		strategy = model.TopupSaleStrategyUnlimited
	}
	if strategy != model.TopupSaleStrategyUnlimited && strategy != model.TopupSaleStrategyPeriodic && strategy != model.TopupSaleStrategyInventory && strategy != model.TopupSaleStrategyTimed {
		return nil, BadAuthRequest("上架策略无效")
	}
	if strategy == model.TopupSaleStrategyPeriodic && (request.PeriodDays <= 0 || request.PeriodDays > 3650 || request.PeriodPurchaseLimit <= 0 || request.PeriodPurchaseLimit > 100000) {
		return nil, BadAuthRequest("周期性商品需设置有效的周期天数和购买次数")
	}
	if strategy == model.TopupSaleStrategyInventory && (request.StockTotal <= 0 || request.StockTotal > 9_000_000_000) {
		return nil, BadAuthRequest("库存性商品库存必须为 1 至 90 亿")
	}
	if strategy == model.TopupSaleStrategyTimed && (request.SaleStartAt == nil || request.SaleEndAt == nil || !request.SaleEndAt.After(*request.SaleStartAt)) {
		return nil, BadAuthRequest("限时商品必须设置有效的发售和结束时间")
	}
	stockRemaining := int64(0)
	if strategy == model.TopupSaleStrategyInventory {
		if consumed < 0 {
			return nil, NewAppError(http.StatusConflict, "库存记录不一致，请先核对库存")
		}
		if request.StockTotal < consumed && request.StockTotal != previousTotal {
			return nil, BadAuthRequest(fmt.Sprintf("库存不能低于已售数量（至少 %d）", consumed))
		}
		stockRemaining = request.StockTotal - consumed
	}
	return &model.TopupProduct{
		ID: id, Name: name, Description: truncateRunes(strings.TrimSpace(request.Description), 500), Benefits: truncateRunes(strings.TrimSpace(request.Benefits), 1000),
		RibbonText: strings.TrimSpace(request.RibbonText), BadgeText: strings.TrimSpace(request.BadgeText), CompareAmountFen: request.CompareAmountFen,
		PriceCaption: strings.TrimSpace(request.PriceCaption), QuotaCaption: strings.TrimSpace(request.QuotaCaption), QuotaDetail: strings.TrimSpace(request.QuotaDetail),
		ActionText: strings.TrimSpace(request.ActionText), AccentColor: accentColor, Featured: request.Featured,
		AmountFen: request.AmountFen, CreditsMicrocredits: request.CreditsMicrocredits,
		Enabled: request.Enabled, SortOrder: request.SortOrder, CreatedBy: actorID, UpdatedBy: actorID,
		SaleStrategy: strategy, PeriodDays: request.PeriodDays, PeriodPurchaseLimit: request.PeriodPurchaseLimit,
		StockTotal: request.StockTotal, StockRemaining: stockRemaining, SaleStartAt: request.SaleStartAt, SaleEndAt: request.SaleEndAt,
	}, nil
}
