package netlabref

import (
	"os"
	"strings"
	"testing"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
)

// These tests are derived from the requirement, not from the implementation: a topology that enables a plugin must
// be told netlab's own declared types for that plugin, and one that does not must be told nothing. The plugin used is
// `fabric`, whose counts are integers (defaults.yml: leafs/spines `type: int, _required: True`): the exact shape an
// author got wrong by writing mappings.

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSchemaIsReturnedForAnEnabledPlugin(t *testing.T) {
	if !netlabcaps.Loaded() {
		t.Fatal("the real capability manifest did not load: nothing below would prove anything")
	}
	got := SchemasForTopology(readTestdata(t, "fabric_five_stage_dict_counts.yml"))
	if len(got) != 1 || got[0].Plugin != "fabric" {
		t.Fatalf("want exactly the fabric schema, got %+v", got)
	}
	s := got[0]
	if !s.Known {
		t.Fatalf("fabric is a real plugin: %+v", s)
	}
	want := map[string]bool{"fabric.leafs": false, "fabric.spines": false}
	for _, a := range s.Attributes {
		if _, ok := want[a.Path]; ok {
			if a.Type != "int" || !a.Required {
				t.Errorf("%s: type %q required %v; netlab declares int and _required", a.Path, a.Type, a.Required)
			}
			want[a.Path] = true
		}
	}
	for p, seen := range want {
		if !seen {
			t.Errorf("schema for fabric is missing %s: %+v", p, s.Attributes)
		}
	}
	if s.Doc != "plugins/fabric.md" {
		t.Errorf("doc pointer = %q, want plugins/fabric.md (the corpus has it)", s.Doc)
	}
}

func TestSchemaIsAbsentWhenNoPluginIsEnabled(t *testing.T) {
	for name, y := range map[string]string{
		"no plugin key": "name: x\nnodes: [r1, r2]\nlinks: [r1-r2]\n",
		"empty list":    "name: x\nplugin: []\n",
		"not yaml":      "{{{",
	} {
		if got := SchemasForTopology(y); got != nil {
			t.Errorf("%s: want no schema, got %+v", name, got)
		}
	}
}

func TestPluginKeyShapes(t *testing.T) {
	if got := EnabledPlugins("plugin: fabric\n"); len(got) != 1 || got[0] != "fabric" {
		t.Errorf("scalar form: %v", got)
	}
	if got := EnabledPlugins("plugin: [ fabric, files, fabric ]\n"); strings.Join(got, ",") != "fabric,files" {
		t.Errorf("list form with a duplicate: %v", got)
	}
}

func TestAnUnknownPluginIsReportedUnknownNotEmpty(t *testing.T) {
	got := SchemasForTopology("plugin: [ fabrik ]\n")
	if len(got) != 1 || got[0].Known || got[0].Note == "" || len(got[0].Attributes) != 0 {
		t.Fatalf("a misspelled plugin must be flagged unknown, got %+v", got)
	}
}

func TestCapabilitiesForAPluginListsItsDeclaredTypes(t *testing.T) {
	out := Capabilities("fabric", "")
	for _, must := range []string{"fabric.leafs (int, required)", "fabric.spines (int, required)", "netlab_docs path=plugins/fabric.md"} {
		if !strings.Contains(out, must) {
			t.Errorf("netlab_capabilities module=fabric lacks %q:\n%s", must, out)
		}
	}
	// fabric neither gates on a device nor ships per-device templates; saying "no device can run it" would be false.
	if strings.Contains(out, "no device can currently run it") {
		t.Errorf("a topology-expanding plugin was reported as runnable on no device:\n%s", out)
	}
}
