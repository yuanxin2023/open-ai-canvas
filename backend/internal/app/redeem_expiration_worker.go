package app

import (
	"context"
	"log"
	"time"
)

const redeemCodeSecretRetention = 30 * 24 * time.Hour

func (s *Service) startRedeemExpirationWorker(ctx context.Context) {
	s.runWorkerLoop(func(ctx context.Context) {
		s.settleExpiredRedeemCodes()
		s.clearExpiredRedeemCodeSecrets()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.settleExpiredRedeemCodes()
				s.clearExpiredRedeemCodeSecrets()
			}
		}
	})
}

func (s *Service) clearExpiredRedeemCodeSecrets() {
	now := time.Now()
	var clearedTotal int64
	for range 10 {
		cleared, err := s.repo.ClearTerminalRedeemBatchSecrets(now.Add(-redeemCodeSecretRetention), now, 100)
		if err != nil {
			log.Printf("redeem code secret cleanup failed: %v", err)
			return
		}
		clearedTotal += cleared
		if cleared == 0 {
			break
		}
	}
	if clearedTotal > 0 {
		log.Printf("redeem code secret cleanup: cleared_batches=%d retention_days=30", clearedTotal)
	}
}

func (s *Service) settleExpiredRedeemCodes() {
	for range 10 {
		expired, refunded, err := s.repo.SettleExpiredRedeemCodes(time.Now(), "", 100)
		if err != nil {
			log.Printf("redeem code expiration settlement failed: %v", err)
			return
		}
		if expired > 0 {
			log.Printf("redeem code expiration settlement: expired=%d refunded_microcredits=%d", expired, refunded)
		}
		if expired == 0 {
			return
		}
	}
}
