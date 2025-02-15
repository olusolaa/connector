package resilience

import (
	"context"

	"github.com/connector-recruitment/internal/app/config"
	"github.com/sony/gobreaker"
)

type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

func New(name string, cfg *config.Config) *CircuitBreaker {
	settings := gobreaker.Settings{
		Name:        name,
		MaxRequests: 5,
		Interval:    cfg.CircuitBreakerInterval,
		Timeout:     cfg.CircuitBreakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	}
	return &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(settings),
	}
}

func (c *CircuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	return c.cb.Execute(fn)
}
