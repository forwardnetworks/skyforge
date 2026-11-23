package netlabcapsgen

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// generator_test.go carries the two things this generator has to prove, and one
// of them is not about the code at all.
//
//  1. IT REFUSES A PRISTINE TREE. The equivalent generator next door
//     (netlabdefaultsgen) once ran against unpatched vendor/netlab, exited 0, and
//     silently dropped a device from the catalog. Missing input, clean exit,
//     wrong answer. So the refusal gets a positive control: a run that SHOULD
//     fail, proven to fail.
//
//  2. THE COMMITTED MANIFEST MATCHES THE PATCHED TREE. That is the staleness
//     risk, and it is the one nothing else in this repo covers for this
//     artifact. See TestManifestIsNotStale.

// markersFile is the checked-in marker list. It may not be present where the tests run, so a test
// that needs it skips rather than fails when it is absent.
func markersFile(t *testing.T) string {
	t.Helper()
	p := markersPath()
	if _, err := os.Stat(p); p == "" || err != nil {
		t.Skip("the checked-in marker list is not available — this was NOT exercised")
	}
	return p
}

func repoRoot(t *testing.T) string {
	t.Helper()
	// internal/netlabcapsgen -> components/server -> components -> <root>
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	return root
}

// TestRefusesAPristineNetlabTree is the POSITIVE CONTROL on the refusal: point
// the generator at the unpatched vendor tree and it must fail, naming what is
// missing. A refusal that has never been observed refusing is not a refusal.
func TestRefusesAPristineNetlabTree(t *testing.T) {
	root := repoRoot(t)
	pristine := filepath.Join(root, "vendor", "netlab", "netsim")
	if _, err := os.Stat(pristine); err != nil {
		t.Skip("no pristine upstream netlab tree is available — the refusal was NOT exercised")
	}
	// Confirm the tree really is unpatched before asserting on it: if somebody
	// leaves a patched checkout here, a pass would prove nothing.
	if _, err := os.Stat(filepath.Join(pristine, "extra", "nat", "eos.j2")); err == nil {
		t.Skip("the netlab tree found is already patched — the refusal was NOT exercised")
	}

	_, err := Generate(Options{
		ModulesDir:  filepath.Join(pristine, "modules"),
		PluginsDir:  filepath.Join(pristine, "extra"),
		DevicesDir:  filepath.Join(pristine, "devices"),
		MarkersFile: markersFile(t),
	})
	if err == nil {
		t.Fatal("generating from a PRISTINE netlab tree must fail — a manifest built from unpatched upstream describes a platform we do not run")
	}
	for _, want := range []string{"NOT patched", "extra/nat/eos.j2", "apply the patch series"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal message does not contain %q; it must say what is missing and how to fix it.\ngot: %v", want, err)
		}
	}
}

// TestMarkerFileIsItselfChecked: an empty marker list is an assertion that
// asserts nothing, and must not be accepted.
func TestMarkerFileMustDeclareMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "markers.json")
	if err := os.WriteFile(path, []byte(`{"markers":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readMarkers(path); err == nil {
		t.Fatal("a marker file with no markers must be rejected")
	}
}

// TestPluginScanDescendsIntoANamespaceThatIsAlsoAPlugin pins the trap that made
// `tunnel.dmvpn` stop being reported: netsim/extra/tunnel has an __init__.py of
// its own AND holds gre and wireguard. A scan that stopped at the parent lost
// both children, and losing them silently disarmed the checker for that whole
// namespace.
func TestPluginScanDescendsIntoANamespaceThatIsAlsoAPlugin(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string, files ...string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(full, f), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mk("tunnel", "__init__.py")
	mk("tunnel/gre", "__init__.py", "defaults.yml")
	mk("tunnel/wireguard", "__init__.py")
	mk("bgp") // a bare namespace directory: not a plugin
	mk("bgp/policy", "__init__.py", "defaults.yml")
	mk("nat", "defaults.yml")
	mk("none") // neither marker: not a plugin

	plugins, err := readPlugins(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range plugins {
		got = append(got, p.Name)
	}
	want := "bgp.policy,nat,tunnel,tunnel.gre,tunnel.wireguard"
	if strings.Join(got, ",") != want {
		t.Fatalf("plugins = %v, want %s", got, want)
	}
}

// TestPluginWithoutDefaultsYmlIsStillAPlugin: defaults.yml is OPTIONAL (it only
// declares per-device support). Treating it as the existence marker dropped
// `multilab` and `check.config`, both used by blueprints in the corpus.
func TestPluginWithoutDefaultsYmlIsStillAPlugin(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "multilab"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "multilab", "__init__.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugins, err := readPlugins(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "multilab" {
		t.Fatalf("a plugin with no defaults.yml must still be found, got %+v", plugins)
	}
}

// TestSubtypeEnumsAreLifted: sr.protocol declares its enum one level down, under
// `_subtype`. Skipping every underscore key would drop the only machine-readable
// statement of which IGPs can carry a segment identifier.
func TestSubtypeEnumsAreLifted(t *testing.T) {
	attrs := flattenAttributes("global", "", map[string]any{
		"protocol": map[string]any{
			"type":     "list",
			"_subtype": map[string]any{"type": "str", "valid_values": []any{"isis", "ospfv2"}},
		},
	})
	if len(attrs) != 1 {
		t.Fatalf("want 1 attribute, got %+v", attrs)
	}
	if strings.Join(attrs[0].ValidValues, ",") != "isis,ospfv2" {
		t.Fatalf("valid values = %v, want [isis ospfv2]", attrs[0].ValidValues)
	}
}

// TestNilAttributeValuesSurvive: `node: {ldp:, bgp:, vpn:}` means "the same shape
// as global". A reader that dropped nil values would lose whole node sections.
func TestNilAttributeValuesSurvive(t *testing.T) {
	attrs := flattenAttributes("node", "", map[string]any{"ldp": nil, "vpn": nil})
	if len(attrs) != 2 {
		t.Fatalf("nil-valued attributes must be kept, got %+v", attrs)
	}
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// THE CLASSIFIER MUST FAIL CLOSED. The defect it replaces was silent: the
// generator treated every non-underscore key under `attributes:` as an authoring
// scope, so ten netlab framework switches (route_map, keep_subif, phy_ifattr,
// lag_no_propagate, ...) were published as places to author attributes and
// nothing noticed for as long as the manifest has existed.
//
// A guess here is worse than a failure. An unknown section filed as a scope
// teaches an assistant to author something netlab will reject; filed as control it
// hides a real authoring surface. Either way it is wrong silently, which is the
// property this test exists to remove.
func TestUnknownAttributeSectionFailsClosed(t *testing.T) {
	if _, err := classifyAttributeSection("bgp", "a_section_netlab_just_added"); err == nil {
		t.Fatal("an unclassified attribute section was accepted; the generator must refuse it")
	} else {
		for _, want := range []string{"neither a known", "classify it"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("refusal does not say what to do; got %q", err.Error())
			}
		}
	}

	// POSITIVE CONTROLS in both directions, or the refusal above proves nothing.
	if ok, err := classifyAttributeSection("eigrp", "link"); err != nil || !ok {
		t.Fatalf("a real scope was refused: ok=%v err=%v", ok, err)
	}
	if ok, err := classifyAttributeSection("lag", "lag_no_propagate"); err != nil || ok {
		t.Fatalf("a framework control key was published as a scope: ok=%v err=%v", ok, err)
	}
	if ok, err := classifyAttributeSection("sr", "_proto_map"); err != nil || ok {
		t.Fatalf("netlab bookkeeping was published as a scope: ok=%v err=%v", ok, err)
	}
}

// Feature paths are the granularity a capability question needs. An explicit
// `false` must be indistinguishable from absence, or two devices that disagree
// only in verbosity would classify differently.
func TestFeaturePathsOmitUnsupported(t *testing.T) {
	got := flattenFeaturePaths("mpls", map[string]any{
		"vpn": true, "ldp": true, "6pe": false,
	})
	sort.Strings(got)
	want := []string{"mpls.ldp", "mpls.vpn"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	// A bare true is the whole path; a list becomes path=value.
	if p := flattenFeaturePaths("initial", true); len(p) != 1 || p[0] != "initial" {
		t.Fatalf("bare true mishandled: %v", p)
	}
	if p := flattenFeaturePaths("evpn.bundle", []any{"vlan_aware"}); len(p) != 1 || p[0] != "evpn.bundle=vlan_aware" {
		t.Fatalf("list value mishandled: %v", p)
	}
	if p := flattenFeaturePaths("mpls", map[string]any{"vpn": false}); len(p) != 0 {
		t.Fatalf("explicit false must be omitted, got %v", p)
	}
}

// TestTypedDictAndListKeysAreDescended pins the 2026-09-22 gap: a typed dict
// declares its fields under `_keys`, a typed list of dicts under
// `_subtype._keys`, and the flattener stopped at the type. That is why the
// manifest could say a block such as `otg.flows` is a list and nothing about what goes in
// one. The `type` key inside `_keys` is an ATTRIBUTE (ospf.default.type), not a
// type declaration, and must come through with its enum.
func TestTypedDictAndListKeysAreDescended(t *testing.T) {
	got := flattenAttributes("node", "", map[string]any{
		"default": map[string]any{
			"type": "dict",
			"_keys": map[string]any{
				"always": "bool",
				"type":   map[string]any{"type": "str", "valid_values": []any{"e1", "e2"}},
			},
		},
		"sites": map[string]any{
			"type": "list",
			"_subtype": map[string]any{
				"type": "dict",
				"_keys": map[string]any{
					"name": map[string]any{"type": "str"},
					"aps": map[string]any{
						"type": "list",
						"_subtype": map[string]any{
							"type":  "dict",
							"_keys": map[string]any{"mac": map[string]any{"type": "str"}},
						},
					},
				},
			},
		},
	})
	byPath := map[string]Attribute{}
	for _, a := range got {
		byPath[a.Path] = a
	}
	for _, want := range []string{"default", "default.always", "default.type", "sites", "sites[].name", "sites[].aps", "sites[].aps[].mac"} {
		if _, ok := byPath[want]; !ok {
			t.Errorf("missing %q in %v", want, paths(got))
		}
	}
	if a := byPath["default.type"]; len(a.ValidValues) != 2 || a.ValidValues[0] != "e1" {
		t.Errorf("default.type lost its enum: %+v", a)
	}
	if a := byPath["default.always"]; a.Type != "bool" {
		t.Errorf("default.always lost its type: %+v", a)
	}
}

func paths(as []Attribute) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Path)
	}
	sort.Strings(out)
	return out
}

// TestSkyforgeGlobalsReaderRefusesTheWrongFile is the positive control on the
// globals input: an attributes.yml without the blocks we ship must be refused,
// not read as "no globals". A pristine netlab attributes.yml is exactly such a
// file. The accepted case is the COMMITTED manifest (generated from the patched
// tree by TestManifestIsNotStale's twin), checked for the field shape an author
// needs.
func TestSkyforgeGlobalsReaderRefusesTheWrongFile(t *testing.T) {
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
	if _, err := readSkyforgeGlobals("", false); err == nil {
		t.Fatal("an empty path must be refused")
	}
	write("node:\n  x: str\n")
	if _, err := readSkyforgeGlobals(modules, false); err == nil {
		t.Fatal("a file with no global section must be refused")
	}
	write("global:\n  addressing:\n    x: {type: str}\n")
	if _, err := readSkyforgeGlobals(modules, false); err == nil || !strings.Contains(err.Error(), "otg") {
		t.Fatalf("a file without the otg block must be refused and name it; got %v", err)
	}
}

// Part B: netlab resolves `copy:`/`merge:` requests only DIRECTLY under a
// namespace; the manifest must publish the resolved attribute, not `x.copy`.
func TestResolveCopyRequestsMatchesNetlabDepthOne(t *testing.T) {
	attrs := map[string]any{
		"global": map[string]any{"mode": map[string]any{"type": "str", "valid_values": []any{"a", "b"}}, "nest": map[string]any{"inner": "int"}},
		"node": map[string]any{
			"mode": map[string]any{"copy": "global"},
			"nest": map[string]any{"inner": map[string]any{"copy": "global"}}, // too deep: netlab leaves it
		},
		"link": map[string]any{"mode": map[string]any{"merge": "global", "valid_values": []any{"c"}}},
	}
	got := resolveCopyRequests(attrs)
	node := flattenAttributes("node", "", got["node"])
	sortAttributes(node)
	var paths []string
	for _, a := range node {
		paths = append(paths, a.Path+":"+a.Type)
	}
	if strings.Join(paths, ",") != "mode:str,nest.inner.copy:global" {
		t.Fatalf("node attributes after resolve = %v", paths)
	}
	link := flattenAttributes("link", "", got["link"])
	if len(link) != 1 || link[0].Type != "str" || strings.Join(link[0].ValidValues, ",") != "c" {
		t.Fatalf("merge must keep the source type and let the entry's valid_values win: %+v", link)
	}
	if _, still := attrs["node"].(map[string]any)["mode"].(map[string]any)["copy"]; !still {
		t.Fatal("resolveCopyRequests mutated its input")
	}
}

// Netlab folds a plugin's defaults in with Box addition, which discards null
// leaves; the manifest reports what netlab validates against.
func TestDropNilLeavesKeepsEmptiedMaps(t *testing.T) {
	got := dropNilLeaves(map[string]any{"bonding": map[string]any{"mode": nil, "primary": "bool"}, "lone": nil})
	flat := flattenAttributes("node", "", got)
	if len(flat) != 1 || flat[0].Path != "bonding.primary" {
		t.Fatalf("got %+v", flat)
	}
}

func TestDottedGetAcceptsBothSpellings(t *testing.T) {
	if dottedGet(map[string]any{"interface.name": "eth{ifindex}"}, "interface.name") != "eth{ifindex}" {
		t.Fatal("literal dotted key")
	}
	if dottedGet(map[string]any{"interface": map[string]any{"name": "et{ifindex}"}}, "interface.name") != "et{ifindex}" {
		t.Fatal("nested spelling")
	}
}

func TestReadKneServicesParsesTheMapAndRefusesAnUnpatchedProvider(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "kne.py")
	src := "X = 1\nKNE_SERVICE_MAP: dict = {\n  # comment { with brace\n  'fortios': [\n    { 'outside': 80, 'inside': 80, 'name': 'http', 'names': ['http'] },\n    { 'outside': 443, 'inside': 443, 'name': 'https', 'names': ['https'] },\n  ],\n  'linux': [\n    { 'outside': 22, 'inside': 22, 'name': 'ssh', 'names': ['ssh'] },\n  ],\n}\nOTHER = { 'outside': 9999, 'name': 'nope' }\n"
	if err := os.WriteFile(good, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readKneServices(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got["fortios"]) != 2 || got["fortios"][1].Port != 443 || got["fortios"][1].Name != "https" || got["linux"][0].Port != 22 {
		t.Fatalf("parsed %+v", got)
	}
	bad := filepath.Join(dir, "pristine.py")
	if err := os.WriteFile(bad, []byte("X = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readKneServices(bad); err == nil {
		t.Fatal("a provider without KNE_SERVICE_MAP must be refused, not read as 'no node has services'")
	}
}

// TestAttributeRendersNamesWhichTemplatesMentionTheAttribute: per-attribute coverage is a WORD-BOUNDED match of the
// dotted path in each device template. `zb.rules` must not be credited to a template that only names `zb.default_rules`
// or `zb.rules_extra`, a macro include (`_m.j2`) is never a device, and an attribute no template names is omitted
// (unknown) rather than listed with no devices.
func TestAttributeRendersNamesWhichTemplatesMentionTheAttribute(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("panos.j2", "{% for r in zb.rules|default([]) %}x{% endfor %}{% for r in zb.default_rules|default([]) %}y{% endfor %}")
	write("fortios.j2", "{% for r in zb.default_rules|default([]) %}y{% endfor %} {{ zb.rules_extra }}")
	write("_m.j2", "{{ zb.never_named }} {{ zb.rules }}")
	attrs := []Attribute{
		{Scope: "node", Path: "zb.rules"},
		{Scope: "node", Path: "zb.default_rules"},
		{Scope: "node", Path: "zb.unnamed"},
		{Scope: "node", Path: "zb.never_named"},
	}
	got := attributeRenders(dir, attrs, []string{"_m", "fortios", "panos"})
	if want := []string{"panos"}; !reflect.DeepEqual(got["zb.rules"], want) {
		t.Errorf("zb.rules named by %v, want %v (fortios names default_rules and rules_extra only; _m is a macro)", got["zb.rules"], want)
	}
	if want := []string{"fortios", "panos"}; !reflect.DeepEqual(got["zb.default_rules"], want) {
		t.Errorf("zb.default_rules named by %v, want %v", got["zb.default_rules"], want)
	}
	for _, p := range []string{"zb.unnamed", "zb.never_named"} {
		if _, ok := got[p]; ok {
			t.Errorf("%s is named by no device template and must be omitted, got %v", p, got[p])
		}
	}
	if attributeRenders(dir, nil, []string{"panos"}) != nil || attributeRenders(dir, attrs, nil) != nil {
		t.Error("no attributes or no templates must give nil")
	}
}
