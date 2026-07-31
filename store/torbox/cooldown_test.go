package torbox

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func reset() {
	cooldownMu.Lock()
	defer cooldownMu.Unlock()
	cooldownUntil = map[string]time.Time{}
	cooldownWatched = map[string]bool{}
	cooldownProbe = map[string]*sync.Once{}
}

func TestAFutureCooldownBarsTheAccount(t *testing.T) {
	reset()
	if !setAccountCooldown("k", time.Now().Add(time.Hour)) {
		t.Fatal("a future cooldown must bar the account")
	}
	if d := accountCooldownRemaining("k"); d <= 59*time.Minute {
		t.Fatalf("expected ~1h remaining, got %s", d)
	}
}

func TestAPastCooldownBarsNothing(t *testing.T) {
	reset()
	if setAccountCooldown("k", time.Now().Add(-time.Hour)) {
		t.Fatal("an expired cooldown must not bar the account")
	}
	if d := accountCooldownRemaining("k"); d != 0 {
		t.Fatalf("expected no bar, got %s", d)
	}
}

func TestTheBarIsPerAccount(t *testing.T) {
	reset()
	setAccountCooldown("a", time.Now().Add(time.Hour))
	if accountCooldownRemaining("b") != 0 {
		t.Fatal("one account's cooldown must not bar another")
	}
}

func TestAnUnreadableCooldownNeverInventsAnOutage(t *testing.T) {
	reset()
	for _, raw := range []string{"", "not-a-date", "tomorrow"} {
		if got := parseCooldownUntil(raw); !got.IsZero() {
			t.Fatalf("%q must parse as no-cooldown, got %s", raw, got)
		}
	}
}

func TestTorBoxTimestampsParse(t *testing.T) {
	reset()
	got := parseCooldownUntil("2026-07-31T19:23:03Z")
	if got.IsZero() {
		t.Fatal("TorBox's own RFC3339 format must parse")
	}
	if got.Year() != 2026 || got.Month() != time.July || got.Day() != 31 {
		t.Fatalf("parsed wrong instant: %s", got)
	}
}

func TestTheBarReleasesWhenItExpires(t *testing.T) {
	reset()
	setAccountCooldown("k", time.Now().Add(40*time.Millisecond))
	if accountCooldownRemaining("k") == 0 {
		t.Fatal("expected the bar to hold initially")
	}
	time.Sleep(60 * time.Millisecond)
	if accountCooldownRemaining("k") != 0 {
		t.Fatal("the bar must release itself once it expires")
	}
}

// The gate has to hold in the real request path, not just in the bookkeeping.
func TestNoRequestReachesTorBoxWhileBarred(t *testing.T) {
	reset()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"data":{}}`))
	}))
	defer srv.Close()

	c := NewAPIClient(&APIClientConfig{BaseURL: srv.URL, APIKey: "k"})
	// Pretend the probe already ran and found a cooldown.
	cooldownMu.Lock()
	cooldownProbe["k"] = &sync.Once{}
	cooldownProbe["k"].Do(func() {})
	cooldownMu.Unlock()
	setAccountCooldown("k", time.Now().Add(time.Hour))

	for _, path := range []string{
		"/v1/api/torrents/createtorrent",
		"/v1/api/torrents/requestdl",
		"/v1/api/torrents/checkcached",
		"/v1/api/torrents/mylist",
	} {
		if _, err := c.Request("GET", path, &Ctx{APIKey: "k"}, &Response[GetUserData]{}); err == nil {
			t.Fatalf("%s must be refused while barred", path)
		}
	}
	if hits != 0 {
		t.Fatalf("expected zero requests to reach TorBox while barred, got %d", hits)
	}

	// The account endpoint stays reachable - it is how the lift is detected.
	if _, err := c.Request("GET", userPath, &Ctx{APIKey: "k"}, &Response[GetUserData]{}); err != nil {
		t.Fatalf("the account endpoint must stay exempt: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected exactly the account read to go out, got %d", hits)
	}

	// Once the bar lifts, normal calls resume.
	setAccountCooldown("k", time.Now().Add(-time.Second))
	if _, err := c.Request("GET", "/v1/api/torrents/mylist", &Ctx{APIKey: "k"}, &Response[GetUserData]{}); err != nil {
		t.Fatalf("calls must resume once the bar lifts: %v", err)
	}
	if hits != 2 {
		t.Fatalf("expected the call to go out after the lift, got %d hits", hits)
	}
}
