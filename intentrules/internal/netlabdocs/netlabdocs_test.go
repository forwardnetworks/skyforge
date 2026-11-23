package netlabdocs

import (
	"strings"
	"testing"
)

// The corpus must be able to answer the question that started this work: an SE
// asked where to read link-attribute syntax and could not find it.
func TestCorpusAnswersTheLinkAttributeQuestion(t *testing.T) {
	links, ok := Read("links.md")
	if !ok {
		t.Fatal("links.md missing from the corpus")
	}
	// The dictionary-format rule is the actual answer to his question.
	if !strings.Contains(links, "dictionary format of the link definition") {
		t.Fatal("links.md does not contain the dictionary-format rule; wrong subset?")
	}
	if hits := Search("eigrp", "link"); len(hits) == 0 {
		t.Fatal("searching for eigrp+link found nothing")
	}
	// The <module>: false idiom he used is documented per module.
	eigrp, ok := Read("module/eigrp.md")
	if !ok {
		t.Fatal("module/eigrp.md missing")
	}
	if !strings.Contains(eigrp, "eigrp: False") {
		t.Fatal("module/eigrp.md does not document the `<module>: False` idiom")
	}
}

// Path handling must refuse traversal and must not pretend a miss is a hit.
func TestReadRefusesTraversalAndReportsMisses(t *testing.T) {
	for _, bad := range []string{"", "  ", "../../etc/passwd", "corpus/../../x", "nope.md"} {
		if body, ok := Read(bad); ok || body != "" {
			t.Fatalf("Read(%q) should have failed, got ok=%v len=%d", bad, ok, len(body))
		}
	}
	// POSITIVE CONTROL: a real path must succeed, or the test above proves only
	// that Read always fails.
	if _, ok := Read("links.md"); !ok {
		t.Fatal("Read rejected a valid path; the negative cases above prove nothing")
	}
	if _, ok := Read("/links.md"); !ok {
		t.Fatal("a leading slash should be tolerated")
	}
}

// The index must describe the corpus that actually shipped. They are generated
// together, so disagreement means one was edited by hand.
func TestIndexMatchesTheEmbeddedCorpus(t *testing.T) {
	entries := List()
	if len(entries) == 0 {
		t.Fatal("index is empty")
	}
	for _, e := range entries {
		body, ok := Read(e.Path)
		if !ok {
			t.Fatalf("index names %s but it is not embedded", e.Path)
		}
		if len(body) != e.Bytes {
			t.Fatalf("%s: index says %d bytes, corpus has %d", e.Path, e.Bytes, len(body))
		}
	}
	if hits := Search("a-string-that-should-not-appear-anywhere-xyzzy"); len(hits) != 0 {
		t.Fatalf("Search matched a nonsense term: %v", hits)
	}
}

// RANKING IS THE FEATURE. Asserting only that links.md is "somewhere in the
// results" would have passed before ranking existed, when it sat below
// addressing.md and caveats.md and the reader had to guess. Assert POSITION.
func TestSearchRanksTheAnswerFirst(t *testing.T) {
	hits := Search("link", "attribute")
	if len(hits) == 0 {
		t.Fatal("no hits for link+attribute")
	}
	if hits[0] != "links.md" {
		t.Fatalf("links.md must rank first for a link-attribute question, got %v", hits[:min(4, len(hits))])
	}
	// The examples page links.md itself points at should be near the top.
	found := false
	for _, h := range hits[:min(3, len(hits))] {
		if h == "example/link-definition.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("example/link-definition.md should rank in the top 3, got %v", hits[:min(4, len(hits))])
	}

	// A filename match must beat a body match: searching a module name must land
	// on that module's page.
	if h := Search("eigrp"); len(h) == 0 || h[0] != "module/eigrp.md" {
		t.Fatalf("searching a module name must rank its page first, got %v", h)
	}
	// POSITIVE CONTROL: ranking must not simply prefer short paths regardless of
	// content, or the two assertions above would pass for the wrong reason.
	if h := Search("vrf"); len(h) == 0 || !strings.HasPrefix(h[0], "module/vrf") {
		t.Fatalf("expected module/vrf.md to win on a vrf search, got %v", h)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// The user-visible symptom: ACL syntax must be findable and readable.
func TestCorpusCarriesRoutingACLSyntax(t *testing.T) {
	body, ok := Read("module/routing-acl.txt")
	if !ok {
		t.Fatal("module/routing-acl.txt missing: routing.acl syntax is not reachable")
	}
	if !strings.Contains(body, "routing.acl") {
		t.Fatal("module/routing-acl.txt does not document routing.acl")
	}
	found := false
	for _, h := range Search("routing.acl", "access", "list") {
		if h == "module/routing-acl.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("Search(routing.acl access list) does not reach module/routing-acl.txt")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
