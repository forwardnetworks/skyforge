package netlabref

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The fixtures in testdata/findings are REAL `netlab create` output on the pinned, patched tree (a check script
// re-renders them live and fails if the text drifts). Each .yml's first line says which codes the output must produce; the
// expectation comes from the requirement ("iBGP with no IGP is a will-not-work finding"), not from reading the rules.

var reExpect = regexp.MustCompile(`(?m)^# expect: (.+)$`)

func fixtures(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/findings/*.yml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no findings fixtures found (err=%v): a test that runs zero cases proves nothing", err)
	}
	return files
}

func TestEveryFixtureProducesExactlyItsExpectedCodes(t *testing.T) {
	ran := 0
	for _, y := range fixtures(t) {
		name := strings.TrimSuffix(filepath.Base(y), ".yml")
		yml, _ := os.ReadFile(y)
		m := reExpect.FindSubmatch(yml)
		if m == nil {
			t.Fatalf("%s: no '# expect:' header", y)
		}
		want := []string{}
		if e := strings.TrimSpace(string(m[1])); e != "none" {
			for _, c := range strings.Split(e, ",") {
				want = append(want, strings.TrimSpace(c))
			}
		}
		out, err := os.ReadFile("testdata/findings/" + name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		for _, f := range ClassifyNetlabOutput(strings.Split(string(out), "\n"), false) {
			got = append(got, f.Code)
		}
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: codes = %v, want %v\noutput:\n%s", name, got, want, out)
		}
		ran++
	}
	if ran < 10 {
		t.Fatalf("only %d fixtures ran", ran)
	}
}

func TestMissingIGPIsWillNotWorkAndNamesTheNodesAndTheFix(t *testing.T) {
	out, _ := os.ReadFile("testdata/findings/spine-no-igp.txt")
	fs := ClassifyNetlabOutput(strings.Split(string(out), "\n"), false)
	if len(fs) != 1 {
		t.Fatalf("two nodes with the same problem must merge into one finding, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Class != ClassWillNotWork || f.IsError {
		t.Errorf("class/isError = %s/%v, want will-not-work compiled-warning", f.Class, f.IsError)
	}
	if strings.Join(f.Subject, ",") != "S1,S2" {
		t.Errorf("subjects = %v, want S1,S2", f.Subject)
	}
	for _, must := range []string{"S1, S2", "IGP"} {
		if !strings.Contains(f.Message, must) {
			t.Errorf("message lacks %q: %s", must, f.Message)
		}
	}
	if !strings.Contains(f.Fix, "ospf") || !strings.Contains(f.Why, "loopback") {
		t.Errorf("fix/why not explained: %+v", f)
	}
}

func TestAPluginBlockWithoutItsPluginNamesThePlugin(t *testing.T) {
	// The group error alone never says why the group is empty; netlab's own hint names the plugin and the finding keeps it.
	out, _ := os.ReadFile("testdata/findings/plugin-block-no-plugin.txt")
	fs := ClassifyNetlabOutput(strings.Split(string(out), "\n"), false)
	if len(fs) != 1 || fs[0].Code != "group-module-no-members" {
		t.Fatalf("got %+v", fs)
	}
	if !strings.Contains(fs[0].Message, "plugin: [ fabric ]") || strings.Join(fs[0].Subject, ",") != "leafs,spines" {
		t.Errorf("message/subjects lose the cause: %q %v", fs[0].Message, fs[0].Subject)
	}
	if !fs[0].IsError {
		t.Error("a netlab error must be flagged IsError")
	}
}

func TestHousekeepingIsCountedButNotReturnedByDefault(t *testing.T) {
	lines := []string{
		"Warning in vrf: Node r1 uses no VRFs, removing 'vrf' from node modules",
		"Warning in netmiko: Netmiko is not installed, device(s) eos will be configured with Ansible",
	}
	if got := ClassifyNetlabOutput(lines, false); len(got) != 0 {
		t.Errorf("informational findings must not reach the diagnostics: %+v", got)
	}
	got := ClassifyNetlabOutput(lines, true)
	if len(got) != 2 || got[0].Class != ClassInformational {
		t.Errorf("telemetry view must keep them: %+v", got)
	}
}

func TestAnUnknownWarningIsStillShown(t *testing.T) {
	got := ClassifyNetlabOutput([]string{"Warning in zzz: Something netlab has never said before"}, false)
	if len(got) != 1 || got[0].Code != "netlab-warning" || got[0].Class != ClassLikelyWrong {
		t.Fatalf("an unmatched warning must surface as likely-wrong, got %+v", got)
	}
}

func TestKubeletTimestampPrefixIsTolerated(t *testing.T) {
	out, _ := os.ReadFile("testdata/findings/spine-no-igp.txt")
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		lines = append(lines, "2026-11-06T12:00:01.123456789Z "+l)
	}
	if got := ClassifyNetlabOutput(lines, false); len(got) != 1 || got[0].Code != "ibgp-without-igp" {
		t.Fatalf("live job lines carry a timestamp prefix: %+v", got)
	}
}

func TestEveryRuleHasACodeClassAndFixAndCodesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range findingRules {
		if r.code == "" || r.why == "" || (r.class != ClassInformational && r.fix == "") {
			t.Errorf("rule %q incomplete", r.code)
		}
		if seen[r.code] {
			t.Errorf("duplicate code %s", r.code)
		}
		seen[r.code] = true
		if FindingClass(r.code) != r.class {
			t.Errorf("FindingClass(%s) mismatch", r.code)
		}
	}
}

// OSPF on the IRB SVI of an EVPN-controlled VLAN (naf-proof E6, live 2026-11-06): the leaves form OSPF adjacencies across their own
// VXLAN tunnel and the fabric flaps every 30-40 s. The requirement: the irb baseline from the proof is flagged once, as likely-wrong
// (never blocking), naming every leaf and the fix that was proven; the proven fix and the two other ways out are not flagged.
func TestOspfOnAnEvpnIrbSviIsFlaggedAndTheProvenFixIsNot(t *testing.T) {
	read := func(name string) []string {
		out, err := os.ReadFile("testdata/findings/" + name + ".txt")
		if err != nil {
			t.Fatal(err)
		}
		return strings.Split(string(out), "\n")
	}
	fs := ClassifyNetlabOutput(read("evpn-svi-ospf-irb"), false)
	if len(fs) != 1 || fs[0].Code != "evpn-svi-ospf-overlay" {
		t.Fatalf("the irb baseline must give exactly one merged finding, got %+v", fs)
	}
	f := fs[0]
	if f.Class != ClassLikelyWrong || f.IsError {
		t.Errorf("class/isError = %s/%v, want likely-wrong compiled-warning (it must never block a deploy)", f.Class, f.IsError)
	}
	if strings.Join(f.Subject, ",") != "l1,l2,l3,l4" {
		t.Errorf("subjects = %v, want every leaf l1..l4", f.Subject)
	}
	for _, must := range []string{"vlans.<name>.ospf: false", "ospf.passive", "mode: bridge"} {
		if !strings.Contains(f.Fix, must) {
			t.Errorf("fix lacks %q: %s", must, f.Fix)
		}
	}
	if !strings.Contains(f.Why, "recursive") || !strings.Contains(f.Message, "l1, l2, l3, l4") {
		t.Errorf("why/message do not explain the cause: %+v", f)
	}
	if FindingClass("evpn-svi-ospf-overlay") != ClassLikelyWrong || !FindingCodeSet()["evpn-svi-ospf-overlay"] {
		t.Error("the code must be in the closed telemetry set with its class")
	}
	for _, clean := range []string{"evpn-svi-ospf-off", "evpn-svi-ospf-bridge", "evpn-svi-ospf-passive"} {
		if got := ClassifyNetlabOutput(read(clean), true); len(got) != 0 {
			t.Errorf("%s must produce no finding, got %+v", clean, got)
		}
	}
}

func TestZonebasedRulesOnADeviceThatDoesNotRenderThemIsClassified(t *testing.T) {
	out, _ := os.ReadFile("testdata/findings/zonebased-rules-fortios.txt")
	fs := ClassifyNetlabOutput(strings.Split(string(out), "\n"), false)
	if len(fs) != 1 || fs[0].Code != "zonebased-rules-not-rendered" {
		t.Fatalf("got %+v", fs)
	}
	f := fs[0]
	if f.Class != ClassLikelyWrong || f.IsError || strings.Join(f.Subject, ",") != "fwA" {
		t.Errorf("class/isError/subject = %s/%v/%v", f.Class, f.IsError, f.Subject)
	}
	if !strings.Contains(f.Fix, "firewall.policy.rules") || !strings.Contains(f.Why, "rendered by only some") {
		t.Errorf("fix/why do not name the rendered path: %+v", f)
	}
	// the same lab on PAN-OS (which renders them) is clean
	clean, _ := os.ReadFile("testdata/findings/zonebased-rules-panos.txt")
	if got := ClassifyNetlabOutput(strings.Split(string(clean), "\n"), false); len(got) != 0 {
		t.Errorf("PAN-OS renders firewall.zonebased.rules; must produce no finding: %+v", got)
	}
}

func TestFabricNearMissNodeNameIsClassified(t *testing.T) {
	out, _ := os.ReadFile("testdata/findings/fabric-near-miss-name.txt")
	fs := ClassifyNetlabOutput(strings.Split(string(out), "\n"), false)
	if len(fs) != 1 || fs[0].Code != "fabric-near-miss-node-name" || strings.Join(fs[0].Subject, ",") != "leaf1,spine1" {
		t.Fatalf("got %+v", fs)
	}
}

func TestNewGuardFindingCodesAreRegisteredAndShown(t *testing.T) {
	for _, c := range []string{"zonebased-rules-not-rendered", "fabric-near-miss-node-name"} {
		if !FindingCodeSet()[c] || FindingClass(c) != ClassLikelyWrong {
			t.Errorf("%s must be a registered likely-wrong code (shown, never blocking): class=%q", c, FindingClass(c))
		}
	}
}
