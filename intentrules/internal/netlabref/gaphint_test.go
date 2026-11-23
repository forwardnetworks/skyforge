package netlabref

import (
	"strings"
	"testing"
)

// THE CASE THAT MATTERS MOST: valid authoring must file NOTHING.
//
// An SE wrote `eigrp: false` on a link on 2026-09-03. It is correct netlab --
// netsim/defaults/attributes.yml declares can_be_false: [link, interface,
// loopback] -- but it is not a module ATTRIBUTE, so a naive "is it in the
// attribute list?" check calls it unmodelled. That would file a gap issue
// against a person who did nothing wrong, and a queue that cries wolf is worse
// than no queue.
func TestValidAuthoringIsNeverClassifiedAsAGap(t *testing.T) {
	// What netlab actually emits for his topology, captured from the real
	// compiler in the pinned image on 2026-09-03.
	captured := `MissingValue in links: Interface data links[2].interfaces[2] is missing a "node" attribute
... found {'r1': None, 'eigrp': False}`
	hint, reason := GapHintFor(captured)
	if hint == GapHintNotModelled {
		t.Fatalf("valid authoring classified as a netlab gap: %s / %s", hint, reason)
	}

	// The unknown-node shape, which is how the same mistake reads in other runs.
	hint, reason = GapHintFor("IncorrectValue in links: link[2] refers to an unknown node eigrp")
	if hint != GapHintAuthorError {
		t.Fatalf("want author-error for a module name in node position, got %s (%s)", hint, reason)
	}
	if !strings.Contains(reason, "DICTIONARY link form") {
		t.Fatalf("the reason must tell the author what to actually change; got %q", reason)
	}
	if instructsFiling(hint) {
		t.Fatal("an author error must not instruct filing an issue")
	}
}

// A module that exists but is not enabled is an author error with a known fix.
func TestKnownModuleNotEnabledIsAuthorError(t *testing.T) {
	hint, reason := GapHintFor(
		"IncorrectAttr in links: links[2].r1 uses an attribute from module eigrp which is not enabled in nodes.r1")
	if hint != GapHintAuthorError {
		t.Fatalf("want author-error, got %s (%s)", hint, reason)
	}
	if !strings.Contains(reason, "module:") {
		t.Fatalf("reason should name the fix (enable the module); got %q", reason)
	}
}

// POSITIVE CONTROL: a genuine gap MUST be classified as one, or the loop never
// fires and the previous test passes for the wrong reason.
func TestGenuineGapIsClassifiedAndInstructsFiling(t *testing.T) {
	hint, reason := GapHintFor(
		"IncorrectAttr in nodes: nodes.r1 uses an attribute from module quantumrouting which is not enabled in nodes.r1")
	if hint != GapHintNotModelled {
		t.Fatalf("want not-modelled for a module netlab has never heard of, got %s (%s)", hint, reason)
	}
	note := GapHintNote(hint)
	assertFilingNote(t, note)
}

// UNRESOLVABLE MUST NOT MEAN "GAP". The default has to be the classification
// whose cost is a wasted minute, not a wrong issue filed against a colleague.
func TestUnresolvableFailsTowardsAuthorError(t *testing.T) {
	for _, in := range []string{
		"",
		"   ",
		"Fatal error in netlab: Cannot proceed beyond this point due to errors, exiting",
		"something entirely unparseable happened",
	} {
		hint, _ := GapHintFor(in)
		if hint == GapHintNotModelled {
			t.Fatalf("unresolvable input %q was classified as a netlab gap", in)
		}
		if instructsFiling(hint) {
			t.Fatalf("unresolvable input %q instructs filing an issue", in)
		}
	}
}
