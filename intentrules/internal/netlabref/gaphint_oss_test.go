package netlabref

import (
	"strings"
	"testing"
)

// instructsFiling reports whether a hint's note tells the reader to report a gap.
func instructsFiling(hint string) bool {
	return strings.Contains(GapHintNote(hint), "report this as a gap")
}

// assertFilingNote checks the not-modelled note carries everything needed to report.
func assertFilingNote(t *testing.T, note string) {
	t.Helper()
	for _, want := range []string{"report this as a gap", "error text", "topology fragment"} {
		if !strings.Contains(note, want) {
			t.Fatalf("the report instruction is incomplete, missing %q: %s", want, note)
		}
	}
}
