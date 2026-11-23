package netlabref

import (
	"strings"
	"testing"
)

// The question that started this work, asked the way an SE asked it on
// 2026-09-03: where do I read link-attribute syntax?
func TestDocsAnswersTheLinkAttributeQuestion(t *testing.T) {
	if hits := Docs("link attribute", ""); !strings.Contains(hits, "links.md") {
		t.Fatalf("search for link attributes did not surface links.md: %s", hits)
	}
	body := Docs("", "links.md")
	if !strings.Contains(body, "dictionary format of the link definition") {
		t.Fatal("links.md content missing the dictionary-format rule, which IS the answer")
	}
	// The corpus must also reach the examples links.md points at.
	if hits := Docs("link definition example", ""); !strings.Contains(hits, "example/link-definition.md") {
		t.Fatalf("the linked examples page is unreachable: %s", hits)
	}
}

// A miss must name what exists; an empty search must not read as "netlab does
// not document this". Both are the broken-reader failure in message form.
func TestDocsMissesAreInformativeNotSilent(t *testing.T) {
	miss := Docs("", "module/eigrp")
	if !strings.Contains(miss, "no such netlab doc") || !strings.Contains(miss, "eigrp") {
		t.Fatalf("a miss must name near matches; got: %s", miss)
	}
	none := Docs("zzz-not-a-real-netlab-term", "")
	if !strings.Contains(none, "no netlab doc mentions") || !strings.Contains(none, "corpus has") {
		t.Fatalf("empty search must say the corpus was consulted; got: %s", none)
	}
	// POSITIVE CONTROL: a real read must succeed, or the above proves only that
	// everything fails.
	if body := Docs("", "module/eigrp.md"); !strings.Contains(body, "eigrp") {
		t.Fatalf("a valid doc path failed: %s", body)
	}
}

// A common authoring idiom: `eigrp: false` on a link is CORRECT netlab. If the manifest
// cannot say so, a gap classifier would file an issue against valid authoring.
func TestCapabilitiesReportsTheModuleFalseIdiom(t *testing.T) {
	out := Capabilities("eigrp", "")
	if !strings.Contains(out, "`eigrp: false` is legal at scope") {
		t.Fatalf("the <module>: false idiom is not reported: %s", out)
	}
	for _, want := range []string{"scope global", "scope link", "scope node"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in: %s", want, out)
		}
	}
	// The ten framework switches must never be presented as authoring targets.
	for _, bad := range []string{"scope route_map", "scope lag_no_propagate", "scope keep_subif", "scope phy_ifattr"} {
		if strings.Contains(out, bad) {
			t.Fatalf("framework control key presented as a scope (%q): %s", bad, out)
		}
	}
	// can_be_true is per module and must show where it applies.
	if mc := Capabilities("multicast", ""); !strings.Contains(mc, "`multicast: true` is legal at scope") {
		t.Fatalf("can_be_true not reported for multicast: %s", mc)
	}
}

// Gary's question: can this device actually do it? Feature granularity, plus the
// honest limit that a declaration is not a promise about an image.
func TestCapabilitiesReportsDeviceFeaturesAndTheirLimit(t *testing.T) {
	out := Capabilities("", "eos")
	if !strings.Contains(out, "mpls.vpn") {
		t.Fatalf("per-feature device support missing: %s", out)
	}
	if !strings.Contains(out, "per device TYPE, not per image") {
		t.Fatal("the declaration-vs-reality limit must be stated, or a caller reads a " +
			"declared feature as a promise about the image it runs")
	}
	if u := Capabilities("", "not-a-device"); !strings.Contains(u, "no netlab device") {
		t.Fatalf("unknown device not reported: %s", u)
	}
	if u := Capabilities("not-a-module", ""); !strings.Contains(u, "no netlab module") {
		t.Fatalf("unknown module not reported: %s", u)
	}
}

// Bare calls must orient rather than dump or return nothing.
func TestBareCallsOrient(t *testing.T) {
	if out := Capabilities("", ""); !strings.Contains(out, "modules") || !strings.Contains(out, "eigrp") {
		t.Fatalf("bare capabilities call unhelpful: %s", out)
	}
	if out := Docs("", ""); !strings.Contains(out, "links.md") {
		t.Fatalf("bare docs call did not list the corpus: %s", out)
	}
}

// THE ANSWER THAT WAS MISSING. `netlab_capabilities module=ipsec` used to say
// "no netlab module named ipsec" and list the modules — which reads as "this
// platform cannot do IPsec". It is a plugin, it exists, and an author who got
// that answer hand-wrote Cisco crypto CLI into three nodes instead.
func TestPluginAskedAsAModuleAnswersAsAPlugin(t *testing.T) {
	out := Capabilities("ipsec", "")
	if strings.Contains(out, "no netlab module named") {
		t.Fatalf("a plugin asked for as a module must be answered, not reported missing:\n%s", out)
	}
	if !strings.Contains(out, "plugin ipsec") {
		t.Errorf("must identify it as a plugin:\n%s", out)
	}
	if !strings.Contains(out, "`plugin: [ ipsec ]`") {
		t.Errorf("must show where the name actually goes:\n%s", out)
	}
	if !strings.Contains(out, "iol") {
		t.Errorf("must name iol among the devices that render it — that was the author's device:\n%s", out)
	}
}

// A genuinely unknown name must STILL be reported unknown. Without this the test
// above could pass on a renderer that never says "no such thing".
func TestUnknownNameIsStillUnknown(t *testing.T) {
	out := Capabilities("definitely-not-a-thing", "")
	if !strings.Contains(out, "no netlab module named") {
		t.Errorf("an unknown name must be reported as unknown:\n%s", out)
	}
}

// ABSENCE STATED, NOT OMITTED: a device query must talk about plugins at all.
func TestDeviceQueryReportsPlugins(t *testing.T) {
	out := Capabilities("", "iol")
	if !strings.Contains(out, "plugins this device RENDERS") {
		t.Fatalf("a device query must state its plugin coverage; silence is what caused this:\n%s", out)
	}
	if !strings.Contains(out, "ipsec") {
		t.Errorf("iol renders ipsec through ios and must say so:\n%s", out)
	}
	if !strings.Contains(out, "it is not omitted, it is absent") {
		t.Errorf("the answer must say that unlisted plugins are absent, not merely unmentioned:\n%s", out)
	}
}

// THE PHANTOM-GAP CONTROL, at the rendering layer. Seven working upstream
// plugins (bgp.policy, bgp.session, ospf.areas, tunnel.gre …) were reported to
// the author as PLATFORM GAPS on iol before the renderer used the provable
// condition. Telling an author to file those wastes their time and the tracker.
func TestDeviceQueryDoesNotInventGaps(t *testing.T) {
	out := Capabilities("", "iol")
	if strings.Contains(out, "REJECT this device") {
		t.Errorf("iol has no rejecting plugin; the renderer must not invent gaps:\n%s", out)
	}
	for _, working := range []string{"bgp.policy", "bgp.session", "ospf.areas", "tunnel.gre"} {
		idx := strings.Index(out, "plugins that REJECT")
		if idx >= 0 && strings.Contains(out[idx:], working) {
			t.Errorf("%s is a working upstream plugin on iol and must not be listed as a gap:\n%s", working, out)
		}
	}
}

// The one real gap must still be found, or the control above is vacuous.
func TestRealGapIsStillReported(t *testing.T) {
	out := Capabilities("ipsec", "")
	if !strings.Contains(out, "asav") {
		t.Errorf("ipsec ships asav.j2 and does not declare asav; that gap must be reported:\n%s", out)
	}
	if !strings.Contains(out, "PLATFORM GAP") {
		t.Errorf("the gap must be named as a platform gap so the author files it:\n%s", out)
	}
}

// A designer or an assistant reads this text to decide what to offer. firewall.zonebased.rules is accepted on fortios, vsrx
// and vyos and rendered by panos only; the answer must say so instead of leaving `rules` looking supported everywhere.
func TestPluginAnswerWarnsAboutAttributesOnlySomeDevicesRender(t *testing.T) {
	out := Capabilities("firewall.zonebased", "")
	if !strings.Contains(out, "CAUTION firewall.zonebased.rules is named only by the panos template(s), not by fortios") {
		t.Errorf("must warn that rules is rendered by panos only:\n%s", out)
	}
	if strings.Contains(out, "CAUTION firewall.zonebased.default_rules") {
		t.Errorf("default_rules renders on every zonebased device and must not carry a caution:\n%s", out)
	}
}
