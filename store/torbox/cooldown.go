package torbox

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

// TorBox suspends an account by putting it "in cooldown". During that window its
// API keeps answering normally while its CDN silently drops the TCP connections
// that actually carry bytes, so nothing in the request path can tell that
// anything is wrong: link generation succeeds, playback stalls for the full dial
// timeout, and every retry is another request against an account that has
// already been told to stop. Continuing to ask is what extends the cooldown.
//
// So the account's own cooldown_until is honoured here: while it holds, every
// call is refused locally and none reach TorBox. The account endpoint is the one
// exemption, because reading it is how the lift is detected - and reading it does
// not extend the cooldown.

// userPath is the account endpoint, exempt from the bar.
const userPath = "/v1/api/user/me"

// How often to re-check while barred. Only runs while a cooldown is actually
// set, so a healthy account pays nothing for this.
const cooldownPollInterval = 5 * time.Minute

// Keyed by API key rather than global: one process can serve several accounts,
// and one account's suspension says nothing about another's.
var (
	cooldownMu      sync.Mutex
	cooldownUntil   = map[string]time.Time{}
	cooldownWatched = map[string]bool{}
	cooldownProbe   = map[string]*sync.Once{}
)

// ensureAccountChecked reads the account once per key per process before the
// first store call is allowed through. Without it a restart during a cooldown
// would fire one blind request per key - which is precisely the request that
// keeps the cooldown alive. The account endpoint is exempt from the bar, so
// this can never deadlock against it.
func (c APIClient) ensureAccountChecked(apiKey string) {
	cooldownMu.Lock()
	once, ok := cooldownProbe[apiKey]
	if !ok {
		once = &sync.Once{}
		cooldownProbe[apiKey] = once
	}
	cooldownMu.Unlock()

	once.Do(func() {
		if _, err := c.GetUser(&GetUserParams{Ctx: Ctx{APIKey: apiKey}}); err != nil {
			// A failed probe must not invent an outage; leave the account
			// unbarred and let the next account read settle it.
			log.Printf("[torbox] initial cooldown probe failed: %v", err)
		}
	})
}

// setAccountCooldown records (or clears) the bar for one account. Returns true
// when a cooldown is now in force.
func setAccountCooldown(apiKey string, until time.Time) bool {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	if until.After(time.Now()) {
		cooldownUntil[apiKey] = until
		return true
	}
	delete(cooldownUntil, apiKey)
	return false
}

// accountCooldownRemaining reports how long the bar still has to run, or zero.
func accountCooldownRemaining(apiKey string) time.Duration {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	until, ok := cooldownUntil[apiKey]
	if !ok {
		return 0
	}
	remaining := time.Until(until)
	if remaining <= 0 {
		delete(cooldownUntil, apiKey)
		return 0
	}
	return remaining
}

// parseCooldownUntil reads TorBox's cooldown_until. An empty or unparseable
// value means "no cooldown": a value we cannot read must never be allowed to
// invent an outage, only to confirm one.
func parseCooldownUntil(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC()
		}
	}
	return time.Time{}
}

// noteAccountCooldown is called with every account response. It arms the bar
// when TorBox reports a cooldown and releases it as soon as one is gone.
func (c APIClient) noteAccountCooldown(apiKey string, data *GetUserData) {
	if data == nil {
		return
	}
	until := parseCooldownUntil(data.CooldownUntil)
	if setAccountCooldown(apiKey, until) {
		log.Printf(
			"[torbox] account in cooldown for another %s; all torbox calls are barred until it lifts",
			time.Until(until).Round(time.Minute),
		)
		c.watchAccountCooldown(apiKey)
	}
}

// watchAccountCooldown re-checks the account until the bar lifts. Started only
// once per account, and only while barred, so it costs nothing when healthy.
func (c APIClient) watchAccountCooldown(apiKey string) {
	cooldownMu.Lock()
	if cooldownWatched[apiKey] {
		cooldownMu.Unlock()
		return
	}
	cooldownWatched[apiKey] = true
	cooldownMu.Unlock()

	go func() {
		defer func() {
			cooldownMu.Lock()
			delete(cooldownWatched, apiKey)
			cooldownMu.Unlock()
		}()
		for {
			time.Sleep(cooldownPollInterval)
			if accountCooldownRemaining(apiKey) == 0 {
				return
			}
			// The account endpoint is exempt from the bar, which is the whole
			// point: it is the only way to see that the cooldown has ended.
			if _, err := c.GetUser(&GetUserParams{Ctx: Ctx{APIKey: apiKey}}); err != nil {
				log.Printf("[torbox] cooldown re-check failed: %v", err)
				continue
			}
			if accountCooldownRemaining(apiKey) == 0 {
				log.Print("[torbox] account cooldown lifted; calls resume")
				return
			}
		}
	}()
}

// errAccountCooldown is what every barred call returns. Modelled as an upstream
// 429 so existing callers already treat it as "this store cannot serve right
// now, move on" rather than as a fault with the requested content.
func errAccountCooldown(remaining time.Duration) error {
	err := core.NewStoreError("account is in cooldown")
	err.StoreName = string(store.StoreNameTorBox)
	err.Code = core.ErrorCodeTooManyRequests
	err.StatusCode = http.StatusTooManyRequests
	err.Msg = "torbox account is in cooldown for another " + remaining.Round(time.Second).String() +
		"; refusing locally so the cooldown is not extended"
	return err
}
