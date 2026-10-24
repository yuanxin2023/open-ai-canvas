package repository

import (
	"database/sql"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

type ControlPreviewFilter struct {
	From, To                           time.Time
	UserID, Model, Capability, Keyword string
	Page, PageSize                     int
}

type ControlPreviewTotals struct {
	AvailableMicrocredits        int64 `json:"availableMicrocredits"`
	ReservedMicrocredits         int64 `json:"reservedMicrocredits"`
	ConsumedMicrocredits         int64 `json:"consumedMicrocredits"`
	LifetimeConsumedMicrocredits int64 `json:"lifetimeConsumedMicrocredits"`
	InflowMicrocredits           int64 `json:"inflowMicrocredits"`
	PaidAmountFen                int64 `json:"paidAmountFen"`
	PayingUsers                  int64 `json:"payingUsers"`
	PaidOrders                   int64 `json:"paidOrders"`
	ConsumingUsers               int64 `json:"consumingUsers"`
	Generations                  int64 `json:"generations"`
	Succeeded                    int64 `json:"succeeded"`
	Failed                       int64 `json:"failed"`
	Pending                      int64 `json:"pending"`
	UnlinkedGenerations          int64 `json:"unlinkedGenerations"`
}

type ControlPreviewModel struct {
	Model                string `json:"model"`
	Capability           string `json:"capability"`
	Generations          int64  `json:"generations"`
	Succeeded            int64  `json:"succeeded"`
	Failed               int64  `json:"failed"`
	Pending              int64  `json:"pending"`
	ConsumedMicrocredits int64  `json:"consumedMicrocredits"`
}

type ControlPreviewUser struct {
	Rank                 int64   `json:"rank"`
	UserID               string  `json:"userId"`
	Username             string  `json:"username"`
	ConsumedMicrocredits int64   `json:"consumedMicrocredits"`
	Generations          int64   `json:"generations"`
	Share                float64 `json:"share"`
}

type ControlPreviewInflow struct {
	Source             string `json:"source"`
	AmountMicrocredits int64  `json:"amountMicrocredits"`
}

type ControlPreviewBucket struct {
	Bucket               string `json:"bucket"`
	InflowMicrocredits   int64  `json:"inflowMicrocredits"`
	ConsumedMicrocredits int64  `json:"consumedMicrocredits"`
}

type ControlPreviewLedger struct {
	ID                   string    `json:"id"`
	Model                string    `json:"model"`
	Capability           string    `json:"capability"`
	BillingOrderID       string    `json:"billingOrderId"`
	ConsumedMicrocredits int64     `json:"consumedMicrocredits"`
	CreatedAt            time.Time `json:"createdAt"`
}

type ControlPreviewSnapshot struct {
	Totals        ControlPreviewTotals
	Ranking       []ControlPreviewUser
	Models        []ControlPreviewModel
	InflowSources []ControlPreviewInflow
	Buckets       []ControlPreviewBucket
}

// Deduplication happens before the period/model filter: a retry after midnight
// belongs to the original generation, and a successful fallback owns the model.
const controlPreviewCTE = `WITH ordinary AS (
 SELECT id, username FROM users WHERE role = 'user'
), spending AS (
 SELECT l.id, l.user_id, COALESCE(NULLIF(l.model, ''), NULLIF(b.model, ''), '未识别') AS model,
 b.capability, -l.amount_microcredits AS consumed_microcredits, l.created_at, l.billing_order_id
 FROM credit_ledger_entries l JOIN ordinary u ON u.id = l.user_id
 JOIN billing_orders b ON b.id = l.billing_order_id AND b.user_id = l.user_id
 WHERE l.type = 'consume' AND l.amount_microcredits < 0
 AND __ledger_at__ >= __from__ AND __ledger_at__ < __to__
 AND (@user = '' OR l.user_id = @user)
), inflows AS (
 SELECT l.user_id, l.created_at, l.amount_microcredits,
 CASE WHEN l.type = 'redeem' THEN 'redeem' WHEN l.type = 'payment_topup' THEN 'payment'
 WHEN l.type IN ('admin_grant', 'admin_adjustment') THEN 'admin' ELSE 'reward' END AS source
 FROM credit_ledger_entries l JOIN ordinary u ON u.id = l.user_id
 WHERE l.type IN ('redeem', 'payment_topup', 'admin_grant', 'admin_adjustment', 'signup_bonus', 'checkin_bonus', 'referral')
 AND l.amount_microcredits > 0 AND __ledger_at__ >= __from__ AND __ledger_at__ < __to__
 AND (@user = '' OR l.user_id = @user)
), roots AS (
 SELECT l.id, l.user_id, l.model, l.capability, l.status, __log_at__ AS event_at,
 l.provider_status, l.provider_request_id, t.status AS task_state, b.status AS billing_state,
 CASE WHEN COALESCE(l.billing_order_id, '') <> '' THEN 'billing:' || l.billing_order_id
 WHEN COALESCE(l.task_id, '') <> '' THEN 'task:' || l.task_id
 WHEN COALESCE(l.request_id, '') <> '' THEN 'request:' || l.request_id ELSE 'log:' || l.id END AS generation_key
 FROM api_call_logs l JOIN ordinary u ON u.id = l.user_id
 LEFT JOIN tasks t ON t.id = l.task_id AND t.user_id = l.user_id
 LEFT JOIN billing_orders b ON b.id = l.billing_order_id AND b.user_id = l.user_id
 WHERE __log_at__ < __to__ AND l.method <> 'INTERNAL'
 AND l.capability IN ('text', 'image', 'video', 'audio')
 AND (l.request_kind IN ('create', 'repair') OR (COALESCE(l.request_kind, '') = '' AND l.billable = TRUE))
 AND COALESCE(l.error_code, '') <> 'request_not_sent'
 AND (@user = '' OR l.user_id = @user)
), numbered AS (
 SELECT roots.*, MIN(event_at) OVER (PARTITION BY user_id, generation_key) AS first_at,
 ROW_NUMBER() OVER (PARTITION BY user_id, generation_key ORDER BY
 CASE WHEN status = 'succeeded' THEN 0 ELSE 1 END, event_at DESC, id DESC) AS choice
 FROM roots
), generations AS (
 SELECT user_id, COALESCE(NULLIF(model, ''), '未识别') AS model, capability,
 CASE WHEN generation_key LIKE 'log:%' THEN 1 ELSE 0 END AS unlinked,
 CASE WHEN task_state = 'succeeded' THEN 'succeeded'
 WHEN task_state IN ('failed', 'cancelled') THEN 'failed'
 WHEN task_state IS NOT NULL THEN 'pending'
 WHEN billing_state = 'settled' THEN 'succeeded'
 WHEN billing_state = 'refunded' THEN 'failed'
 WHEN billing_state IN ('reserved', 'running', 'uncertain') THEN 'pending'
 WHEN provider_status IN ('failed', 'error', 'cancelled', 'canceled') THEN 'failed'
 WHEN provider_status IN ('succeeded', 'success', 'completed') THEN 'succeeded'
 WHEN provider_status IN ('queued', 'pending', 'running', 'processing', 'accepted', 'in_progress') THEN 'pending'
 WHEN status = 'failed' THEN 'failed'
 WHEN capability IN ('video', 'audio') AND COALESCE(provider_request_id, '') <> '' THEN 'pending'
 ELSE 'succeeded' END AS outcome
 FROM numbered WHERE choice = 1 AND first_at >= __from__
), model_values AS (
 SELECT user_id, model, capability, consumed_microcredits, 0 AS generations,
 0 AS succeeded, 0 AS failed, 0 AS pending FROM spending
 UNION ALL
 SELECT user_id, model, capability, 0, 1,
 CASE WHEN outcome = 'succeeded' THEN 1 ELSE 0 END,
 CASE WHEN outcome = 'failed' THEN 1 ELSE 0 END,
 CASE WHEN outcome = 'pending' THEN 1 ELSE 0 END FROM generations
), filtered_values AS (
 SELECT * FROM model_values WHERE (@model = '' OR model = @model) AND (@capability = '' OR capability = @capability)
) `

func (r *Repository) controlPreviewSQL(tail string) string {
	clock := func(column string) string {
		// SQLite stores offsets in text; lexical comparison breaks mixed UTC/local
		// timestamps. Normalize both comparisons and retry ordering to one clock.
		if r.Dialect() == "sqlite" {
			return "julianday(" + column + ")"
		}
		return column
	}
	return strings.NewReplacer(
		"__ledger_at__", clock("l.created_at"), "__log_at__", clock("l.created_at"),
		"__paid_at__", clock("p.credited_at"), "__entry_at__", clock("created_at"),
		"__from__", clock("@from"), "__to__", clock("@to"),
	).Replace(controlPreviewCTE + tail)
}

func previewArgs(f ControlPreviewFilter) map[string]any {
	return map[string]any{"from": f.From.UTC(), "to": f.To.UTC(), "user": f.UserID, "model": f.Model, "capability": f.Capability, "keyword": "%" + f.Keyword + "%"}
}

func (r *Repository) ControlPreviewSnapshot(f ControlPreviewFilter, hourly bool) (*ControlPreviewSnapshot, error) {
	result := &ControlPreviewSnapshot{Ranking: []ControlPreviewUser{}, Models: []ControlPreviewModel{}, InflowSources: []ControlPreviewInflow{}, Buckets: []ControlPreviewBucket{}}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		reader := &Repository{db: tx}
		if err := tx.Raw(reader.controlPreviewSQL(`SELECT
 (SELECT COALESCE(SUM(a.available_microcredits), 0) FROM credit_accounts a JOIN ordinary u ON u.id = a.user_id) AS available_microcredits,
 (SELECT COALESCE(SUM(a.reserved_microcredits), 0) FROM credit_accounts a JOIN ordinary u ON u.id = a.user_id) AS reserved_microcredits,
 (SELECT COALESCE(SUM(consumed_microcredits), 0) FROM spending) AS consumed_microcredits,
 (SELECT COUNT(DISTINCT user_id) FROM spending) AS consuming_users,
 (SELECT COALESCE(SUM(amount_microcredits), 0) FROM inflows) AS inflow_microcredits,
 (SELECT COALESCE(SUM(p.amount_fen), 0) FROM payment_orders p JOIN ordinary u ON u.id = p.user_id
 WHERE p.status = 'credited' AND p.currency = 'CNY' AND __paid_at__ >= __from__ AND __paid_at__ < __to__) AS paid_amount_fen,
 (SELECT COUNT(*) FROM payment_orders p JOIN ordinary u ON u.id = p.user_id
 WHERE p.status = 'credited' AND p.currency = 'CNY' AND __paid_at__ >= __from__ AND __paid_at__ < __to__) AS paid_orders,
 (SELECT COUNT(DISTINCT p.user_id) FROM payment_orders p JOIN ordinary u ON u.id = p.user_id
 WHERE p.status = 'credited' AND p.currency = 'CNY' AND __paid_at__ >= __from__ AND __paid_at__ < __to__) AS paying_users,
 (SELECT COALESCE(-SUM(l.amount_microcredits), 0) FROM credit_ledger_entries l JOIN ordinary u ON u.id = l.user_id
 JOIN billing_orders b ON b.id = l.billing_order_id AND b.user_id = l.user_id
 WHERE l.type = 'consume' AND l.amount_microcredits < 0 AND __ledger_at__ < __to__) AS lifetime_consumed_microcredits,
 COUNT(*) AS generations,
 COALESCE(SUM(CASE WHEN outcome = 'succeeded' THEN 1 ELSE 0 END), 0) AS succeeded,
 COALESCE(SUM(CASE WHEN outcome = 'failed' THEN 1 ELSE 0 END), 0) AS failed,
 COALESCE(SUM(CASE WHEN outcome = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
 COALESCE(SUM(unlinked), 0) AS unlinked_generations FROM generations`), previewArgs(f)).Scan(&result.Totals).Error; err != nil {
			return err
		}
		var err error
		result.Models, err = reader.ControlPreviewModels(f)
		if err != nil {
			return err
		}
		f.Page, f.PageSize = 1, 20
		result.Ranking, _, err = reader.ControlPreviewUsers(f, false)
		if err != nil {
			return err
		}
		if err := tx.Raw(reader.controlPreviewSQL(`SELECT source, SUM(amount_microcredits) AS amount_microcredits FROM inflows GROUP BY source`), previewArgs(f)).Scan(&result.InflowSources).Error; err != nil {
			return err
		}
		bucket := previewBucketExpression(r.Dialect(), hourly)
		return tx.Raw(reader.controlPreviewSQL(`SELECT `+bucket+` AS bucket, SUM(inflow_microcredits) AS inflow_microcredits, SUM(consumed_microcredits) AS consumed_microcredits FROM (
 SELECT created_at, amount_microcredits AS inflow_microcredits, 0 AS consumed_microcredits FROM inflows
 UNION ALL SELECT created_at, 0, consumed_microcredits FROM spending
 ) events GROUP BY `+bucket+` ORDER BY bucket`), previewArgs(f)).Scan(&result.Buckets).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func previewBucketExpression(dialect string, hourly bool) string {
	if dialect == "postgres" {
		format := "YYYY-MM-DD"
		if hourly {
			format = "YYYY-MM-DD HH24:00"
		}
		return "to_char(created_at AT TIME ZONE 'Asia/Shanghai', '" + format + "')"
	}
	format := "%Y-%m-%d"
	if hourly {
		format = "%Y-%m-%d %H:00"
	}
	return "strftime('" + format + "', created_at, '+8 hours')"
}

func (r *Repository) ControlPreviewModels(f ControlPreviewFilter) ([]ControlPreviewModel, error) {
	rows := []ControlPreviewModel{}
	err := r.db.Raw(r.controlPreviewSQL(`SELECT model, capability, SUM(generations) AS generations,
 SUM(succeeded) AS succeeded, SUM(failed) AS failed, SUM(pending) AS pending,
 SUM(consumed_microcredits) AS consumed_microcredits FROM filtered_values
 GROUP BY model, capability ORDER BY generations DESC, model, capability`), previewArgs(f)).Scan(&rows).Error
	return rows, err
}

func (r *Repository) ControlPreviewUsers(f ControlPreviewFilter, modelOnly bool) ([]ControlPreviewUser, int64, error) {
	rows := []ControlPreviewUser{}
	having := "SUM(v.consumed_microcredits) > 0"
	if modelOnly {
		having = "SUM(v.consumed_microcredits) > 0 OR SUM(v.generations) > 0"
	}
	query := r.controlPreviewSQL(`, ranked AS (
 SELECT ROW_NUMBER() OVER (ORDER BY SUM(v.consumed_microcredits) DESC, v.user_id) AS rank,
 v.user_id, u.username, SUM(v.consumed_microcredits) AS consumed_microcredits, SUM(v.generations) AS generations
 FROM filtered_values v JOIN ordinary u ON u.id = v.user_id
 GROUP BY v.user_id, u.username HAVING ` + having + `
 ) `)
	args := previewArgs(f)
	var total int64
	if err := r.db.Raw(query+`SELECT COUNT(*) FROM ranked WHERE LOWER(username) LIKE LOWER(@keyword) OR LOWER(user_id) LIKE LOWER(@keyword)`, args).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	args["limit"], args["offset"] = f.PageSize, (f.Page-1)*f.PageSize
	err := r.db.Raw(query+`SELECT *, CASE WHEN (SELECT COALESCE(SUM(consumed_microcredits), 0) FROM filtered_values) > 0 THEN
 consumed_microcredits * 1.0 / (SELECT SUM(consumed_microcredits) FROM filtered_values) ELSE 0 END AS share
 FROM ranked WHERE LOWER(username) LIKE LOWER(@keyword) OR LOWER(user_id) LIKE LOWER(@keyword)
 ORDER BY rank LIMIT @limit OFFSET @offset`, args).Scan(&rows).Error
	return rows, total, err
}

func (r *Repository) ControlPreviewUser(f ControlPreviewFilter) (*ControlPreviewUser, error) {
	var row ControlPreviewUser
	if err := r.db.Model(&model.User{}).Select("id AS user_id, username").Where("id = ? AND role = ?", f.UserID, model.UserRoleUser).Take(&row).Error; err != nil {
		return nil, err
	}
	var totals ControlPreviewUser
	err := r.db.Raw(r.controlPreviewSQL(`SELECT
 (SELECT COALESCE(SUM(consumed_microcredits), 0) FROM spending) AS consumed_microcredits,
 (SELECT COUNT(*) FROM generations) AS generations`), previewArgs(f)).Scan(&totals).Error
	totals.UserID, totals.Username = row.UserID, row.Username
	return &totals, err
}

func (r *Repository) ControlPreviewLedger(f ControlPreviewFilter) ([]ControlPreviewLedger, int64, error) {
	rows := []ControlPreviewLedger{}
	var total int64
	args := previewArgs(f)
	if err := r.db.Raw(r.controlPreviewSQL(`SELECT COUNT(*) FROM spending`), args).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	args["limit"], args["offset"] = f.PageSize, (f.Page-1)*f.PageSize
	err := r.db.Raw(r.controlPreviewSQL(`SELECT id, model, capability, billing_order_id, consumed_microcredits, created_at FROM spending
 ORDER BY __entry_at__ DESC, id DESC LIMIT @limit OFFSET @offset`), args).Scan(&rows).Error
	return rows, total, err
}
