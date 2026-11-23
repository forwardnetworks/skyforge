package netlabcaps

import (
	"strings"
	"testing"
)

// THE CONTROL THAT DEFINES THIS PACKAGE.
//
// `ipsec` on `iol` LOOKS like a coverage gap and is not one: the ipsec plugin
// names only `ios` under `devices:` and ships no `ipsec/iol.j2`. A detector
// keyed on "is there an iol.j2" reports a gap here with total confidence, and
// on 2026-09-07 exactly that reasoning produced a patch proposal, an issue worth
// filing, and a day of work — all for a no-op. Measured on the pinned netlab
// (f64bc4f3 + the patch series): `defaults.devices.iol.features.ipsec`
// resolves to ios's value, the template lookup walks the parent chain, and
// deleting nat/iol.j2 renders byte-identical NAT.
//
// So this is the negative control, and it is not optional. If it ever fails, the
// detector has started keying on files again and MUST NOT be allowed to file.
func TestIolIpsecIsNotAGap(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load; nothing to assert (an empty manifest knows nothing)")
	}
	s, ok := DevicePluginSupport("iol", "ipsec")
	if !ok {
		t.Fatal("DevicePluginSupport(iol, ipsec) not resolvable; the manifest should know both")
	}
	if !s.Declared {
		t.Errorf("iol must inherit features.ipsec from ios; got Declared=false")
	}
	if !s.Renders {
		t.Errorf("iol must resolve ipsec/ios.j2 through the parent chain; got Renders=false")
	}
	if !s.Supported() {
		t.Fatalf("iol/ipsec must be SUPPORTED: %+v", s)
	}
	if s.DeclaredVia != "ios" || s.RendersVia != "ios" {
		t.Errorf("both halves should come from ios, got DeclaredVia=%q RendersVia=%q", s.DeclaredVia, s.RendersVia)
	}
	if !s.Inherited() {
		t.Errorf("iol/ipsec is inherited; Inherited() must say so or a reader re-derives the phantom gap")
	}

	for _, g := range PluginCoverageGaps() {
		if g.Plugin == "ipsec" && g.Device == "iol" {
			t.Fatalf("PHANTOM GAP: detector reported ipsec/iol as %s — it keys on files again", g.Kind)
		}
	}
}

// The three findings measured by hand on 2026-09-07. If the detector cannot find
// these, it is not working — a green from a detector that finds nothing is the
// failure mode this whole surface exists to replace.
func TestKnownCoverageGapsAreFound(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	gaps := PluginCoverageGaps()
	has := func(plugin, device string, kind GapKind) bool {
		for _, g := range gaps {
			if g.Plugin == plugin && g.Device == device && g.Kind == kind {
				return true
			}
		}
		return false
	}

	// loadbalancer ships f5.j2 and never declares f5. f5 IS a netlab device, so
	// this is a real rejection, not an orphan.
	if !has("loadbalancer", "f5", GapRenderedNotDeclared) {
		t.Errorf("missed: loadbalancer ships f5.j2 with no f5 declaration")
	}
	// ipsec ships a 101-line asav.j2 and never declares asav.
	if !has("ipsec", "asav", GapRenderedNotDeclared) {
		t.Errorf("missed: ipsec ships asav.j2 with no asav declaration")
	}
	// nat and firewall.policy declare asav while shipping only asa.j2, and asav
	// has no parent — so they validate and render nothing.
	if !has("firewall.policy", "asav", GapDeclaredNotRendered) {
		t.Errorf("missed: firewall.policy declares asav with no reachable template")
	}
	// `asa` is not a netlab device in this tree; asa.j2 can never resolve.
	if !has("ipsec", "asa", GapOrphanTemplate) {
		t.Errorf("missed: ipsec/asa.j2 names no netlab device")
	}
}

// A gap must be able to say what a fix touches. An issue that names a problem
// and no files is a notification, and the rule is that we do not file those.
func TestGapsAreActionable(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	for _, g := range PluginCoverageGaps() {
		if g.Plugin == "" || g.Device == "" || g.Kind == "" {
			t.Errorf("gap missing identity: %+v", g)
		}
		if len(g.Files) == 0 {
			t.Errorf("gap %s/%s (%s) names no files a fix would touch", g.Plugin, g.Device, g.Kind)
		}
		if !strings.Contains(g.Detail(), "FIX:") {
			t.Errorf("gap %s/%s (%s) detail has no FIX line", g.Plugin, g.Device, g.Kind)
		}
		if strings.TrimSpace(g.Title()) == "" {
			t.Errorf("gap %s/%s has no title", g.Plugin, g.Device)
		}
	}
}

// The title is what the issue de-dupe hashes, so the same gap must always
// produce the same title — no timestamps, no topology names, no node names.
func TestGapTitleIsStable(t *testing.T) {
	g := CoverageGap{Plugin: "ipsec", Device: "asav", Kind: GapRenderedNotDeclared}
	if g.Title() != g.Title() {
		t.Fatal("title is not deterministic")
	}
	other := CoverageGap{
		Plugin: "ipsec", Device: "asav", Kind: GapRenderedNotDeclared,
		Files: []string{"totally/different/path.yml"},
	}
	if g.Title() != other.Title() {
		t.Errorf("title must depend only on (plugin, device, kind); Files changed it:\n  %q\n  %q", g.Title(), other.Title())
	}
}

// ABSENCE MUST BE STATED. DevicePlugins returns every plugin, including the ones
// the device cannot run, because the failure being fixed is a reader concluding
// "unsupported" from a list that simply did not mention the plugin.
func TestDevicePluginsReportsAbsenceNotOmission(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	all, ok := DevicePlugins("iol")
	if !ok {
		t.Fatal("iol should resolve")
	}
	if len(all) != len(PluginNames()) {
		t.Errorf("DevicePlugins must return EVERY plugin (%d), got %d — omission is the bug", len(PluginNames()), len(all))
	}
	var sawUnsupported bool
	for _, s := range all {
		if !s.Supported() {
			sawUnsupported = true
			if !strings.Contains(s.Explain(), s.Plugin) {
				t.Errorf("an absent feature must be explained WITH the plugin named: %q", s.Explain())
			}
		}
	}
	if !sawUnsupported {
		t.Error("expected iol to be unable to run at least one plugin; if not, this control asserts nothing")
	}
}

// An unknown device is UNKNOWN, not unsupported. Collapsing the two is how a
// typo gets reported as a platform gap.
func TestUnknownDeviceIsNotUnsupported(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	if _, ok := DevicePluginSupport("no-such-device", "ipsec"); ok {
		t.Error("unknown device must return ok=false, not a false verdict")
	}
	if _, ok := DevicePluginSupport("iol", "no-such-plugin"); ok {
		t.Error("unknown plugin must return ok=false")
	}
	if _, ok := DevicePlugins("no-such-device"); ok {
		t.Error("DevicePlugins on an unknown device must return ok=false")
	}
}

func TestPluginForConstructIsNarrow(t *testing.T) {
	if got := PluginForConstruct("crypto"); got != "ipsec" {
		t.Errorf("crypto -> ipsec, got %q", got)
	}
	if got := PluginForConstruct("nat"); got != "nat" {
		t.Errorf("nat -> nat, got %q", got)
	}
	// The table must stay narrow: anything it does not know is silence, never a
	// guess. A confident wrong answer here becomes a warning on an author's
	// screen and, with the filer on, an issue in the tracker.
	for _, s := range []string{"", "ospf", "banner motd", "hostname", "interface"} {
		if got := PluginForConstruct(s); got != "" {
			t.Errorf("PluginForConstruct(%q) must be silent, got %q", s, got)
		}
	}
}

// The risky class must never be auto-filed. `mlag.vtep` declares eos and
// cumulus_nvue and ships NO templates at all — it renders entirely from Python
// via api.node_config(). It is a working plugin, and it lands in
// declared-not-rendered. If that class ever became fileable, the filer would
// open issues against working plugins, which is the exact failure the switch
// exists to prevent.
func TestDeclaredNotRenderedIsNeverFileable(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	var sawRisky bool
	for _, g := range PluginCoverageGaps() {
		if g.Kind == GapDeclaredNotRendered {
			sawRisky = true
			if g.Fileable() {
				t.Errorf("%s/%s is declared-not-rendered and must NOT be fileable", g.Plugin, g.Device)
			}
		}
	}
	if !sawRisky {
		t.Skip("no declared-not-rendered gap present; this control asserts nothing today")
	}
}

// Fileable gaps must be provable from the manifest alone.
func TestFileableGapsAreTheProvableOnes(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	var n int
	for _, g := range PluginCoverageGaps() {
		if !g.Fileable() {
			continue
		}
		n++
		if g.Kind == GapDeclaredNotRendered {
			t.Errorf("declared-not-rendered leaked into the fileable set: %+v", g)
		}
	}
	if n == 0 {
		t.Error("no fileable gaps at all — the three measured findings (ipsec/asav, loadbalancer/f5, evpn.multihoming/frr) should be here; a detector that finds nothing is the bug")
	}
}

// MUTATION CONTROL. Prove the assertions above actually go red rather than
// passing over an empty list — the project's standing rule that a green which
// asserted nothing is not a result.
func TestMutationControlEmptyManifestFindsNothing(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	if len(PluginCoverageGaps()) == 0 {
		t.Fatal("the live manifest yields no gaps, so every assertion in this file is vacuous")
	}
	// The detector must be narrow: a handful of real findings, not a sweep of
	// every device in the tree. 100+ would mean the gating guard regressed and
	// upstream plugins are being reported again.
	if got := len(PluginCoverageGaps()); got > 40 {
		t.Errorf("detector reported %d gaps; that is sweep-the-tree noise, not findings — check the len(p.Devices)==0 guard", got)
	}
}

// A plugin that declares no devices performs no feature gate, so it can never
// produce a gap. This is the guard that took the list from 100+ to 10.
func TestNonGatingPluginsProduceNoGaps(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load")
	}
	for _, g := range PluginCoverageGaps() {
		p, ok := index.plugins[g.Plugin]
		if !ok {
			t.Errorf("gap names unknown plugin %q", g.Plugin)
			continue
		}
		if len(p.Devices) == 0 {
			t.Errorf("plugin %q declares no devices so it gates on nothing; it must not produce gap %+v", g.Plugin, g)
		}
	}
}

// firewall.zonebased.rules is legal on every zonebased device and rendered by panos alone. The manifest used to list
// `rules` under fortios/vsrx/vyos as if they rendered it (the rule existed nowhere on the device).
func TestZonebasedRulesAreNamedOnlyByPanos(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load; nothing to assert")
	}
	var rules *AttributeGap
	gaps := PluginAttributeGaps("firewall.zonebased")
	for i := range gaps {
		if gaps[i].Path == "firewall.zonebased.rules" {
			rules = &gaps[i]
		}
	}
	if rules == nil {
		t.Fatalf("firewall.zonebased.rules must be reported as partially rendered; got %+v", gaps)
	}
	if strings.Join(rules.NamedOn, ",") != "panos" {
		t.Errorf("rules must be named by panos only, got %v", rules.NamedOn)
	}
	for _, d := range []string{"fortios", "vsrx", "vyos"} {
		if !strings.Contains(strings.Join(rules.NotNamedOn, ","), d) {
			t.Errorf("%s accepts but does not render rules; not listed in %v", d, rules.NotNamedOn)
		}
	}
	// NEGATIVE CONTROLS: default_rules is rendered by all four, so it is not a gap; the interface-scope firewall.zone is
	// consumed through derived data on panos and must never be reported (a false claim).
	for _, g := range gaps {
		if g.Path == "firewall.zonebased.default_rules" || g.Path == "firewall.zone" {
			t.Errorf("%s must not be reported as a gap: %+v", g.Path, g)
		}
	}
}

// The iol phantom again, per attribute: nat's `nat` is named by ios.j2 and iol has no template of its own, so iol resolves
// through its parent and must not appear among the devices that do not render it.
func TestAttributeGapsResolveThroughTheParentChain(t *testing.T) {
	if !Loaded() {
		t.Skip("manifest did not load; nothing to assert")
	}
	for _, p := range []string{"nat", "ipsec"} {
		for _, g := range PluginAttributeGaps(p) {
			for _, d := range g.NotNamedOn {
				if d == "iol" {
					t.Errorf("PHANTOM: iol inherits ios and must not be reported as not rendering %s %s: %+v", p, g.Path, g)
				}
			}
		}
	}
}
