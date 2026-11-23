package intentrules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabref"
)

// testdata/netlab/<fixture>.txt is what the pinned, patched netlab printed for the fixture of the same name (re-rendered and
// compared by a parity check, so it cannot go stale unnoticed).
func netlabOutput(t *testing.T, fixture string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "netlab", strings.TrimSuffix(filepath.Base(fixture), ".yml")+".txt"))
	if err != nil {
		t.Fatalf("no captured netlab output for %s: %v", fixture, err)
	}
	return strings.Split(string(b), "\n")
}

// The compile wins and nothing is reported twice: every static finding whose code the compile already carries is dropped, and a
// finding the compile did not make survives.
func TestCompileWinsAndNothingIsReportedTwice(t *testing.T) {
	e := engine(t)
	dropped, kept := 0, 0
	for _, f := range fixtureFiles(t) {
		b, _ := os.ReadFile(f)
		static := e.InspectYAML(string(b), Options{}).Findings
		lines := netlabOutput(t, f)
		owned := NetlabOwnedCodes(lines)
		merged := MergeWithCompile(static, lines, false)
		for _, m := range merged {
			if owned[m.Code] {
				t.Errorf("%s: %s is owned by the compile but is still reported by the static layer", filepath.Base(f), m.Code)
			}
		}
		for _, s := range static {
			survived := false
			for _, m := range merged {
				if m.Code == s.Code && m.Path == s.Path {
					survived = true
				}
			}
			switch {
			case owned[s.Code] && survived:
				t.Errorf("%s: %s was not dropped", filepath.Base(f), s.Code)
			case !owned[s.Code] && !survived:
				t.Errorf("%s: %s was dropped although the compile did not report it", filepath.Base(f), s.Code)
			}
			if owned[s.Code] {
				dropped++
			} else {
				kept++
			}
		}
	}
	// both halves must be exercised, or a merge that always drops (or never drops) passes
	if dropped == 0 || kept == 0 {
		t.Fatalf("the fixtures exercised dropped=%d kept=%d: both must be positive", dropped, kept)
	}
}

// Where the static layer and the compile both speak about the same defect, they must agree on the code and on which nodes it
// names: the netlab-owned codes are a parity contract, not a second opinion.
func TestStaticAndCompileAgreeOnTheCodesNetlabOwns(t *testing.T) {
	e := engine(t)
	agreed := 0
	for _, f := range fixtureFiles(t) {
		b, _ := os.ReadFile(f)
		static := e.InspectYAML(string(b), Options{}).Findings
		compile := netlabref.ClassifyNetlabOutput(netlabOutput(t, f), true)
		for _, s := range static {
			if _, _, _, owns := netlabref.RuleText(s.Code); !owns {
				continue
			}
			var match *netlabref.Finding
			for i := range compile {
				if compile[i].Code == s.Code {
					match = &compile[i]
				}
			}
			if match == nil {
				// netlab may word the defect in a way its classifier has no row for; the text table then owns it
				if !NetlabOwnedCodes(netlabOutput(t, f))[s.Code] {
					t.Errorf("%s: the static layer reports %s but the compile does not", filepath.Base(f), s.Code)
				}
				agreed++
				continue
			}
			if s.Class != match.Class {
				t.Errorf("%s: %s is %s statically and %s from the compile", filepath.Base(f), s.Code, s.Class, match.Class)
			}
			// every node the static layer names, the compile names too, when it names any
			if len(match.Subject) > 0 {
				have := map[string]bool{}
				for _, n := range match.Subject {
					have[n] = true
				}
				for _, n := range s.Subject {
					if !have[n] {
						t.Errorf("%s: static %s names %q, the compile names %v", filepath.Base(f), s.Code, n, match.Subject)
					}
				}
			}
			agreed++
		}
	}
	if agreed < 4 {
		t.Fatalf("only %d agreements checked", agreed)
	}
}

// Every fixture the rules call clean must also be clean for netlab on the codes this layer can emit: the other direction of
// parity, the false-positive guard against real compiler output.
func TestNoStaticFindingOnAFixtureNetlabCompilesCleanOnThoseCodes(t *testing.T) {
	e := engine(t)
	emits := map[string]bool{}
	for _, r := range e.Rules {
		emits[r.Code] = true
	}
	for _, f := range fixtureFiles(t) {
		b, _ := os.ReadFile(f)
		lines := netlabOutput(t, f)
		rcZero := false
		for _, l := range lines {
			if strings.TrimSpace(l) == "rc=0" {
				rcZero = true
			}
		}
		if !rcZero {
			continue // netlab refused the topology; only the codes it names are comparable, and the test above did that
		}
		owned := NetlabOwnedCodes(lines)
		for _, s := range e.InspectYAML(string(b), Options{}).Findings {
			if _, _, _, netlabKnows := netlabref.RuleText(s.Code); netlabKnows && !owned[s.Code] {
				t.Errorf("%s: netlab compiled it without %s, the static layer reports it", filepath.Base(f), s.Code)
			}
		}
	}
}

func TestDesignerIssuesAreWarningsWithAPathAndTheFix(t *testing.T) {
	e := engine(t)
	b, _ := os.ReadFile("testdata/evpn-svi-ospf-default.yml")
	issues := e.DesignerIssues(string(b))
	if len(issues) != 1 {
		t.Fatalf("want one issue, got %+v", issues)
	}
	i := issues[0]
	if i.Severity != "warning" || i.Path != "vlans.red" || !strings.Contains(i.Message, "evpn-svi-ospf-overlay") || !strings.Contains(i.Message, "vlans.<name>.ospf: false") {
		t.Errorf("issue = %+v", i)
	}
	clean, _ := os.ReadFile("testdata/evpn-clean.yml")
	if got := e.DesignerIssues(string(clean)); len(got) != 0 {
		t.Errorf("a clean design must give no issues: %+v", got)
	}
	if got := e.DesignerIssues("::: not yaml :::\n\t- ["); len(got) != 0 {
		t.Errorf("a document that does not parse is the preflight's to report, not this layer's: %+v", got)
	}
}

// A document the loader cannot resolve says so instead of passing: the result lists what was skipped.
func TestWhatCannotBeJudgedIsListedNotPassed(t *testing.T) {
	e := engine(t)
	b, _ := os.ReadFile("testdata/plugin-generated-node-link.yml")
	res := e.InspectYAML(string(b), Options{})
	if res.Skipped["link-unknown-node"] == "" {
		t.Errorf("a node-generating plugin makes link references unjudgeable; skipped = %v", res.Skipped)
	}
	if len(res.Evaluated) == 0 {
		t.Error("evaluated list is empty")
	}
}

// A document that cannot be resolved in full is not judged where it cannot be: an included file, or a plugin this build does not
// know, may define nodes, so a link naming an undefined node is not a finding.
func TestUnresolvableDocumentsAreSkippedNotFlagged(t *testing.T) {
	e := engine(t)
	cases := map[string]string{
		"includes": "defaults.device: eos\nnodes:\n  r1:\n  _include: [ more.yml ]\nlinks:\n- r1-r2\n",
		"plugin":   "defaults.device: eos\nplugin: [ some-local-plugin ]\nnodes:\n  r1:\nlinks:\n- r1-r2\n",
	}
	for name, doc := range cases {
		res := e.InspectYAML(doc, Options{})
		for _, f := range res.Findings {
			if f.Code == "link-unknown-node" {
				t.Errorf("%s: a link to a node that may be defined elsewhere was flagged: %+v", name, f)
			}
		}
		if res.Skipped["link-unknown-node"] == "" && res.Skipped["includes"] == "" {
			t.Errorf("%s: nothing says the references were not judged: %v", name, res.Skipped)
		}
	}
	// control: the same links with nothing that could define a node ARE flagged
	res := e.InspectYAML("defaults.device: eos\nnodes:\n  r1:\nlinks:\n- r1-r2\n", Options{})
	if len(res.Findings) != 1 || res.Findings[0].Code != "link-unknown-node" {
		t.Errorf("control: want one link-unknown-node, got %+v", res.Findings)
	}
}

// Cyclic groups with fan-out must not cost exponential time: one document must not be able to hang the designer.
func TestCyclicGroupsDoNotBlowUp(t *testing.T) {
	e := engine(t)
	var b strings.Builder
	b.WriteString("defaults.device: eos\ngroups:\n")
	names := []string{"a", "b", "c", "d", "e", "f"}
	for _, n := range names {
		b.WriteString("  " + n + ":\n    members: [ " + strings.Join(names, ", ") + ", r1 ]\n    module: [ ospf ]\n")
	}
	b.WriteString("nodes:\n  r1:\n")
	done := make(chan struct{})
	go func() { e.InspectYAML(b.String(), Options{}); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("six mutually-recursive groups took more than five seconds")
	}
}

// When the compile succeeded, a code netlab can own that it did not report is the layer's model being wrong and is dropped; a code
// netlab has no check for is kept. When the compile failed, the same finding is kept (netlab may not have reached its check).
func TestAGoodCompileDropsCodesNetlabCouldHaveOwned(t *testing.T) {
	owned := netlabref.Finding{Code: "evpn-svi-ospf-overlay", Path: "vlans.red"}
	unowned := netlabref.Finding{Code: "node-name-duplicate", Path: "nodes[2]"}
	if !NetlabChecksCode("evpn-svi-ospf-overlay") || !NetlabChecksCode("link-endpoints") || NetlabChecksCode("an-invented-code") || NetlabChecksCode("node-name-duplicate") {
		t.Fatal("NetlabChecksCode disagrees with the classifier and the text table")
	}
	got := MergeWithCompile([]netlabref.Finding{owned, {Code: "link-endpoints"}}, nil, true)
	if len(got) != 0 {
		t.Errorf("a good compile must drop codes netlab can own: %+v", got)
	}
	got = MergeWithCompile([]netlabref.Finding{owned, {Code: "link-endpoints"}}, nil, false)
	if len(got) != 2 {
		t.Errorf("a compile that failed without naming them must keep them: %+v", got)
	}
	got = MergeWithCompile([]netlabref.Finding{unowned, {Code: "an-invented-code"}}, nil, true)
	if len(got) != 2 {
		t.Errorf("codes netlab does not check exhaustively must survive a good compile: %+v", got)
	}
}
