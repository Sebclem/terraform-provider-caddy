package caddy

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

type Decision struct {
	Method string
	Path   Path
	Body   Value
	Done   bool
}
type Reconcile func(Snapshot) (Decision, error)

var endpointLocks sync.Map
var ErrUncertainWrite = errors.New("Caddy write outcome requires recovery")

func (c *Client) Mutate(ctx context.Context, scope Path, f Reconcile) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	key := strings.ToLower(c.endpoint.Scheme + "://" + c.endpoint.Host)
	v, _ := endpointLocks.LoadOrStore(key, make(chan struct{}, 1))
	lock := v.(chan struct{})
	select {
	case lock <- struct{}{}:
		defer func() { <-lock }()
	case <-ctx.Done():
		return ctx.Err()
	}
	afterWrite := false
	verifiedWrite := false
	var lastErr error
	for attempt := 0; ; attempt++ {
		snap, err := c.Get(ctx, scope)
		if err != nil {
			if afterWrite {
				return fmt.Errorf("%w: readback failed: %w", ErrUncertainWrite, err)
			}
			return err
		}
		snap.AfterWrite = afterWrite
		decision, err := f(snap)
		if err != nil {
			if afterWrite {
				return fmt.Errorf("%w: %w", ErrUncertainWrite, err)
			}
			return err
		}
		if decision.Done {
			return nil
		}
		if snap.ETag == "" {
			return errors.New("Caddy did not return ETag; conditional writes are required")
		}
		if verifiedWrite {
			return fmt.Errorf("%w: Caddy readback differs from the requested configuration", ErrUncertainWrite)
		}
		if attempt > c.options.ConflictRetries {
			return fmt.Errorf("Caddy mutation retry limit reached after reconciliation: %w", lastErr)
		}
		err = c.Write(ctx, decision.Method, decision.Path, decision.Body, snap.ETag)
		var he *HTTPError
		if err == nil {
			afterWrite = true
			verifiedWrite = true
			continue
		}
		if errors.As(err, &he) && he.Status != 412 && he.Status < 500 {
			return err
		}
		lastErr = err
		afterWrite = !IsStatus(err, 412)
		if attempt >= c.options.ConflictRetries && !afterWrite {
			return fmt.Errorf("Caddy mutation retry limit reached: %w", err)
		}
		timer := time.NewTimer(time.Duration(5+rand.IntN(15)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			if afterWrite {
				return fmt.Errorf("%w: %w", ErrUncertainWrite, ctx.Err())
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
