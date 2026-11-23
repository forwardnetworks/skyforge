package intentrules

// LogFunc is the shape of a structured log sink (encore.dev/rlog's Info and Error have it).
type LogFunc func(msg string, ctx ...interface{})

// This package imports neither encore.dev nor the server's metrics package, so it builds and tests with plain Go and can live in a
// standalone module. The server wires the real sinks once at process start with SetStatusSinks, before PublishStatus. Unwired, the
// sinks are no-ops: the layer is advisory and must never panic or fail the start. The log sinks are also variables so the unit tests
// can record the line instead of emitting it.
var (
	logInfo  LogFunc = func(string, ...interface{}) {}
	logError LogFunc = func(string, ...interface{}) {}
	// onStatus receives (rules loaded, layer enabled) exactly once per PublishStatus: the server maps it to the
	// skyforge_intentrules_loaded / _enabled gauges.
	onStatus = func(rules int, enabled bool) {}
)

// SetStatusSinks wires PublishStatus's log lines and gauge setter. A nil argument leaves that sink as it was.
func SetStatusSinks(info, errLog LogFunc, status func(rules int, enabled bool)) {
	if info != nil {
		logInfo = info
	}
	if errLog != nil {
		logError = errLog
	}
	if status != nil {
		onStatus = status
	}
}

// PublishStatus says, once at process start, whether the rules model the binary embeds loaded and whether the layer is on. It
// logs one line (role is "server" or "worker") and sets skyforge_intentrules_loaded / _enabled. A silent layer is
// indistinguishable from a healthy one, so absence of this line in a new image is itself the finding.
// It never panics and never fails the start: the layer is advisory.
func PublishStatus(role string, disabled bool) (rules int) {
	var e *Engine
	var err error
	if fault := Guard(func() { e, err = Default() }); fault != "" {
		logError("intent rules layer status: load panicked", "role", role, "cause", fault)
		onStatus(0, !disabled)
		return 0
	}
	if err != nil || e == nil {
		logError("intent rules layer status: the embedded model did not load", "role", role, "enabled", !disabled, "err", err)
		onStatus(0, !disabled)
		return 0
	}
	rules = len(e.Rules)
	logInfo("intent rules layer loaded", "role", role, "rules", rules, "enabled", !disabled)
	onStatus(rules, !disabled)
	return rules
}
