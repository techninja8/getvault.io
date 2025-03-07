package datastorage

import (
	"time"

	"go.uber.org/zap"
)

// Retry executes the given function fn and retries it in case of an error.
// It uses exponential backoff for the retry intervals.
func Retry(attempts int, sleep time.Duration, logger *zap.Logger, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		logger.Warn("Operation failed, retrying...", zap.Int("attempt", i+1), zap.Error(err))
		time.Sleep(sleep)
		sleep *= 2
	}
	return err
}
