package auth

import (
	"encoding/json"
	"math"
	"time"

	"github.com/tidwall/gjson"
)

const quotaLowRemainingPercent = 2.0

type quotaCooldownDecision struct {
	Window           string
	UsedPercent      float64
	RemainingPercent float64
	Until            time.Time
}

func parseQuotaWindows(raw json.RawMessage, checkedAt time.Time) (primary, secondary *QuotaWindowInfo) {
	if len(raw) == 0 {
		return nil, nil
	}
	return parseQuotaWindow(raw, "rate_limit.primary_window", checkedAt),
		parseQuotaWindow(raw, "rate_limit.secondary_window", checkedAt)
}

func parseQuotaWindow(raw json.RawMessage, path string, checkedAt time.Time) *QuotaWindowInfo {
	window := gjson.GetBytes(raw, path)
	if !window.Exists() || !window.IsObject() {
		return nil
	}
	used, hasUsed := finiteJSONFloat(window, "used_percent")
	windowMinutes, hasWindowMinutes := finiteJSONFloat(window, "window_minutes")
	resetSeconds, hasResetSeconds := resetSecondsFromWindow(window, checkedAt)
	if !hasUsed && !hasWindowMinutes && !hasResetSeconds {
		return nil
	}
	if !hasUsed {
		used = 0
	}
	if !hasWindowMinutes {
		if seconds, ok := finiteJSONFloat(window, "limit_window_seconds"); ok && seconds > 0 {
			windowMinutes = seconds / 60
		}
	}
	remaining := 100 - used
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}
	info := &QuotaWindowInfo{
		UsedPercent:      used,
		RemainingPercent: remaining,
		WindowMinutes:    windowMinutes,
		ResetsInSeconds:  resetSeconds,
	}
	if resetSeconds > 0 {
		resetAt := checkedAt.Add(time.Duration(resetSeconds) * time.Second)
		info.ResetAt = &resetAt
	}
	return info
}

func finiteJSONFloat(window gjson.Result, key string) (float64, bool) {
	value := window.Get(key)
	if !value.Exists() {
		return 0, false
	}
	f := value.Float()
	return f, !math.IsNaN(f) && !math.IsInf(f, 0)
}

func resetSecondsFromWindow(window gjson.Result, checkedAt time.Time) (int64, bool) {
	for _, key := range []string{"resets_in_seconds", "reset_after_seconds"} {
		if v, ok := finiteJSONFloat(window, key); ok && v > 0 {
			return int64(math.Ceil(v)), true
		}
	}
	for _, key := range []string{"reset_at", "resets_at"} {
		if v, ok := finiteJSONFloat(window, key); ok && v > 0 {
			resetAt := time.Unix(int64(v), 0)
			seconds := int64(math.Ceil(resetAt.Sub(checkedAt).Seconds()))
			if seconds > 0 {
				return seconds, true
			}
		}
	}
	return 0, false
}

func quotaCooldownForInfo(info *QuotaInfo, now time.Time) (quotaCooldownDecision, bool) {
	if info == nil || !info.Valid {
		return quotaCooldownDecision{}, false
	}
	if decision, ok := quotaCooldownForWindow("weekly", info.SecondaryWindow, now); ok {
		return decision, true
	}
	return quotaCooldownForWindow("5h", info.PrimaryWindow, now)
}

func quotaCooldownForWindow(name string, window *QuotaWindowInfo, now time.Time) (quotaCooldownDecision, bool) {
	if window == nil {
		return quotaCooldownDecision{}, false
	}
	remaining := window.RemainingPercent
	if remaining == 0 && window.UsedPercent != 100 {
		remaining = 100 - window.UsedPercent
	}
	if remaining < 0 {
		remaining = 0
	}
	if remaining >= quotaLowRemainingPercent {
		return quotaCooldownDecision{}, false
	}
	var until time.Time
	if window.ResetAt != nil && window.ResetAt.After(now) {
		until = *window.ResetAt
	} else if window.ResetsInSeconds > 0 {
		until = now.Add(time.Duration(window.ResetsInSeconds) * time.Second)
	}
	if until.IsZero() || !until.After(now) {
		return quotaCooldownDecision{}, false
	}
	return quotaCooldownDecision{
		Window:           name,
		UsedPercent:      window.UsedPercent,
		RemainingPercent: remaining,
		Until:            until,
	}, true
}
