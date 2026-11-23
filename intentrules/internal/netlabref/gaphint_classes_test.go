package netlabref

import (
	"strings"
	"testing"
)

// Every case below is netlab's REAL text, captured from the live compiler on
// 2026-09-03 by putting the topology through it -- not composed to fit the
// classifier. The typo case is here because the previous classifier got it
// WRONG: it filed a netlab gap issue for a misspelling.
func TestEveryErrorClassGetsTheRightSignal(t *testing.T) {
	cases := []struct {
		name       string
		netlab     string
		wantHint   string
		wantClass  string
		wantInWhy  string
		wantSugges string
	}{
		{
			name:      "typo in a link attribute",
			netlab:    "IncorrectAttr in links: Invalid link attribute 'eigpr' found in links[1]\n... use 'netlab show attributes link' to display valid attributes",
			wantHint:  GapHintAuthorError,
			wantClass: ClassTypo, wantInWhy: "did you mean", wantSugges: "eigrp",
		},
		{
			name:      "module exists but is not enabled",
			netlab:    "IncorrectAttr in links: links[2].r1 uses an attribute from module eigrp which is not enabled in nodes.r1",
			wantHint:  GapHintAuthorError,
			wantClass: ClassNotEnabled, wantInWhy: "module:", wantSugges: "eigrp",
		},
		{
			name:      "list form: the interface lost its node",
			netlab:    "MissingValue in links: Interface data links[2].interfaces[2] is missing a \"node\" attribute\n... found {'r1': None, 'eigrp': False}",
			wantHint:  GapHintAuthorError,
			wantClass: ClassWrongShape, wantInWhy: "DICTIONARY link form",
		},
		{
			name:      "list form: module read as a node name",
			netlab:    "IncorrectValue in links: link[2] refers to an unknown node eigrp",
			wantHint:  GapHintAuthorError,
			wantClass: ClassWrongShape, wantInWhy: "MODULE, not a node", wantSugges: "eigrp",
		},
		{
			name:      "genuinely unmodelled",
			netlab:    "IncorrectAttr in nodes: nodes.r1 uses an attribute from module quantumrouting which is not enabled in nodes.r1",
			wantHint:  GapHintNotModelled,
			wantClass: ClassUnmodelled, wantInWhy: "nothing is a near-miss of it",
		},
		{
			name:      "unresolvable",
			netlab:    "Fatal error in netlab: Cannot proceed beyond this point due to errors, exiting",
			wantHint:  GapHintUnknown,
			wantClass: ClassUnresolved,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.netlab)
			if got.Hint != c.wantHint {
				t.Fatalf("hint = %s, want %s (reason: %s)", got.Hint, c.wantHint, got.Reason)
			}
			if got.Class != c.wantClass {
				t.Fatalf("class = %s, want %s (reason: %s)", got.Class, c.wantClass, got.Reason)
			}
			if c.wantInWhy != "" && !strings.Contains(got.Reason, c.wantInWhy) {
				t.Fatalf("reason %q does not contain %q", got.Reason, c.wantInWhy)
			}
			if c.wantSugges != "" && got.Suggestion != c.wantSugges {
				t.Fatalf("suggestion = %q, want %q", got.Suggestion, c.wantSugges)
			}
			// ONLY a real gap may instruct filing. This is the assertion that
			// matters: everything else must stay silent.
			filing := instructsFiling(got.Hint)
			if filing != (c.wantHint == GapHintNotModelled) {
				t.Fatalf("filing instruction = %v for class %s; only not-modelled may file", filing, got.Class)
			}
		})
	}
}

// A transposition is the commonest typo shape and the one netlabcaps.NearestNames
// cannot see: "eigpr" neither contains nor is contained by "eigrp". If this
// regresses, every transposed module name becomes a false gap issue again.
func TestTranspositionIsCaughtAsATypoNotAGap(t *testing.T) {
	for _, typo := range []string{"eigpr", "eigrpp", "eigr", "ospv", "bpg"} {
		got := Classify("IncorrectAttr in links: Invalid link attribute '" + typo + "' found in links[1]")
		if got.Hint == GapHintNotModelled {
			t.Fatalf("typo %q was filed as a netlab gap (reason: %s)", typo, got.Reason)
		}
		if got.Class != ClassTypo {
			t.Fatalf("typo %q classified %s, want %s (reason: %s)", typo, got.Class, ClassTypo, got.Reason)
		}
		if got.Suggestion == "" {
			t.Fatalf("typo %q produced no suggestion; the author is left guessing", typo)
		}
	}
	// POSITIVE CONTROL: a name that is NOT a near-miss must still reach
	// not-modelled, or the guard above would be achieved by never filing at all.
	got := Classify("IncorrectAttr in links: Invalid link attribute 'flurbleglop' found in links[1]")
	if got.Hint != GapHintNotModelled {
		t.Fatalf("a genuinely unknown name must be a gap, got %s (%s)", got.Hint, got.Reason)
	}
}
