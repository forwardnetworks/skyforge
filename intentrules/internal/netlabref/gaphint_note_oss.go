package netlabref

// GapHintNote is the instruction that travels with the hint, so the caller does
// not have to remember the policy.
func GapHintNote(hint string) string {
	switch hint {
	case GapHintNotModelled:
		return "netlab does not model this, and it is not a near-miss of anything it does. That is a gap in netlab, " +
			"not in the topology: report this as a gap, quoting netlab's own error text and the topology fragment."
	case GapHintAuthorError:
		return "netlab models this — the topology is wrong. Fix it, say exactly what to change, and do not report a gap."
	default:
		return "could not classify this failure. Treat it as an author error, check the netlab capability manifest " +
			"yourself, and do not report a gap on a guess."
	}
}
