package core

import (
	"context"
	"errors"
	"time"
)

func Retry(ctx context.Context, tries int, base time.Duration, fn func() error) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// 不可重试 error 直接返回
		var re *RetryStop
		if errors.As(err, &re) {
			return err
		}
		sleep := base * (1 << i) // 1x,2x,4x…
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// RetryStop 用于 Provider 主动标记“别再重试”
type RetryStop struct{ error }

func (r *RetryStop) Unwrap() error { return r.error }
