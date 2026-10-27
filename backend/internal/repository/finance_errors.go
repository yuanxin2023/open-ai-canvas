package repository

import "errors"

var (
	ErrInsufficientCredits         = errors.New("积分不足，请先充值")
	ErrRedeemCodeInvalid           = errors.New("兑换码无效或已使用")
	ErrActiveTaskLimit             = errors.New("同时进行的任务数已达上限，请等待已有任务完成")
	ErrTaskNotRetryable            = errors.New("该任务当前状态不支持重试")
	ErrBillingStateConflict        = errors.New("计费状态已变化，请刷新后重试")
	ErrBillingUsageUnavailable     = errors.New("暂时无法获取用量，计费未完成")
	ErrBillingChargeLimit          = errors.New("本次费用超过授权上限，已拒绝扣费")
	ErrMeasuredDurationUnavailable = errors.New("实测时长不可用，需人工核对")
	ErrChannelModelInUse           = errors.New("该渠道模型正在被使用，无法删除或停用")
)
