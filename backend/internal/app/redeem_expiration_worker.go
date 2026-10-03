package app

import (
	"context"
	"log"
	"time"
)

func (s *Service) startRedeemExpirationWorker(ctx context.Context) {
	s.runWorkerLoop(func(ctx context.Context) {
		s.settleExpiredRedeemCodes()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.settleExpiredRedeemCodes()
			}
		}
	})
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
