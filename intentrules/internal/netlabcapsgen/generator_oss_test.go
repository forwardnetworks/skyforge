package netlabcapsgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readOSSManifest(t *testing.T) Manifest {
	t.Helper()
	blob, err := os.ReadFile(filepath.Join("..", "netlabcaps", "netlab_capabilities_oss.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(blob, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// The requirement: nothing of a private-only netlab patch is in the public
// manifest, and the public surface that IS there is still the real one.
func TestOSSManifestHasNoPrivateSurface(t *testing.T) {
	m := readOSSManifest(t)
	devices := map[string]bool{}
	for _, d := range m.Devices {
		devices[d.Name] = true
	}
	plugins := map[string]bool{}
	for _, p := range m.Plugins {
		plugins[p.Name] = true
	}
	for _, d := range []string{"gobgp", "gobgp-speaker"} {
		if devices[d] {
			t.Errorf("public manifest lists the private device %q", d)
		}
	}
	for _, p := range []string{"wireless", "firewall.geneve", "skyforge_collectors"} {
		if plugins[p] {
			t.Errorf("public manifest lists the private plugin %q", p)
		}
	}
	if len(m.SkyforgeGlobals) != 1 || m.SkyforgeGlobals[0].Name != "otg" {
		t.Errorf("public manifest must carry exactly the otg block, got %+v", m.SkyforgeGlobals)
	}
	blob, _ := json.Marshal(m)
	for _, w := range []string{"gobgp", "geneve", "zscaler", "sase", "wireless", "mist"} {
		if strings.Contains(strings.ToLower(string(blob)), w) {
			t.Errorf("public manifest still mentions %q", w)
		}
	}
	// POSITIVE CONTROL: the same lookups find the public surface, so the absences
	// above are not an empty manifest.
	for _, d := range []string{"eos", "iol", "panos", "a10", "keysight_ixia-c-controller"} {
		if !devices[d] {
			t.Errorf("public manifest lost the public device %q", d)
		}
	}
	for _, p := range []string{"nat", "firewall.policy", "firewall.zonebased", "bgp.session"} {
		if !plugins[p] {
			t.Errorf("public manifest lost the public plugin %q", p)
		}
	}
}

// OSS mode reads only the blocks the published series carries.
func TestOSSReaderRequiresOnlyTheOpenBlocks(t *testing.T) {
	dir := t.TempDir()
	modules := filepath.Join(dir, "netsim", "modules")
	if err := os.MkdirAll(modules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "netsim", "defaults"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		if err := os.WriteFile(filepath.Join(dir, "netsim", "defaults", "attributes.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("global:\n  cloud:\n    scenario: {type: str}\n")
	if _, err := readSkyforgeGlobals(modules, true); err == nil || !strings.Contains(err.Error(), "otg") {
		t.Fatalf("a tree without otg must be refused in OSS mode and name it; got %v", err)
	}
	write("global:\n  otg:\n    controller: {type: str}\n  cloud:\n    scenario: {type: str}\n")
	got, err := readSkyforgeGlobals(modules, true)
	if err != nil || len(got) != 1 || got[0].Name != "otg" {
		t.Fatalf("OSS mode must return only otg even if the tree also has cloud; got %+v, %v", got, err)
	}
}

// The committed public manifest must be what the generator produces from upstream
// netlab with the published series applied. The tree is not in this module, so it
// is supplied: NETLAB_PUBLIC_TREE=<dir of that applied tree>. Without it this test
// says it did NOT check, rather than passing.
func TestOSSManifestMatchesAPublicTree(t *testing.T) {
	tree := os.Getenv("NETLAB_PUBLIC_TREE")
	if tree == "" {
		t.Skip("NETLAB_PUBLIC_TREE not set: the public manifest was NOT checked against a patched public tree")
	}
	got, err := Generate(Options{
		ModulesDir:  filepath.Join(tree, "netsim", "modules"),
		PluginsDir:  filepath.Join(tree, "netsim", "extra"),
		DevicesDir:  filepath.Join(tree, "netsim", "devices"),
		MarkersFile: markersFile(t),
		OSS:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := readOSSManifest(t)
	got.GeneratedAt, want.GeneratedAt = "", ""
	g, _ := Encode(got)
	w, _ := Encode(&want)
	if string(g) != string(w) {
		t.Fatalf("netlab_capabilities_oss.json is STALE against the public tree %s (got %d devices / %d plugins, committed %d / %d); "+
			"regenerate it with the generator in OSS mode from that tree",
			tree, len(got.Devices), len(got.Plugins), len(want.Devices), len(want.Plugins))
	}
}
