package worker

import (
	"context"
	"log"
	"time"

	"github.com/stayhub/backend/internal/domain/interfaces"
	"github.com/stayhub/backend/internal/platform/clock"
	"github.com/stayhub/backend/internal/platform/redis"
)

const (
	DefaultPollInterval = 5 * time.Second
	DefaultSweepLimit   = 100
)

type TickResult struct {
	DueRedisTimers  int `json:"due_redis_timers"`
	ExpiredPending  int `json:"expired_pending"`
	CheckedIn       int `json:"checked_in"`
	CheckedOut      int `json:"checked_out"`
	PayoutsReleased int `json:"payouts_released"`
}

type Scheduler struct {
	reservationService interfaces.ReservationService
	rdb                *redis.Client
	clock              clock.Clock
	pollInterval       time.Duration
	sweepLimit         int
}

func NewScheduler(
	reservationService interfaces.ReservationService,
	rdb *redis.Client,
	clk clock.Clock,
	pollInterval time.Duration,
	sweepLimit int,
) *Scheduler {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	if sweepLimit <= 0 {
		sweepLimit = DefaultSweepLimit
	}
	return &Scheduler{
		reservationService: reservationService,
		rdb:                rdb,
		clock:              clk,
		pollInterval:       pollInterval,
		sweepLimit:         sweepLimit,
	}
}

// Run executes the background timer & reconciliation loop until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	log.Printf("StayHub background worker started (interval=%s, batch=%d)", s.pollInterval, s.sweepLimit)

	if _, err := s.Tick(ctx); err != nil {
		log.Printf("worker initial tick warning: %v", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("StayHub background worker stopping...")
			return nil
		case <-ticker.C:
			res, err := s.Tick(ctx)
			if err != nil {
				log.Printf("worker tick error: %v", err)
				continue
			}
			if res.ExpiredPending > 0 || res.CheckedIn > 0 || res.CheckedOut > 0 || res.PayoutsReleased > 0 {
				log.Printf(
					"worker tick processed: expired=%d checked_in=%d checked_out=%d payouts_released=%d (redis_due=%d)",
					res.ExpiredPending,
					res.CheckedIn,
					res.CheckedOut,
					res.PayoutsReleased,
					res.DueRedisTimers,
				)
			}
		}
	}
}

// Tick runs a single reconciliation pass across Redis timers and PostgreSQL due states.
func (s *Scheduler) Tick(ctx context.Context) (TickResult, error) {
	var result TickResult
	now := s.clock.Now()

	var dueMembers []string
	if s.rdb != nil {
		members, err := s.rdb.FetchDueTimers(ctx, now, int64(s.sweepLimit))
		if err == nil {
			dueMembers = members
			result.DueRedisTimers = len(members)
		}
	}

	expiredCount, err := s.reservationService.ExpireDuePendingReservations(ctx, s.sweepLimit)
	if err != nil {
		return result, err
	}
	result.ExpiredPending = expiredCount

	checkedInCount, err := s.reservationService.ProcessDueCheckIns(ctx, s.sweepLimit)
	if err != nil {
		return result, err
	}
	result.CheckedIn = checkedInCount

	checkedOutCount, err := s.reservationService.ProcessDueCheckOuts(ctx, s.sweepLimit)
	if err != nil {
		return result, err
	}
	result.CheckedOut = checkedOutCount

	payoutsCount, err := s.reservationService.ReleaseDuePayouts(ctx, s.sweepLimit)
	if err != nil {
		return result, err
	}
	result.PayoutsReleased = payoutsCount

	// Clean up any due Redis timer entries that were already handled or cancelled in Postgres
	if s.rdb != nil && len(dueMembers) > 0 {
		for _, member := range dueMembers {
			_ = s.rdb.RemoveTimer(ctx, member)
		}
	}

	return result, nil
}
