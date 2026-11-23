package netlabcaps

import (
	"regexp"
	"strings"
	"testing"
)

// The public manifest carries the otg block as its own section and none of the
// private blocks; the accessor must not resolve a private block name.
func TestOSSGlobalsAreOtgOnly(t *testing.T) {
	got, ok := AttributeFact(FactNetlabAttributes)
	if !ok {
		t.Fatal("manifest did not load")
	}
	for _, want := range []string{"- otg: ", "flows[].tx"} {
		if !strings.Contains(got, want) {
			t.Errorf("attribute fact lacks %q", want)
		}
	}
	for _, name := range []string{"cloud", "wireless", "forward", "sase"} {
		if _, ok := SkyforgeGlobal(name); ok {
			t.Errorf("SkyforgeGlobal(%q) resolves in the public build", name)
		}
		if strings.Contains(got, "- "+name+": ") {
			t.Errorf("attribute fact lists the private block %q", name)
		}
	}
	if g, ok := SkyforgeGlobal("otg"); !ok || len(g.Attributes) == 0 {
		t.Errorf("SkyforgeGlobal(otg) must answer in the public build, ok=%v", ok)
	}
	for _, n := range []string{"gobgp", "gobgp-speaker"} {
		if HasDevice(n) {
			t.Errorf("device %q is known to the public build", n)
		}
	}
	if HasPlugin("wireless") || HasPlugin("firewall.geneve") || HasPlugin("skyforge_collectors") {
		t.Error("a private plugin is known to the public build")
	}
	// POSITIVE CONTROL: the same predicates say yes for public names.
	if !HasPlugin("nat") {
		t.Error("public plugin nat is unknown: the absences above prove nothing")
	}
}

// The catalogue's closing advice is the neutral text in the public build.
func TestOSSCatalogueAdviceIsNeutral(t *testing.T) {
	got := capabilityCatalogue()
	if !strings.Contains(got, "report the gap; do NOT invent a name") {
		t.Fatalf("catalogue lacks the neutral advice: %s", got)
	}
	if regexp.MustCompile(`[0-9]+-patch`).MatchString(got) {
		t.Fatal("catalogue still states the size of the private patch series")
	}
	if strings.Contains(got, "file a gap issue") {
		t.Fatal("catalogue still tells the reader to file a gap issue")
	}
}
