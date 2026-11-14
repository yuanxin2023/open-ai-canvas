package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

type ReferralMetrics struct {
	InviteeCount      int64 `json:"inviteeCount"`
	PaidInviteeCount  int64 `json:"paidInviteeCount"`
	PaymentOrderCount int64 `json:"paymentOrderCount"`
	PaymentAmountFen  int64 `json:"paymentAmountFen"`
}

type ReferralStatistics struct {
	InviterCount int64 `json:"inviterCount"`
	ReferralMetrics
}

type ReferralUserStatistics struct {
	Rank     int64  `json:"rank"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Code     string `json:"code"`
	ReferralMetrics
}

type ReferralStatisticsQuery struct {
	Keyword  string `form:"keyword"`
	SortBy   string `form:"sortBy"`
	Page     int    `form:"page,default=1" binding:"min=1,max=1000000"`
	PageSize int    `form:"pageSize,default=20" binding:"min=1,max=100"`
}

type ReferralUserStatisticsPage struct {
	Users    []ReferralUserStatistics `json:"users"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"pageSize"`
}

type ReferralInviteeStatistics struct {
	UserID            string    `json:"userId"`
	Username          string    `json:"username"`
	JoinedAt          time.Time `json:"joinedAt"`
	PaymentOrderCount int64     `json:"paymentOrderCount"`
	PaymentAmountFen  int64     `json:"paymentAmountFen"`
}

type ReferralInviteeStatisticsPage struct {
	Inviter  ReferralUserStatistics      `json:"inviter"`
	Invitees []ReferralInviteeStatistics `json:"invitees"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"pageSize"`
}

// Aggregate orders per invitee before joining invitation relationships, so a
// repeat purchase cannot multiply registrations. Both users must still exist.
const referralStatisticsCTE = `WITH payment_totals AS (
	SELECT user_id, COUNT(*) AS payment_order_count, SUM(amount_fen) AS payment_amount_fen
	FROM payment_orders
	WHERE status = ? AND currency = 'CNY' AND amount_fen > 0 AND product_kind <> ?
	GROUP BY user_id
), invitee_totals AS (
	SELECT rp.inviter_id, u.id AS user_id, u.username, u.created_at AS joined_at,
		COALESCE(p.payment_order_count, 0) AS payment_order_count,
		COALESCE(p.payment_amount_fen, 0) AS payment_amount_fen
	FROM referral_profiles rp
	JOIN users u ON u.id = rp.user_id
	JOIN users inviter ON inviter.id = rp.inviter_id
	LEFT JOIN payment_totals p ON p.user_id = u.id
	WHERE rp.inviter_id <> '' AND rp.inviter_id <> rp.user_id
), inviter_totals AS (
	SELECT inviter_id AS user_id, COUNT(*) AS invitee_count,
		SUM(CASE WHEN payment_order_count > 0 THEN 1 ELSE 0 END) AS paid_invitee_count,
		SUM(payment_order_count) AS payment_order_count, SUM(payment_amount_fen) AS payment_amount_fen
	FROM invitee_totals GROUP BY inviter_id
), user_statistics AS (
	SELECT u.id AS user_id, u.username, COALESCE(rp.code, '') AS code,
		t.invitee_count, t.paid_invitee_count, t.payment_order_count, t.payment_amount_fen
	FROM inviter_totals t JOIN users u ON u.id = t.user_id
	LEFT JOIN referral_profiles rp ON rp.user_id = u.id
) `

var referralStatisticsSortColumns = map[string]string{
	"inviteeCount":      "invitee_count",
	"paidInviteeCount":  "paid_invitee_count",
	"paymentOrderCount": "payment_order_count",
	"paymentAmountFen":  "payment_amount_fen",
}

func ValidReferralStatisticsSort(sortBy string) bool {
	_, ok := referralStatisticsSortColumns[sortBy]
	return ok
}

func referralStatisticsArgs() []any {
	return []any{model.PaymentOrderCredited, model.TopupProductKindStorage}
}

func (r *Repository) AdminReferralStatistics(ctx context.Context) (*ReferralStatistics, error) {
	var result ReferralStatistics
	err := r.db.WithContext(ctx).Raw(referralStatisticsCTE+`SELECT COUNT(*) AS inviter_count,
		COALESCE(SUM(invitee_count), 0) AS invitee_count,
		COALESCE(SUM(paid_invitee_count), 0) AS paid_invitee_count,
		COALESCE(SUM(payment_order_count), 0) AS payment_order_count,
		COALESCE(SUM(payment_amount_fen), 0) AS payment_amount_fen FROM user_statistics`, referralStatisticsArgs()...).Scan(&result).Error
	return &result, err
}

func (r *Repository) AdminReferralStatisticsUsers(ctx context.Context, q ReferralStatisticsQuery) (*ReferralUserStatisticsPage, error) {
	column, ok := referralStatisticsSortColumns[q.SortBy]
	if !ok {
		return nil, errors.New("invalid referral statistics sort")
	}
	// Rank the entire site before searching; keyword filters must not reset ranks.
	ranked := referralStatisticsCTE + `, ranked AS (SELECT *, ROW_NUMBER() OVER (ORDER BY ` + column + ` DESC, user_id ASC) AS rank FROM user_statistics) `
	filter := ""
	args := referralStatisticsArgs()
	if q.Keyword != "" {
		keyword := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(strings.ToLower(q.Keyword)) + "%"
		filter = ` WHERE LOWER(username) LIKE ? ESCAPE '\' OR LOWER(user_id) LIKE ? ESCAPE '\' OR LOWER(code) LIKE ? ESCAPE '\'`
		args = append(args, keyword, keyword, keyword)
	}
	result := &ReferralUserStatisticsPage{Users: []ReferralUserStatistics{}, Page: q.Page, PageSize: q.PageSize}
	if err := r.db.WithContext(ctx).Raw(ranked+`SELECT COUNT(*) FROM ranked`+filter, args...).Scan(&result.Total).Error; err != nil {
		return nil, err
	}
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)
	err := r.db.WithContext(ctx).Raw(ranked+`SELECT * FROM ranked`+filter+` ORDER BY rank LIMIT ? OFFSET ?`, args...).Scan(&result.Users).Error
	return result, err
}

func (r *Repository) AdminReferralInvitees(ctx context.Context, userID string, page, pageSize int) (*ReferralInviteeStatisticsPage, error) {
	result := &ReferralInviteeStatisticsPage{Invitees: []ReferralInviteeStatistics{}, Page: page, PageSize: pageSize}
	var user model.User
	if err := r.db.WithContext(ctx).Select("id", "username").First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	result.Inviter.UserID, result.Inviter.Username = user.ID, user.Username
	args := append(referralStatisticsArgs(), userID)
	if err := r.db.WithContext(ctx).Raw(referralStatisticsCTE+`SELECT u.id AS user_id, u.username, COALESCE(rp.code, '') AS code,
		COALESCE(t.invitee_count, 0) AS invitee_count, COALESCE(t.paid_invitee_count, 0) AS paid_invitee_count,
		COALESCE(t.payment_order_count, 0) AS payment_order_count, COALESCE(t.payment_amount_fen, 0) AS payment_amount_fen
		FROM users u LEFT JOIN referral_profiles rp ON rp.user_id = u.id LEFT JOIN inviter_totals t ON t.user_id = u.id
		WHERE u.id = ?`, args...).Scan(&result.Inviter).Error; err != nil {
		return nil, err
	}
	result.Total = result.Inviter.InviteeCount
	args = append(args, pageSize, (page-1)*pageSize)
	err := r.db.WithContext(ctx).Raw(referralStatisticsCTE+`SELECT user_id, username, joined_at, payment_order_count, payment_amount_fen
		FROM invitee_totals WHERE inviter_id = ? ORDER BY joined_at DESC, user_id ASC LIMIT ? OFFSET ?`, args...).Scan(&result.Invitees).Error
	return result, err
}
