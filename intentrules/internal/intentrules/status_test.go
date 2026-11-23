package intentrules

import "testing"

// The gauge setter is a hook, not an import: PublishStatus reports (rules, enabled) through whatever SetStatusSinks wired, and an
// unwired package is silent rather than a panic.
func TestPublishStatusReportsThroughTheWiredGaugeHook(t *testing.T) {
	oi, oe, os := logInfo, logError, onStatus
	defer func() { logInfo, logError, onStatus = oi, oe, os }()
	var gotRules int
	var gotEnabled, called bool
	SetStatusSinks(func(string, ...interface{}) {}, func(string, ...interface{}) {}, func(r int, e bool) { gotRules, gotEnabled, called = r, e, true })
	n := PublishStatus("server", true)
	if !called || gotRules != n || gotEnabled {
		t.Fatalf("hook called=%v rules=%d (PublishStatus returned %d) enabled=%v, want called, same rule count, enabled=false", called, gotRules, n, gotEnabled)
	}
	logInfo, logError, onStatus = func(string, ...interface{}) {}, func(string, ...interface{}) {}, func(int, bool) {}
	SetStatusSinks(nil, nil, nil) // nil leaves the sinks alone: still silent, no panic
	_ = PublishStatus("worker", false)
}
