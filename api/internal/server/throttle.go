package server

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/db/dbgen"
)

// Throttle scopes (login_throttle.scope).
const (
	scopeAccount = "account"
	scopeIP      = "ip"
)

// throttleKeys are the two counters a failed attempt is recorded against. An empty account means
// the attempt names no valid account (a malformed username counts against the IP only).
type throttleKeys struct {
	account string
	ip      string
}

// sessionThrottleKeys returns the keys of the signed-in user of a request.
func sessionThrottleKeys(a *requestAuth) throttleKeys {
	return throttleKeys{account: a.user.Username, ip: a.clientIP}
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// retryAfterSeconds is the remaining block time in whole seconds, rounded up, at least 1.
func retryAfterSeconds(until, now time.Time) int {
	return max(1, int(math.Ceil(until.Sub(now).Seconds())))
}

// tooMany builds the 429 body with its Retry-After header.
func tooMany(retryAfter int) api.TooManyRequestsApplicationProblemPlusJSONResponse {
	return api.TooManyRequestsApplicationProblemPlusJSONResponse{
		Body:    problem(429, ""),
		Headers: api.TooManyRequestsResponseHeaders{RetryAfter: &retryAfter},
	}
}

// throttleBlocked reports whether either key is blocked now, with the remaining seconds.
func (s *Server) throttleBlocked(ctx context.Context, k throttleKeys) (retryAfter int, blocked bool, err error) {
	now := s.now()
	until, err := s.q.GetThrottleBlock(ctx, dbgen.GetThrottleBlockParams{AccountKey: k.account, IpKey: k.ip, Now: timestamptz(now)})
	if err != nil {
		return 0, false, fmt.Errorf("read login throttle: %w", err)
	}
	if !until.Valid {
		return 0, false, nil
	}
	return retryAfterSeconds(until.Time, now), true, nil
}

// throttleFailure records one failure against both keys. When a counter reaches the threshold it
// blocks that key; blocked is true and retryAfter is the longest block set by this failure.
func (s *Server) throttleFailure(ctx context.Context, k throttleKeys) (retryAfter int, blocked bool, err error) {
	cfg := s.deps.Config
	now := s.now()
	targets := []struct{ scope, key string }{{scopeIP, k.ip}}
	if k.account != "" {
		targets = append(targets, struct{ scope, key string }{scopeAccount, k.account})
	}
	for _, t := range targets {
		failures, err := s.q.RecordThrottleFailure(ctx, dbgen.RecordThrottleFailureParams{
			Scope:       t.scope,
			Key:         t.key,
			Now:         timestamptz(now),
			DecayBefore: timestamptz(now.Add(-cfg.ThrottleMaxDelay)),
		})
		if err != nil {
			return 0, false, fmt.Errorf("record login failure: %w", err)
		}
		if int(failures) < cfg.ThrottleFailures {
			continue
		}
		delay := blockDelay(int(failures)-cfg.ThrottleFailures, cfg.ThrottleBaseDelay, cfg.ThrottleMaxDelay)
		err = s.q.SetThrottleBlock(ctx, dbgen.SetThrottleBlockParams{BlockedUntil: timestamptz(now.Add(delay)), Scope: t.scope, Key: t.key})
		if err != nil {
			return 0, false, fmt.Errorf("set login block: %w", err)
		}
		blocked = true
		retryAfter = max(retryAfter, retryAfterSeconds(now.Add(delay), now))
	}
	return retryAfter, blocked, nil
}

// blockDelay is base * 2^excess, capped at maxDelay. It doubles in a loop that stops at the cap,
// so the product can never overflow.
func blockDelay(excess int, base, maxDelay time.Duration) time.Duration {
	d := base
	for range excess {
		if d >= maxDelay {
			break
		}
		d *= 2
	}
	return min(d, maxDelay)
}

// throttleClearAccount forgets the failures of one account after a full sign-in. The IP row stays,
// so one valid account cannot reset an attacker's counter.
func (s *Server) throttleClearAccount(ctx context.Context, account string) error {
	if err := s.q.DeleteAccountThrottle(ctx, account); err != nil {
		return fmt.Errorf("clear login throttle: %w", err)
	}
	return nil
}
