package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// enqueueScheduled enqueues every schedule that's due at now and moves it to
// its next interval. Start calls it once at boot; runScheduler calls it on
// each tick. The kind is the unique_key, so a pending run always wins.
func (q *Queue) enqueueScheduled(ctx context.Context, now time.Time) error {
	var errs []error
	for i := range q.schedules {
		s := &q.schedules[i]
		if now.Before(s.next) {
			continue
		}
		err := q.Enqueue(ctx, s.kind, nil, EnqueueOptions{UniqueKey: s.kind})
		if err != nil {
			// Leave next where it is, so the next tick tries again.
			errs = append(errs, fmt.Errorf("enqueue scheduled %s job: %w", s.kind, err))
			continue
		}
		s.next = now.Add(s.interval)
	}
	return errors.Join(errs...)
}

// runScheduler checks the schedules every schedulerTick (or on the injected
// tick channel in tests) until ctx is cancelled. It's interval-based, not
// cron: a scheduled kind runs at startup and then every interval.
func (q *Queue) runScheduler(ctx context.Context) {
	defer q.wg.Done()
	if len(q.schedules) == 0 {
		return
	}
	tick := q.tick
	if tick == nil {
		t := time.NewTicker(schedulerTick)
		defer t.Stop()
		tick = t.C
	}
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-tick:
			if !ok {
				return
			}
			if err := q.enqueueScheduled(ctx, q.now()); err != nil {
				q.logger.Error("scheduler", "err", err)
			}
		}
	}
}
