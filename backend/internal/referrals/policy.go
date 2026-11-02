package referrals

import (
	"errors"
	"math/big"
	"time"
)

const Scale int64 = 1_000_000
const SettingKey = "referral_policy"

// Until is an exclusive day boundary, or an inclusive order boundary; zero is unlimited.
type Stage struct {
	Until   int64 `json:"until"`
	RateBPS int64 `json:"rateBps"`
}

type Policy struct {
	VersionID                 string  `json:"versionId"`
	Enabled                   bool    `json:"enabled"`
	Mode                      string  `json:"mode"`
	Stages                    []Stage `json:"stages"`
	MaxDays                   int64   `json:"maxDays"`
	MaxOrders                 int64   `json:"maxOrders"`
	PerInviteeCapMicrocredits int64   `json:"perInviteeCapMicrocredits"`
	NewcomerEnabled           bool    `json:"newcomerEnabled"`
	NewcomerRateBPS           int64   `json:"newcomerRateBps"`
	NewcomerCapMicrocredits   int64   `json:"newcomerCapMicrocredits"`
}

func Default() Policy {
	return Policy{Mode: "orders", Stages: []Stage{{Until: 3, RateBPS: 2000}, {RateBPS: 500}}, PerInviteeCapMicrocredits: 300 * Scale, NewcomerRateBPS: 500}
}

func (p Policy) Validate() error {
	if p.Mode != "time" && p.Mode != "orders" {
		return errors.New("请选择时间或单数阶梯")
	}
	if len(p.Stages) == 0 || len(p.Stages) > 20 {
		return errors.New("阶梯数量必须为 1 至 20")
	}
	previous := int64(0)
	for i, s := range p.Stages {
		if s.RateBPS < 0 || s.RateBPS > 10000 || s.Until < 0 || s.Until > 1_000_000 || (p.Mode == "time" && s.Until > 36500) || (s.Until == 0 && i != len(p.Stages)-1) || (s.Until != 0 && s.Until <= previous) {
			return errors.New("阶段结束值必须递增，只有最后阶段可不限；比例必须为 0% 至 100%")
		}
		previous = s.Until
	}
	if p.MaxDays < 0 || p.MaxDays > 36500 || p.MaxOrders < 0 || p.MaxOrders > 1_000_000 || p.PerInviteeCapMicrocredits < 0 || p.PerInviteeCapMicrocredits > 1_000_000_000*Scale || p.NewcomerCapMicrocredits < 0 || p.NewcomerCapMicrocredits > 1_000_000_000*Scale || p.NewcomerRateBPS < 0 || p.NewcomerRateBPS > 10000 || p.PerInviteeCapMicrocredits%10000 != 0 || p.NewcomerCapMicrocredits%10000 != 0 {
		return errors.New("奖励限制超出范围，积分最多保留两位小数")
	}
	return nil
}

func Credits(base, rate int64) (int64, error) {
	if base < 0 || rate < 0 || rate > 10000 {
		return 0, errors.New("奖励计算参数无效")
	}
	v := new(big.Int).Mul(big.NewInt(base), big.NewInt(rate))
	v.Quo(v, big.NewInt(10000))
	if !v.IsInt64() {
		return 0, errors.New("奖励积分溢出")
	}
	return v.Int64() / 10000 * 10000, nil
}

type Result struct {
	Stage    int
	RateBPS  int64
	Reward   int64
	Uncapped int64
	Newcomer int64
}

func Evaluate(p Policy, base, ordinal, used int64, first, now time.Time) (Result, error) {
	if err := p.Validate(); err != nil {
		return Result{}, err
	}
	r := Result{Stage: -1}
	if !p.Enabled || ordinal <= 0 {
		return r, nil
	}
	if ordinal == 1 && p.NewcomerEnabled {
		bonus, err := Credits(base, p.NewcomerRateBPS)
		if err != nil {
			return r, err
		}
		if p.NewcomerCapMicrocredits > 0 && bonus > p.NewcomerCapMicrocredits {
			bonus = p.NewcomerCapMicrocredits
		}
		r.Newcomer = bonus
	}
	if now.Before(first) || (p.MaxDays > 0 && !now.Before(first.Add(time.Duration(p.MaxDays)*24*time.Hour))) || (p.MaxOrders > 0 && ordinal > p.MaxOrders) {
		return r, nil
	}
	for i, s := range p.Stages {
		matches := s.Until == 0 || (p.Mode == "orders" && ordinal <= s.Until) || (p.Mode == "time" && now.Before(first.Add(time.Duration(s.Until)*24*time.Hour)))
		if !matches {
			continue
		}
		r.Stage, r.RateBPS = i, s.RateBPS
		amount, err := Credits(base, s.RateBPS)
		if err != nil {
			return r, err
		}
		r.Uncapped = amount
		if p.PerInviteeCapMicrocredits > 0 && amount > p.PerInviteeCapMicrocredits-used {
			amount = p.PerInviteeCapMicrocredits - used
		}
		if amount > 0 {
			r.Reward = amount / 10000 * 10000
		}
		return r, nil
	}
	return r, nil
}
