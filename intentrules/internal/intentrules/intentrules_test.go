package intentrules

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabref"
)

func engine(t *testing.T) *Engine {
	t.Helper()
	e, err := Default()
	if err != nil {
		t.Fatalf("the embedded model does not load: %v", err)
	}
	return e
}

var expectRE = regexp.MustCompile(`(?m)^# expect: (.+)$`)

func fixtureFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found (err=%v): a test that runs zero cases proves nothing", err)
	}
	return files
}

func codes(fs []netlabref.Finding) []string {
	set := map[string]bool{}
	for _, f := range fs {
		set[f.Code] = true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func wantCodes(t *testing.T, text string) []string {
	t.Helper()
	m := expectRE.FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("fixture has no '# expect:' header")
	}
	if strings.TrimSpace(m[1]) == "none" {
		return nil
	}
	var out []string
	for _, c := range strings.Split(m[1], ",") {
		out = append(out, strings.TrimSpace(c))
	}
	sort.Strings(out)
	return out
}

// Every fixture declares the codes it must produce; "none" is a clean fixture that must produce nothing. The fixtures are written
// from the requirement (a design with one named defect each), not from the output of the code under test.
func TestEveryFixtureProducesExactlyItsDeclaredCodes(t *testing.T) {
	e := engine(t)
	ran := 0
	for _, f := range fixtureFiles(t) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			want := wantCodes(t, string(b))
			res := e.InspectYAML(string(b), Options{})
			got := codes(res.Findings)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("codes = %v, want %v\nfindings: %+v\nskipped: %v", got, want, res.Findings, res.Skipped)
			}
			for _, fd := range res.Findings {
				if fd.Source != netlabref.SourceIntentRules || fd.Why == "" || fd.Fix == "" || fd.Message == "" || fd.Path == "" {
					t.Errorf("finding is missing a field the surfaces rely on: %+v", fd)
				}
			}
		})
		ran++
	}
	if ran < 10 {
		t.Fatalf("only %d fixtures ran", ran)
	}
}

// Positive control: the same defective fixtures, with the rule turned off, must come out clean. A fixture that still fails with its
// rule off was failing for another reason, and proves nothing about the rule.
func TestTurningTheRuleOffMakesTheDefectivePass(t *testing.T) {
	e := engine(t)
	for _, f := range fixtureFiles(t) {
		b, _ := os.ReadFile(f)
		want := wantCodes(t, string(b))
		if len(want) == 0 {
			continue
		}
		off := map[string]bool{}
		for _, c := range want {
			off[c] = true
		}
		res := e.InspectYAML(string(b), Options{Disabled: off})
		if got := codes(res.Findings); len(got) != 0 {
			t.Errorf("%s: with %v disabled the fixture still produces %v", filepath.Base(f), want, got)
		}
	}
}

// The sidecar cannot drift from the schema: a key that names nothing is refused.
func TestSidecarKeyThatResolvesToNothingIsRefused(t *testing.T) {
	e := engine(t)
	_, err := LoadSidecar("x", []byte("sidecar_version: 1\nannotations:\n  NetlabDevice.nonesuch:\n    netlab: {key: a}\n"), e.Schema)
	if err == nil || !strings.Contains(err.Error(), "drifted") {
		t.Fatalf("want a drift error, got %v", err)
	}
	_, err = LoadSidecar("x", []byte("sidecar_version: 1\nannotations:\n  NetlabNope.name:\n    netlab: {key: a}\n"), e.Schema)
	if err == nil {
		t.Fatal("a kind the schema does not declare must be refused")
	}
}

// Infrahub rejects any key it does not know; so does the loader, from the published schema.
func TestAnExtensionKeyInTheSchemaIsRejected(t *testing.T) {
	good := "version: \"1.0\"\nnodes:\n  - name: Thing\n    namespace: Test\n    attributes:\n      - name: label\n        kind: Text\n"
	if _, err := LoadSchema("good", []byte(good)); err != nil {
		t.Fatalf("a plain schema must load: %v", err)
	}
	for name, bad := range map[string]string{
		"node":      strings.Replace(good, "    namespace: Test\n", "    namespace: Test\n    x_skyforge: {a: 1}\n", 1),
		"attribute": strings.Replace(good, "        kind: Text\n", "        kind: Text\n        x_skyforge: {a: 1}\n", 1),
		"root":      "x_skyforge: 1\n" + good,
	} {
		if _, err := LoadSchema(name, []byte(bad)); err == nil || !strings.Contains(err.Error(), "not an Infrahub schema key") {
			t.Errorf("%s: an extension key must be rejected, got %v", name, err)
		}
	}
}

// The rules file may only name kinds, attributes, relationships and codes that exist.
func TestRuleThatNamesSomethingMissingIsRefused(t *testing.T) {
	e := engine(t)
	cases := map[string]string{
		"attribute":    "rules_version: 1\nrules:\n  - id: x\n    code: link-endpoints\n    kind: NetlabVlan\n    where: {attr: nonesuch, eq: a}\n    assert: {attr: mode, eq: irb}\n    message: m\n",
		"relationship": "rules_version: 1\nrules:\n  - id: x\n    code: link-endpoints\n    kind: NetlabVlan\n    assert: {related: {rel: nonesuch}}\n    message: m\n",
		"code":         "rules_version: 1\nrules:\n  - id: x\n    code: no-such-code\n    kind: NetlabVlan\n    assert: {attr: mode, eq: irb}\n    message: m\n",
		"two forms":    "rules_version: 1\nrules:\n  - id: x\n    code: link-endpoints\n    kind: NetlabVlan\n    assert: {attr: mode, eq: irb}\n    assert_not: {attr: mode, eq: irb}\n    message: m\n",
		"scripting":    "rules_version: 1\nrules:\n  - id: x\n    code: link-endpoints\n    kind: NetlabVlan\n    assert: {expr: \"1+1\"}\n    message: m\n",
	}
	for name, doc := range cases {
		if _, err := LoadRules(name, []byte(doc), e.Schema, e.Sidecar); err == nil {
			t.Errorf("%s: the rules file must be refused", name)
		}
	}
}

// Deleting a rule must fail the suite: every code the model can emit has a fixture that declares it, so removing the rule (or the
// constraint's finding mapping) leaves a fixture expecting a code nothing produces.
func TestEveryCodeTheModelEmitsHasAFixtureThatNeedsIt(t *testing.T) {
	e := engine(t)
	emitted := map[string]string{}
	for _, r := range e.Rules {
		emitted[r.Code] = "rule " + r.ID
	}
	for key, a := range e.Sidecar.Annotations {
		if a == nil {
			continue
		}
		for c, code := range a.Findings {
			emitted[code] = "constraint " + key + ":" + c
		}
	}
	declared := map[string]bool{}
	for _, f := range fixtureFiles(t) {
		b, _ := os.ReadFile(f)
		for _, c := range wantCodes(t, string(b)) {
			declared[c] = true
		}
	}
	for code, who := range emitted {
		if !declared[code] {
			t.Errorf("%s reports %s, but no fixture declares it: nothing would fail if it were deleted", who, code)
		}
	}
}
