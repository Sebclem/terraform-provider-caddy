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
	for attempt := 0; ; attempt++ {
		snap, err := c.Get(ctx, scope)
		if err != nil {
			if afterWrite {
				return fmt.Errorf("cannot reconcile uncertain write; retain resource ID for recovery: %w", err)
			}
			return err
		}
		snap.AfterWrite = afterWrite
		decision, err := f(snap)
		if err != nil {
			return err
		}
		if decision.Done {
			return nil
		}
		if snap.ETag == "" {
			return errors.New("Caddy did not return ETag; conditional writes are required")
		}
		if attempt > c.options.ConflictRetries {
			return errors.New("Caddy mutation did not converge within retry limit")
		}
		err = c.Write(ctx, decision.Method, decision.Path, decision.Body, snap.ETag)
		var he *HTTPError
		if err == nil {
			afterWrite = true
			continue
		}
		if errors.As(err, &he) && he.Status != 412 {
			return err
		}
		afterWrite = !IsStatus(err, 412)
		if attempt >= c.options.ConflictRetries {
			return fmt.Errorf("Caddy mutation retry limit reached: %w", err)
		}
		timer := time.NewTimer(time.Duration(5+rand.IntN(15)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
