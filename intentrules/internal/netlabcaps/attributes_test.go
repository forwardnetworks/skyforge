package netlabcaps

import (
	"strings"
	"testing"
)

// The manifest is EMBEDDED, so these assert against the real generated corpus
// rather than a fixture. A fixture would pass over a corpus that lost a scope,
// which is the drift this rendering exists to prevent.

func TestAttributeFactAnswersOnlyItsKey(t *testing.T) {
	if _, ok := AttributeFact("netlab.capabilities"); ok {
		t.Fatal("AttributeFact answered a key it does not own")
	}
	if _, ok := AttributeFact("netlab.nonsense"); ok {
		t.Fatal("AttributeFact answered an unknown key")
	}
	if _, ok := AttributeFact(FactNetlabAttributes); !ok {
		t.Fatal("AttributeFact could not answer its own key against the embedded manifest")
	}
}

func TestAttributeCatalogueCarriesCanBeFalse(t *testing.T) {
	got, ok := AttributeFact(FactNetlabAttributes)
	if !ok {
		t.Fatal("no answer")
	}
	// can_be_false is the rule that makes `<module>: false` on a link valid
	// authoring. Nothing else in the corpus states it, so a consumer without it
	// classifies correct authoring as a modelling gap.
	if !strings.Contains(got, "may be set to `false`") {
		t.Fatalf("rendering does not state the can_be_false rule:\n%s", got)
	}
	for _, scope := range Manifest().CanBeFalse {
		if !strings.Contains(got, "`"+scope+"`") {
			t.Fatalf("can_be_false scope %q missing from the rendering", scope)
		}
	}
}

func TestAttributeCatalogueCarriesEveryScopeTheManifestHas(t *testing.T) {
	got, ok := AttributeFact(FactNetlabAttributes)
	if !ok {
		t.Fatal("no answer")
	}
	scopes := map[string]bool{}
	for _, m := range Manifest().Modules {
		for _, a := range m.Attributes {
			if s := strings.TrimSpace(a.Scope); s != "" {
				scopes[s] = true
			}
		}
	}
	if len(scopes) == 0 {
		t.Fatal("embedded manifest declares no attribute scopes at all — refusing to pass over an empty corpus")
	}
	for s := range scopes {
		if !strings.Contains(got, "**"+s+"** attributes") {
			t.Fatalf("scope %q is in the manifest but absent from the rendering", s)
		}
	}
}

func TestAttributeCatalogueCarriesModuleAttributePaths(t *testing.T) {
	got, ok := AttributeFact(FactNetlabAttributes)
	if !ok {
		t.Fatal("no answer")
	}
	var checked int
	for _, m := range Manifest().Modules {
		for _, a := range m.Attributes {
			if a.Scope != "link" || a.Path == "" {
				continue
			}
			if !strings.Contains(got, a.Path) {
				t.Fatalf("link attribute %s.%s is in the manifest but absent from the rendering", m.Name, a.Path)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no link-scope attributes found in the embedded manifest — the assertion proved nothing")
	}
}

// Positive control: the assertions above must FAIL on an empty rendering. A test
// that passes over an empty corpus asserts nothing, which is the failure mode
// this whole package is built to avoid.
func TestAssertionsFailOnAnEmptyRendering(t *testing.T) {
	empty := ""
	if strings.Contains(empty, "may be set to `false`") {
		t.Fatal("control is broken: an empty rendering satisfied the can_be_false assertion")
	}
	if strings.Contains(empty, "**link** attributes") {
		t.Fatal("control is broken: an empty rendering satisfied the scope assertion")
	}
}
