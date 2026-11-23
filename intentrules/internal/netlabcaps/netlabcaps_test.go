package netlabcaps

import (
	"strings"
	"testing"
)

// TestManifestLoads is the floor: an embed that failed, or a manifest that
// parsed to nothing, must not read as "this platform has no capabilities". Every
// consumer gates on Loaded(), so if this is wrong they all silently do nothing.
func TestManifestLoads(t *testing.T) {
	if !Loaded() {
		t.Fatal("the embedded capability manifest did not load — every consumer degrades to knowing nothing")
	}
	if n := len(ModuleNames()); n < 20 {
		t.Errorf("only %d modules in the manifest; the patched tree has 22 — a short manifest is a broken one", n)
	}
	if n := len(PluginNames()); n < 25 {
		t.Errorf("only %d plugins in the manifest; the patched tree has 32", n)
	}
	if n := len(DeviceNames()); n < 40 {
		t.Errorf("only %d devices in the manifest; the patched tree has 50", n)
	}
}

// TestKnownNamesExist pins the names the corpus and our own checks rely on. If a
// patch renames one, this fails HERE with the name, rather than silently
// disarming a check somewhere else.
func TestKnownNamesExist(t *testing.T) {
	for _, m := range []string{"mpls", "evpn", "vxlan", "sr", "vrf", "bgp", "ospf", "isis", "sdwan"} {
		if !HasModule(m) {
			t.Errorf("module %q missing from the manifest", m)
		}
	}
	for _, p := range []string{"nat", "firewall.policy", "ospf.areas", "tunnel.gre", "tunnel.wireguard", "bgp.session", "bgp.policy", "multilab"} {
		if !HasPlugin(p) {
			t.Errorf("plugin %q missing from the manifest", p)
		}
	}
	// Three plugin names USED to be the finding: an example topology named them and
	// the tree had none of them. The patch series has since ADDED all three, this
	// assertion fired exactly as it was built to, and the finding is closed. They
	// are asserted PRESENT now, for the same reason every other name here is --
	// so a rename fails with the name rather than silently disarming a consumer.
	for _, p := range []string{"tunnel.dmvpn", "ospf.instances", "bgp.rr_group"} {
		if !HasPlugin(p) {
			t.Errorf("plugin %q missing from the manifest", p)
		}
	}
	for _, d := range []string{"eos", "iol", "iosxr", "panos", "linux"} {
		if !HasDevice(d) {
			t.Errorf("device %q missing from the manifest", d)
		}
	}
}

// TestValidValuesComeFromTheManifest: the EVPN transport enum is READ, not
// written. This is the fact the intent checker uses to decide which module a
// fabric's encapsulation needs.
func TestValidValuesComeFromTheManifest(t *testing.T) {
	got := ValidValues("evpn", "transport")
	if strings.Join(got, ",") != "vxlan,mpls,sr" {
		t.Fatalf("evpn.transport valid values = %v, want [vxlan mpls sr]", got)
	}
	if got := ValidValues("sr", "protocol"); strings.Join(got, ",") != "isis,ospfv2" {
		t.Fatalf("sr.protocol valid values = %v, want [isis ospfv2]", got)
	}
	// An attribute with no declared enum returns nothing — and that means
	// "netlab declares no enum", never "any value is fine".
	if got := ValidValues("mpls", "ldp.igp_sync"); len(got) != 0 {
		t.Errorf("mpls.ldp.igp_sync has no enum; got %v", got)
	}
}

// TestModuleRequiresIsPublished: netlab REFUSES evpn without bgp, so a checker
// must be able to see that and decline to duplicate the error.
func TestModuleRequiresIsPublished(t *testing.T) {
	if got := ModuleRequires("evpn"); strings.Join(got, ",") != "bgp" {
		t.Fatalf("evpn requires = %v, want [bgp]", got)
	}
}

func TestNearestNamesSuggestsTheRealOnes(t *testing.T) {
	cases := map[string]string{
		"tunnel.dmvpn":   "tunnel.gre",
		"ospf.instances": "ospf.areas",
		"bgp.rr_group":   "bgp.session",
	}
	for wrong, wantOne := range cases {
		near := NearestNames(wrong, PluginNames())
		found := false
		for _, n := range near {
			if n == wantOne {
				found = true
			}
		}
		if !found {
			t.Errorf("NearestNames(%q) = %v, expected it to include %q", wrong, near, wantOne)
		}
	}
	if got := NearestNames("", PluginNames()); len(got) != 0 {
		t.Errorf("an empty name must suggest nothing, got %v", got)
	}
}

// TestCapabilityFactNamesWhatExists: the fact a skill corpus splices
// must carry the real lists and must warn off the three names that do not exist.
func TestCapabilityFactNamesWhatExists(t *testing.T) {
	d, ok := Fact(FactNetlabCapabilities)
	if !ok {
		t.Fatal("the capability fact must resolve when the manifest loaded")
	}
	for _, want := range []string{"mpls", "evpn", "sdwan", "firewall.policy", "ospf.areas", "tunnel.gre", "tunnel.dmvpn", "PATCHED", "vxlan/mpls/sr"} {
		if !strings.Contains(d, want) {
			t.Errorf("capability fact does not mention %q", want)
		}
	}
	// It answers ONE key. Anything else must report false so the skill renders its
	// unavailable marker instead of an empty section.
	if _, ok := Fact("netlab.devices"); ok {
		t.Error("this package must not claim to answer the per-device fact keys")
	}
}

func TestKneServicePortsComeFromTheManifest(t *testing.T) {
	if ports, known := KneServicePorts("fortios"); !known || len(ports) < 3 {
		t.Fatalf("fortios ports=%v known=%v", ports, known)
	}
	if web, known := HasWebUI("panos"); !known || !web {
		t.Fatal("panos must have a web UI")
	}
	if web, known := HasWebUI("fortios"); !known || !web {
		t.Fatal("fortios has a web UI on http/80 (web_ui_overrides.json)")
	}
	if web, known := HasWebUI("linux"); !known || web {
		t.Fatalf("linux must be KNOWN with no web UI (web=%v known=%v)", web, known)
	}
	if web, known := HasWebUI("not-a-device"); known || web {
		t.Fatal("an unknown device must be unknown, not 'no web UI'")
	}
}

// withWebUIOverride installs a synthetic override for the test and restores the
// committed table afterwards.
func withWebUIOverride(t *testing.T, device string, o webUIOverride) {
	t.Helper()
	table := loadWebUIOverrides()
	prev, had := table[device]
	table[device] = o
	t.Cleanup(func() {
		if had {
			table[device] = prev
		} else {
			delete(table, device)
		}
	})
}

func TestWebUIEndpointDefaultsAndFortiOSOverride(t *testing.T) {
	for _, d := range []string{"panos", "eos", "vsrx", "linux", "not-a-device", ""} {
		if s, p := WebUIEndpoint(d); s != "https" || p != 443 {
			t.Errorf("%q = %s/%d, want https/443", d, s, p)
		}
	}
	if s, p := WebUIEndpoint("fortios"); s != "http" || p != 80 {
		t.Fatalf("fortios = %s/%d, want http/80", s, p)
	}
	if s, p := WebUIEndpoint(" FortiOS "); s != "http" || p != 80 {
		t.Fatalf("fortios key must be case/space-insensitive, got %s/%d", s, p)
	}
	if r := WebUIUnsupportedReason("fortios"); r != "" {
		t.Fatalf("fortios is supported, reason=%q", r)
	}
}

func TestWebUIEndpointSchemeOnlyOverrideTakesTheSchemeDefaultPort(t *testing.T) {
	withWebUIOverride(t, "eos", webUIOverride{Supported: true, Scheme: "http"})
	if s, p := WebUIEndpoint("eos"); s != "http" || p != 80 {
		t.Fatalf("scheme-only http = %s/%d, want http/80", s, p)
	}
	withWebUIOverride(t, "eos", webUIOverride{Supported: true, Scheme: "http", Port: 8080})
	if s, p := WebUIEndpoint("eos"); s != "http" || p != 8080 {
		t.Fatalf("explicit port = %s/%d, want http/8080", s, p)
	}
	withWebUIOverride(t, "eos", webUIOverride{Supported: true, Scheme: "gopher", Port: 70000})
	if s, p := WebUIEndpoint("eos"); s != "https" || p != 443 {
		t.Fatalf("invalid scheme/port must be ignored, got %s/%d", s, p)
	}
}

func TestHasWebUIFollowsTheOverridePort(t *testing.T) {
	// The kind's service map carries 80 and 443 but not 9999: a web UI declared
	// there can never be dialled, so the action must not be offered.
	withWebUIOverride(t, "fortios", webUIOverride{Supported: true, Scheme: "http", Port: 9999})
	if web, known := HasWebUI("fortios"); !known || web {
		t.Fatalf("fortios with a port its service map lacks must not be offered (web=%v known=%v)", web, known)
	}
	withWebUIOverride(t, "fortios", webUIOverride{Supported: true, Scheme: "https"})
	if web, known := HasWebUI("fortios"); !known || !web {
		t.Fatalf("fortios on https/443 is exposed by the map (web=%v known=%v)", web, known)
	}
	// a kind whose map has no 80 at all
	withWebUIOverride(t, "panos", webUIOverride{Supported: true, Scheme: "http"})
	if exposed, _ := KneServiceExposes("panos", 80); exposed {
		t.Skip("panos now exposes 80; pick another control kind")
	}
	if web, known := HasWebUI("panos"); !known || web {
		t.Fatalf("panos has no port 80: not offered (web=%v known=%v)", web, known)
	}
}

func TestUnsupportedOverrideStillWinsOverPort(t *testing.T) {
	withWebUIOverride(t, "fortios", webUIOverride{Supported: false, Reason: "nope", Scheme: "http", Port: 80})
	if web, known := HasWebUI("fortios"); !known || web {
		t.Fatal("supported=false must hide the action")
	}
	if WebUIUnsupportedReason("fortios") != "nope" {
		t.Fatal("reason lost")
	}
}

func TestWebUIPathPrefixRewriteIsDataDriven(t *testing.T) {
	rw := WebUIPathPrefixRewrite("FortiOS ")
	if rw == nil || len(rw.HTMLLiterals) == 0 || len(rw.ScriptLiterals) == 0 || len(rw.ScriptReplacements) == 0 || len(rw.StripPrefixQueryParams) == 0 {
		t.Fatalf("fortios rewrite data incomplete: %+v", rw)
	}
	for _, r := range rw.ScriptReplacements {
		if r.Old == "" || r.Old == r.New {
			t.Fatalf("a replacement must change something: %+v", r)
		}
	}
	// Mutating the returned copy must not change the loaded data.
	rw.HTMLLiterals[0] = "/mutated"
	if WebUIPathPrefixRewrite("fortios").HTMLLiterals[0] == "/mutated" {
		t.Fatal("WebUIPathPrefixRewrite returns shared slices")
	}
	for _, dev := range []string{"panos", "eos", "unknown-device", ""} {
		if WebUIPathPrefixRewrite(dev) != nil {
			t.Fatalf("%q must have no rewrite", dev)
		}
	}
}
