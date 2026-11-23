// Package netlabcapsgen generates the netlab CAPABILITY MANIFEST — what the
// patched netlab can actually be asked to do — from the PATCHED netlab tree.
//
// WHY THIS IS GENERATED AND NOT WRITTEN DOWN. netlab runs here with a patch
// series on top of a pinned upstream. The patches add modules, plugins, device
// support, attributes and valid values that upstream does not have. Anything a
// human types about that surface drifts, and has: a committed blueprint declared
// `plugin: [tunnel.dmvpn, ospf.instances, bgp.rr_group]` and NONE OF THE THREE
// EXISTED in the tree — the real plugin names are tunnel.gre / tunnel.wireguard,
// ospf.areas, and bgp.domain / bgp.policy / bgp.session. That blueprint could not
// compile, and no gate said so, because the capability surface lived only in
// people's heads.
//
// It follows the shape of the device-defaults generator exactly, including the
// part that matters most:
//
//	THE INPUT TREE MUST BE PATCHED, AND THE GENERATOR REFUSES TO GUESS.
//	A checkout of upstream netlab is PRISTINE; the patch series is applied when
//	the netlab image is built. The device-defaults generator learned this the
//	expensive way — run against the pristine tree it exited 0 and quietly
//	dropped a device from the catalog. Missing input, clean exit, wrong answer.
//	So this generator asserts a set of CHECKED-IN MARKER PATHS that exist only
//	after the patch series is applied, and fails loudly when they are absent.
package netlabcapsgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// --- manifest schema --------------------------------------------------------
//
// The schema is deliberately flat and name-first. Its two consumers ask
// different questions of it — "does this name exist?" (the authoring validator)
// and "what may I write?" (an authoring assistant's context) — and both are
// answered by lists of names, not by a re-implementation of netlab's type
// system. Anything richer would be a second, drifting copy of the schema.

// Manifest is the generated capability surface of the PATCHED netlab tree.
type Manifest struct {
	// SchemaVersion is the version of THIS manifest's shape. The field arrives
	// with Part B of the netlab shadow-layer plan (Plugin.Attributes, Core, device
	// naming and provider blocks) and is 1: every addition is tolerated by the
	// unversioned (0) reader in skyforge-skills' netlabschema, which reserves 1 for
	// "compatible shape". Bump it on any change a consumer could misread -- a
	// removed or re-meant field -- never for a pure addition.
	SchemaVersion int      `json:"schema_version"`
	Source        string   `json:"source"`
	GeneratedAt   string   `json:"generated_at"`
	Markers       []string `json:"patched_tree_markers"`
	Modules       []Module `json:"modules"`
	Plugins       []Plugin `json:"plugins"`
	Devices       []Device `json:"devices"`
	// KneServices is the kne provider's per-device SERVICE MAP
	// (netsim/providers/kne.py KNE_SERVICE_MAP): the ports the platform exposes on
	// a node's Kubernetes Service, keyed by netlab device name. It is what decides
	// whether a node HAS a web UI (an https/ssl entry on 443) and which TCP ports a
	// jump-host forward may reach. Read from the map, never re-typed: a kind
	// absent here exposes nothing, and a consumer must not invent a port for it.
	KneServices map[string][]KneService `json:"kne_services,omitempty"`
	// Core is netlab's OWN attribute vocabulary (netsim/defaults/attributes.yml):
	// the scopes an author can put attributes in, the reusable entry shapes
	// (`_v_entry`, `_prefix`, ...) and the type names those use.
	Core *Core `json:"core,omitempty"`
	// CanBeFalse are the scopes where `<module>: false` is legal for ANY module,
	// from netsim/defaults/attributes.yml. netlab declares it globally
	// ([link, interface, loopback]) while can_be_true is per module.
	//
	// This is the exact idiom an SE used correctly on 2026-09-03 (`eigrp: false`
	// on a link) while the manifest had no way to say it was legal -- so a
	// classifier reading only module attributes would have called valid authoring
	// a modelling gap and filed a false issue.
	CanBeFalse []string `json:"can_be_false,omitempty"`
	// SkyforgeGlobals are the top-level topology blocks that are OURS, not
	// netlab's: `otg:` (and whatever else the runtime registers under
	// attributes.global in the PATCHED netsim/defaults/attributes.yml). netlab
	// validates them like any other attribute, but they are not upstream's, so a
	// reader must pick them out of attributes.global by name. A block the
	// manifest does not carry is an authoring surface an assistant either
	// guesses at or calls a gap.
	SkyforgeGlobals []SkyforgeGlobal `json:"skyforge_globals,omitempty"`
}

// SchemaVersion is the current manifest schema version (see Manifest.SchemaVersion).
const SchemaVersion = 1

// Core is the netlab-core attribute vocabulary, read from the patched tree's
// netsim/defaults/attributes.yml. Modules and plugins carry their own
// attributes; this is what is left: a link's `bandwidth`, a node's `device`, a
// validation test's `wait`.
type Core struct {
	// Scopes are the sections an author can put attributes in (global, node, link,
	// interface, loopback, pool, prefix, node_group, ...), each flattened with the
	// walker a module's attributes go through.
	Scopes []CoreScope `json:"scopes"`
	// Shapes are the reusable entry definitions, named with netlab's leading
	// underscore (`_v_entry` = one validation test, `_v_option`, `_prefix`,
	// `_r_import`). An attribute whose type is a shape name takes this structure.
	Shapes []CoreShape `json:"shapes"`
	// Control are the non-authorable list switches (can_be_false, link_no_propagate,
	// global_extra_ns, ...): recorded so they are CLASSIFIED, not lost.
	Control map[string][]string `json:"control,omitempty"`
	// Types maps every type name the core attributes reference to what netlab
	// does with it: "validator" (a must_be_<name> in netsim/data/types.py),
	// "shape" (a section of attributes.yml) or "special" (a name netlab handles
	// inline: error, NoneType, a constant such as MAX_NODE_ID).
	Types map[string]string `json:"types"`
}

// CoreScope is one authorable section of netsim/defaults/attributes.yml.
type CoreScope struct {
	Name       string      `json:"name"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

// CoreShape is one `_`-prefixed reusable entry definition.
type CoreShape struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Attributes  []Attribute `json:"attributes,omitempty"`
}

// SkyforgeGlobal is one Skyforge-owned top-level block under attributes.global in
// the patched netsim/defaults/attributes.yml. Attributes use the same shape as a module's so
// every consumer that already renders module attributes renders these unchanged.
type SkyforgeGlobal struct {
	Name string `json:"name"`
	// Attributes are the authorable paths WITHIN the block (scope is always
	// "global"; the path is relative to the block, so `otg.flows[].tx`
	// appears as `flows[].tx` under name "otg" -- the same
	// convention Module.Attributes uses for `mpls.vpn.ipv4` -> "vpn.ipv4").
	Attributes []Attribute `json:"attributes,omitempty"`
}

// Module is one netsim/modules/<name>.yml.
type Module struct {
	Name string `json:"name"`
	// Requires is the module's own `requires:` list — modules netlab REFUSES the
	// topology without. Recorded so a consumer never duplicates an error netlab
	// already raises.
	Requires []string `json:"requires,omitempty"`
	// Features is the module's declared feature catalogue (name -> description).
	Features map[string]string `json:"features,omitempty"`
	// Attributes is every authorable attribute path, per scope.
	Attributes []Attribute `json:"attributes,omitempty"`
	// CanBeTrue are the scopes where `<module>: true` is legal, from the module's
	// own `attributes.can_be_true` (multicast: [link, interface], gateway: [link]).
	CanBeTrue []string `json:"can_be_true,omitempty"`
	// ControlSections are the non-authorable framework switches this module
	// declares. Recorded so a consumer can see they were CLASSIFIED, not lost.
	ControlSections []string `json:"control_sections,omitempty"`
}

// netlabAuthorableScopes are the attribute sections netlab actually VALIDATES
// against, derived from every attr_list= it passes to validate_attributes():
// augment/topology.py ['global','internal'], augment/nodes.py ['node'] and
// ['loopback'], augment/links.py ['link'] and ['interface','link'],
// augment/addressing.py ['pool'], modules/vrf.py ['loopback','link','interface'].
// These are the only sections an author can put an attribute in.
var netlabAuthorableScopes = map[string]bool{
	"global": true, "internal": true, "node": true, "link": true,
	"interface": true, "loopback": true, "pool": true,
	// Group namespaces: augment/groups.py validates with a computed namespace.
	"node_group": true, "vlan_group": true, "vrf_group": true,
	// Module-level object sections carried through by vlan.py / vrf.py.
	"vlan": true, "vrf": true, "node_vlan": true,
}

// netlabControlSections are keys under `attributes:` that are NOT authorable
// scopes but netlab framework switches, each read BY NAME in netsim/*.py.
// Publishing them as scopes is what made the manifest advertise
// `route_map`/`keep_subif`/`phy_ifattr` as places to author attributes.
//
// can_be_false/can_be_true are promoted to first-class fields below rather than
// discarded: they declare where `<module>: false` / `<module>: true` are legal,
// which is the netlab idiom an SE used correctly on 2026-09-03 while our
// manifest had no way to say so.
var netlabControlSections = map[string]bool{
	"can_be_false": true, "can_be_true": true,
	"link_no_propagate": true, "link_module_no_propagate": true,
	"vlan_no_propagate": true, "lag_no_propagate": true,
	"global_extra_ns": true, "node_extra_ns": true, "pool_no_copy": true,
	"lag_l2_linkattr": true, "lag_member_linkattr": true,
	"copy_vlan_to_intf": true, "keep_subif": true, "phy_ifattr": true,
	"route_map": true, "as_list": true, "node_copy": true,
	"vrf_copy": true, "vrf_aware": true, "protocols": true, "reserved": true,
	"anycast": true, "bundle": true, "clients": true, "vrrp": true,
	"intf_to_neighbor": true, "link_to_neighbor": true, "intf_optional": true,
}

// classifyAttributeSection returns whether a section is authorable, and FAILS
// CLOSED on anything it has never seen.
//
// A silent guess here is the whole defect this replaces: the generator used to
// treat every non-underscore key as a scope, so ten framework switches were
// published as authoring targets and nothing noticed. When netlab adds a
// section, this must go RED and a human must classify it — an unknown section
// misfiled as a scope teaches an assistant to author something netlab will
// reject, and misfiled as control hides a real authoring surface.
func classifyAttributeSection(module, section string) (authorable bool, err error) {
	switch {
	case strings.HasPrefix(section, "_"):
		return false, nil // netlab bookkeeping: _proto_map, _inherit_community
	case netlabAuthorableScopes[section]:
		return true, nil
	case netlabControlSections[section]:
		return false, nil
	}
	return false, fmt.Errorf(
		"module %q declares attribute section %q which is neither a known netlab "+
			"authorable scope nor a known framework control key; classify it in "+
			"internal/netlabcapsgen (netlabAuthorableScopes or netlabControlSections) "+
			"before regenerating — guessing publishes a wrong authoring surface",
		module, section)
}

// Attribute is one authorable key under a module's `attributes:`.
type Attribute struct {
	// Scope is netlab's attribute section: global, node, link, interface, vlan,
	// vrf, node_vlan, ...
	Scope string `json:"scope"`
	// Path is the dotted attribute path WITHIN the module, e.g. "vpn.ipv4" for
	// the mpls module (authored as `mpls.vpn.ipv4`).
	Path string `json:"path"`
	// Type is netlab's declared type when it declares one.
	Type string `json:"type,omitempty"`
	// ValidValues is the declared enum when there is one. This is the only place
	// a consumer may learn, for example, that evpn.transport is one of
	// vxlan/mpls/sr — and it is why that fact is not hand-written anywhere.
	ValidValues []string `json:"valid_values,omitempty"`
	// Required is netlab's own `_required: True` on the attribute: netlab (or the
	// plugin that registers it) refuses a topology that enables the owner and
	// omits this key. Carried so a consumer can tell an author which keys they
	// MUST write, from netlab itself rather than from recall.
	Required bool `json:"required,omitempty"`
}

// Plugin is one netsim/extra/<...>/defaults.yml, named as an author writes it.
type Plugin struct {
	Name string `json:"name"`
	// Devices declare support for the plugin in its defaults.yml.
	Devices []string `json:"devices,omitempty"`
	// Renders are the devices with a NON-EMPTY <plugin-dir>/<device>.j2. A
	// declared-but-untemplated plugin is a silent no-op, so the two are reported
	// separately rather than collapsed — the same split netlabdefaultsgen keeps,
	// for the same reason.
	Renders []string `json:"renders,omitempty"`
	// Attributes are the authorable attributes the plugin registers under its
	// defaults.yml `attributes:` (firewall.zone / firewall.security_level on an
	// interface, nat on a node, the plugin's own global block). Same shape as a
	// module's, same walker. Only sections netlab validates against are kept;
	// the plugin's reusable `_x` shapes and its helper sections are not
	// authoring targets (the latter are named in AttributeSections).
	Attributes []Attribute `json:"attributes,omitempty"`
	// AttributeRenders is PER-ATTRIBUTE render coverage: for each authorable attribute (Attributes[].Path) that at least
	// one device template NAMES, the devices whose `<plugin-dir>/<device>.j2` names it. It exists because Renders alone
	// says "fortios renders firewall.zonebased" and the schema says "firewall.zonebased.rules is a valid attribute", so
	// together they implied fortios renders the rules; it does not (only panos does), and netlab accepts the attribute
	// and emits nothing. EVIDENCE, NOT PROOF: it is a word-bounded text match of the dotted path in the template, so a
	// plugin that renders an attribute through Python-derived data, or a macro include, can look less covered than it is.
	// An attribute no template names is OMITTED (unknown), never listed with an empty device list.
	AttributeRenders map[string][]string `json:"attribute_renders,omitempty"`
	// AttributeSections are the non-authorable, non-underscore sections the plugin
	// declares under `attributes:` (bgp_role, exbs_timers, ...): classified, not lost.
	AttributeSections []string `json:"attribute_sections,omitempty"`
}

// Device is one netsim/devices/<name>.yml.
type Device struct {
	Name string `json:"name"`
	// Parent is the device it inherits from (iol -> ios), because module
	// templates resolve along that chain.
	Parent string `json:"parent,omitempty"`
	// Modules are the module names the device declares under `features:`. In
	// netlab 26.08 that key IS the declaration: there is no `supported_on` on the
	// module side.
	Modules []string `json:"modules,omitempty"`
	// Features are the dotted feature paths the device declares TRUE, e.g.
	// "mpls.vpn", "bgp.local_as", "vrf.ospfv2". This is the granularity a
	// capability question actually needs: "does this device do mpls" is rarely
	// the question. A path is present only when netlab declares it supported --
	// an explicit `false` is omitted, so absence means unsupported.
	//
	// NOT VERSION-AWARE, and cannot be: netlab declares this per DEVICE TYPE, not
	// per image. A device that declares mpls.vpn on an image which refuses it is
	// a declaration-vs-reality gap only a certification run can catch.
	Features []string `json:"features,omitempty"`
	// InterfaceName is the device's OWN interface_name template (Ethernet{ifindex}),
	// not inherited from Parent: netlab merges the parent chain when it loads a
	// node, so the manifest reports what the device file declares.
	InterfaceName string `json:"interface_name,omitempty"`
	// IfindexOffset is the device's own ifindex_offset when it declares one.
	IfindexOffset *int `json:"ifindex_offset,omitempty"`
	// MgmtIf is the device's own management interface name.
	MgmtIf string `json:"mgmt_if,omitempty"`
	// SupportLevel is netlab's `support.level` (minimal, best-effort, obsolete, ...)
	// when the device declares one; empty means fully supported.
	SupportLevel string `json:"support_level,omitempty"`
	// Providers are the per-provider overrides a device declares (`kne:` carries
	// `interface.name: eth{ifindex}`, `clab:` carries et{ifindex} on cEOS). The
	// provider that runs the lab decides the real interface name, so a consumer
	// validating a name must look here first. Keyed by netlab's own provider names.
	Providers map[string]DeviceProvider `json:"providers,omitempty"`
}

// KneService is one entry of KNE_SERVICE_MAP. Only the outside port and the
// primary name are kept; the aliases ("names") are netlab-internal.
type KneService struct {
	Port int    `json:"port"`
	Name string `json:"name"`
}

// DeviceProvider is the subset of a device's `<provider>:` block that names
// interfaces.
type DeviceProvider struct {
	InterfaceName string `json:"interface_name,omitempty"`
	MgmtIf        string `json:"mgmt_if,omitempty"`
}

// --- generation -------------------------------------------------------------

// Options are the generator's inputs. EVERY netlab-tree path is separately
// redirectable, and that is not a convenience: netlabdefaultsgen was once fixed
// by redirecting only --devices-dir, which left two other inputs resolving to
// the PRISTINE tree and would have taken SNMP collection dark fleet-wide with
// nothing failing anywhere. Half a tree is not the tree.
type Options struct {
	ModulesDir string
	PluginsDir string
	DevicesDir string
	// ProvidersDir is netsim/providers of the PATCHED tree. Empty means the
	// sibling of DevicesDir, so every existing caller keeps one input tree.
	ProvidersDir string
	MarkersFile  string
	Out          string
	// OSS selects the PUBLIC manifest: the input tree must be upstream netlab with
	// the PUBLISHED series applied (the private-only changes absent), and only the
	// globals that series carries (globalBlockNames) are required and read. Nothing
	// private is filtered out of a manifest here: it is absent because the tree it
	// is read from does not have it. The private manifest is unchanged.
	OSS bool
}

// Encode renders the manifest deterministically. generated_at is the ONLY
// volatile field, so a drift check can strip exactly one thing.
func Encode(m *Manifest) ([]byte, error) {
	blob, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(blob, '\n'), nil
}

// Generate reads the patched tree and builds the manifest.
func Generate(o Options) (*Manifest, error) {
	o.OSS = o.OSS || ossBuild
	if err := assertPatchedTree(o); err != nil {
		return nil, err
	}
	markers, err := readMarkers(o.MarkersFile)
	if err != nil {
		return nil, err
	}
	source := manifestSource(o.OSS)
	m := &Manifest{
		SchemaVersion: SchemaVersion,
		Source:        source,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Markers:       markers.Paths(),
	}
	if m.Modules, err = readModules(o.ModulesDir); err != nil {
		return nil, err
	}
	if m.Plugins, err = readPlugins(o.PluginsDir); err != nil {
		return nil, err
	}
	if m.Devices, err = readDevices(o.DevicesDir); err != nil {
		return nil, err
	}
	providersDir := o.ProvidersDir
	if providersDir == "" {
		providersDir = filepath.Join(filepath.Dir(filepath.Clean(o.DevicesDir)), "providers")
	}
	if m.KneServices, err = readKneServices(filepath.Join(providersDir, "kne.py")); err != nil {
		return nil, err
	}
	if m.Core, err = readCore(o.ModulesDir); err != nil {
		return nil, err
	}
	if m.CanBeFalse, err = readCanBeFalse(o.ModulesDir); err != nil {
		return nil, err
	}
	if m.SkyforgeGlobals, err = readSkyforgeGlobals(o.ModulesDir, o.OSS); err != nil {
		return nil, err
	}
	return m, nil
}

// --- skyforge-only global blocks ------------------------------------------------

// globalBlockNames (globals_private.go, globals_oss.go) are the top-level topology blocks
// read out of attributes.global. attributes.global also holds netlab's OWN blocks
// (addressing, groups, ...), so ours are picked out by name. They are all REQUIRED:
// an attributes.yml that lacks one is an unpatched or wrong tree, and must go RED
// rather than publish a manifest that quietly carries none of the blocks we ship.
// If a block is genuinely retired, retire its name there in the same change.

// readSkyforgeGlobals reads our blocks out of attributes.global of the PATCHED
// tree's netsim/defaults/attributes.yml (derived from the modules dir, like
// readCanBeFalse). Each block is flattened with the same walker a module's
// attributes go through, so a consumer that can render `mpls.vpn.ipv4` can
// render `otg.flows[].tx` without learning a second shape.
func readSkyforgeGlobals(modulesDir string, oss bool) ([]SkyforgeGlobal, error) {
	names := globalBlockNames(oss)
	if strings.TrimSpace(modulesDir) == "" {
		return nil, fmt.Errorf("no modules dir given — the patched netsim/defaults/attributes.yml is the ONLY source of the top-level blocks")
	}
	netsim := filepath.Dir(strings.TrimRight(modulesDir, string(filepath.Separator)))
	path := filepath.Join(netsim, "defaults", "attributes.yml")
	doc, err := readYAMLMap(path)
	if err != nil {
		return nil, err
	}
	global := asMap(doc["global"])
	if len(global) == 0 {
		return nil, fmt.Errorf("%s has no global section — is this netlab's netsim/defaults/attributes.yml?", path)
	}
	var missing []string
	for _, k := range names {
		if _, ok := global[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"REFUSING to generate: %s global lacks %s.\n"+
				"These blocks are added by the netlab patch series; an attributes.yml without them is an UNPATCHED tree, "+
				"or the block moved and globalBlockNames in internal/netlabcapsgen must be re-pointed in the same change.",
			path, strings.Join(missing, ", "))
	}
	out := []SkyforgeGlobal{}
	for _, name := range names {
		g := SkyforgeGlobal{Name: name, Attributes: flattenAttributes("global", "", global[name])}
		sort.Slice(g.Attributes, func(i, j int) bool { return g.Attributes[i].Path < g.Attributes[j].Path })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- the patched-tree refusal -------------------------------------------------

type markerFile struct {
	// Markers each name a path relative to netsim/ that a patch CREATES, plus
	// the patch that creates it, so a marker that stops discriminating can be
	// traced instead of guessed at.
	Markers []struct {
		Path  string `json:"path"`
		Patch string `json:"patch"`
		Why   string `json:"why"`
	} `json:"markers"`
}

func (m markerFile) Paths() []string {
	out := make([]string, 0, len(m.Markers))
	for _, k := range m.Markers {
		out = append(out, k.Path)
	}
	sort.Strings(out)
	return out
}

func readMarkers(path string) (markerFile, error) {
	var mf markerFile
	blob, err := os.ReadFile(path)
	if err != nil {
		return mf, fmt.Errorf("reading marker file %s: %w", path, err)
	}
	if err := json.Unmarshal(blob, &mf); err != nil {
		return mf, fmt.Errorf("parsing marker file %s: %w", path, err)
	}
	if len(mf.Markers) == 0 {
		return mf, fmt.Errorf("marker file %s declares no markers — an assertion that asserts nothing cannot prove the tree is patched", path)
	}
	return mf, nil
}

// assertPatchedTree refuses to generate from a pristine upstream checkout.
//
// The marker set is CHECKED IN rather than hard-coded, for the reason
// netlabdefaultsgen records: its own positive control went inert when the file
// it asserted on (netsim/ansible/templates/services/eos.j2) was absorbed
// upstream, so the check passed even if zero patches applied, and its comment
// still names two patches that no longer exist. A marker in a data file can be
// re-pointed by whoever moves the patch series; a marker in a comment rots.
func assertPatchedTree(o Options) error {
	mf, err := readMarkers(o.MarkersFile)
	if err != nil {
		return err
	}
	netsim := filepath.Dir(strings.TrimRight(o.ModulesDir, string(filepath.Separator)))
	var missing []string
	for _, k := range mf.Markers {
		if _, err := os.Stat(filepath.Join(netsim, k.Path)); err != nil {
			missing = append(missing, fmt.Sprintf("%s (added by %s: %s)", k.Path, k.Patch, k.Why))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"REFUSING to generate: the netlab tree under %s is NOT patched.\n"+
			"  missing marker(s):\n    %s\n"+
			"This almost always means the input points at a PRISTINE upstream netlab checkout. The patch series is\n"+
			"applied when the netlab IMAGE is built, never in a checkout. Materialise a patched tree first: copy the\n"+
			"upstream checkout and apply the patch series to the copy,\n"+
			"then point --modules-dir/--plugins-dir/--devices-dir at it. ALL THREE: redirecting one and leaving the others\n"+
			"on their defaults publishes a manifest that is half pristine.",
		netsim, strings.Join(missing, "\n    "))
}

// --- modules ------------------------------------------------------------------

func readModules(dir string) ([]Module, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading modules dir %s: %w", dir, err)
	}
	out := []Module{}
	for _, e := range entries {
		// netsim/modules also holds a `routing/` PACKAGE directory with Python and
		// no yml; skipping directories is what makes the *.yml glob correct.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		doc, err := readYAMLMap(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		m := Module{Name: strings.TrimSuffix(e.Name(), ".yml")}
		m.Requires = stringList(doc["requires"])
		sort.Strings(m.Requires)
		if feats := asMap(doc["features"]); len(feats) > 0 {
			m.Features = map[string]string{}
			for k, v := range feats {
				m.Features[k] = fmt.Sprint(v)
			}
		}
		for scope, section := range resolveCopyRequests(asMap(doc["attributes"])) {
			// can_be_true declares WHERE `<module>: true` is legal. It is data, not
			// a scope, and it is the answer to "can I switch this module off/on on
			// one link" -- the question an SE asked on 2026-09-03.
			if scope == "can_be_true" {
				m.CanBeTrue = stringList(section)
				sort.Strings(m.CanBeTrue)
				continue
			}
			authorable, cerr := classifyAttributeSection(m.Name, scope)
			if cerr != nil {
				return nil, cerr
			}
			if !authorable {
				if !strings.HasPrefix(scope, "_") {
					m.ControlSections = append(m.ControlSections, scope)
				}
				continue
			}
			m.Attributes = append(m.Attributes, flattenAttributes(scope, "", section)...)
		}
		sort.Strings(m.ControlSections)
		sortAttributes(m.Attributes)
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, fmt.Errorf("no module definitions found under %s — an empty manifest is not a small manifest", dir)
	}
	return out, nil
}

// flattenAttributes turns netlab's polymorphic attribute tree into dotted paths.
//
// A leaf can be a bare type name (`igp_sync: bool`), an inline map
// (`{type: str, valid_values: [...]}`), a nested map of sub-attributes, or NIL —
// `node: {ldp:, bgp:, vpn:}` means "the same shape as global". A reader that
// dropped nil values would silently lose entire node sections.
func flattenAttributes(scope, prefix string, node any) []Attribute {
	switch v := node.(type) {
	case nil:
		if prefix == "" {
			return nil
		}
		return []Attribute{{Scope: scope, Path: prefix}}
	case string:
		if prefix == "" {
			return nil
		}
		return []Attribute{{Scope: scope, Path: prefix, Type: v}}
	case []any:
		// e.g. `session: [ibgp, ebgp]` — a list of permitted values.
		if prefix == "" {
			return nil
		}
		return []Attribute{{Scope: scope, Path: prefix, ValidValues: stringList(v)}}
	}
	m := asMap(node)
	if len(m) == 0 {
		if prefix == "" {
			return nil
		}
		return []Attribute{{Scope: scope, Path: prefix}}
	}
	// An inline type declaration, not a nesting: {type: ..., valid_values: [...]}.
	if _, isTyped := m["type"]; isTyped && prefix != "" {
		a := Attribute{Scope: scope, Path: prefix, Type: fmt.Sprint(m["type"])}
		a.ValidValues = stringList(m["valid_values"])
		a.Required = isTrue(m["_required"])
		// A LIST attribute declares its element enum one level down, under
		// `_subtype`. sr.protocol is exactly this shape —
		// `{type: list, _subtype: {type: str, valid_values: [isis, ospfv2]}}` —
		// and skipping every underscore key would drop the only machine-readable
		// statement of which IGPs can carry a segment identifier. Lift it onto
		// the parent, which is where an author writes the value.
		sub := asMap(m["_subtype"])
		if len(a.ValidValues) == 0 && len(sub) > 0 {
			a.ValidValues = stringList(sub["valid_values"])
		}
		out := []Attribute{a}
		// A typed DICT declares its sub-attributes under `_keys` -- netlab's
		// way to keep `type` usable as an attribute name (ospf.default.type is
		// one of e1/e2). A typed LIST of dicts does the same one level down,
		// `_subtype._keys`. Both were dropped here until 2026-09-22, which is
		// why the manifest said a block such as `otg.flows` is "a list" and nothing more:
		// every field an author actually has to write (flows[].tx, ...) lives
		// under those two keys. List elements carry a `[]` marker so
		// the rendered path says what it is -- an author does not write
		// `sites.aps`, they write one `aps:` entry per site.
		if keys := asMap(m["_keys"]); len(keys) > 0 {
			out = append(out, flattenKeys(scope, prefix, keys)...)
		}
		if keys := asMap(sub["_keys"]); len(keys) > 0 {
			out = append(out, flattenKeys(scope, prefix+"[]", keys)...)
		}
		return out
	}
	out := []Attribute{}
	if prefix != "" {
		if vv := stringList(m["valid_values"]); len(vv) > 0 {
			out = append(out, Attribute{Scope: scope, Path: prefix, ValidValues: vv})
		}
	}
	for k, child := range m {
		if strings.HasPrefix(k, "_") || k == "type" || k == "valid_values" {
			continue
		}
		next := k
		if prefix != "" {
			next = prefix + "." + k
		}
		out = append(out, flattenAttributes(scope, next, child)...)
	}
	return out
}

// flattenKeys walks a `_keys:` map. It differs from flattenAttributes in one
// deliberate way: EVERY key is an attribute name, including `type` and
// `valid_values`. That is the whole point of `_keys` in netlab's schema -- the
// map exists so an attribute can be called `type` without being read as the
// parent's type declaration.
func flattenKeys(scope, prefix string, keys map[string]any) []Attribute {
	out := []Attribute{}
	for k, child := range keys {
		if strings.HasPrefix(k, "_") {
			continue
		}
		out = append(out, flattenAttributes(scope, prefix+"."+k, child)...)
	}
	return out
}

// --- plugins ------------------------------------------------------------------

// readPlugins walks netsim/extra, which is a MIXED one- and two-level tree:
// `nat/defaults.yml` is the plugin `nat`, and `firewall/policy/defaults.yml` is
// the plugin `firewall.policy` — while `firewall.policy/` ALSO exists as a
// literal dotted directory at the top level, side by side with `firewall/`.
// A one-level scan finds `firewall`, sees no defaults.yml, and reports NOTHING,
// which is how every nested plugin once looked unsupported on every device.
func readPlugins(dir string) ([]Plugin, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading plugins dir %s: %w", dir, err)
	}
	out := []Plugin{}
	seen := map[string]bool{}
	add := func(name, pdir string) error {
		// `firewall.policy` exists BOTH as a literal dotted top-level directory
		// and as netsim/extra/firewall/policy. Same plugin, two paths; record it
		// once.
		if seen[name] {
			return nil
		}
		p, err := readPlugin(name, pdir)
		if err != nil {
			return err
		}
		seen[name] = true
		out = append(out, p)
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		top := filepath.Join(dir, e.Name())
		if isPluginDir(top) {
			if err := add(e.Name(), top); err != nil {
				return nil, err
			}
		}
		// DESCEND REGARDLESS. `tunnel/` is itself a plugin directory AND the
		// parent of tunnel.gre and tunnel.wireguard; stopping at the parent lost
		// both children, and the loss was invisible until a checker stopped
		// flagging `tunnel.dmvpn` — a name that does not exist — because the
		// namespace it belongs to had vanished from the manifest too.
		children, err := os.ReadDir(top)
		if err != nil {
			continue
		}
		for _, c := range children {
			if !c.IsDir() {
				continue
			}
			sub := filepath.Join(top, c.Name())
			if isPluginDir(sub) {
				if err := add(e.Name()+"."+c.Name(), sub); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, fmt.Errorf("no plugins found under %s", dir)
	}
	return out, nil
}

// isPluginDir decides whether a directory under netsim/extra IS a plugin.
//
// The marker is `__init__.py`, NOT `defaults.yml`. defaults.yml is optional —
// it exists only to declare per-device support — and using it as the marker
// silently DROPS the plugins that have none. Measured against the blueprint
// corpus on 2026-09-01, that mistake reported `multilab` (used by two
// blueprints) and `check.config` (used by two more) as nonexistent. The
// namespace directories that are NOT plugins (bgp/, ebgp/, firewall/, mlag/,
// node/, ospf/, vrrp/, check/, test/, none/) have neither file, so the marker
// still tells them apart from their real members.
func isPluginDir(dir string) bool {
	return fileExists(filepath.Join(dir, "__init__.py")) ||
		fileExists(filepath.Join(dir, "plugin.py")) ||
		fileExists(filepath.Join(dir, "defaults.yml"))
}

func readPlugin(name, dir string) (Plugin, error) {
	p := Plugin{Name: name}
	// defaults.yml is OPTIONAL: a plugin with no per-device declarations simply
	// has none, which is not an error and must not drop the plugin.
	if fileExists(filepath.Join(dir, "defaults.yml")) {
		doc, err := readYAMLMap(filepath.Join(dir, "defaults.yml"))
		if err != nil {
			return p, err
		}
		for device := range asMap(doc["devices"]) {
			p.Devices = append(p.Devices, device)
		}
		sort.Strings(p.Devices)
		for section, body := range asMap(doc["attributes"]) {
			body = dropNilLeaves(body)
			switch {
			case strings.HasPrefix(section, "_"):
				continue // the plugin's reusable entry shapes, not authoring targets
			case netlabAuthorableScopes[section]:
				p.Attributes = append(p.Attributes, flattenAttributes(section, "", body)...)
			default:
				p.AttributeSections = append(p.AttributeSections, section)
			}
		}
		sortAttributes(p.Attributes)
		sort.Strings(p.AttributeSections)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return p, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".j2") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 {
			continue // an empty template renders nothing; it is not support
		}
		p.Renders = append(p.Renders, strings.TrimSuffix(e.Name(), ".j2"))
	}
	sort.Strings(p.Renders)
	p.AttributeRenders = attributeRenders(dir, p.Attributes, p.Renders)
	return p, nil
}

// attributeRenders implements Plugin.AttributeRenders: which of the plugin's device templates name each attribute.
// Underscore-prefixed templates are macro includes, never resolved as a device, so they are not evidence for one.
func attributeRenders(dir string, attrs []Attribute, templates []string) map[string][]string {
	if len(attrs) == 0 || len(templates) == 0 {
		return nil
	}
	text := map[string]string{}
	for _, t := range templates {
		if strings.HasPrefix(t, "_") {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, t+".j2")); err == nil {
			text[t] = string(b)
		}
	}
	out := map[string][]string{}
	for _, a := range attrs {
		re, err := regexp.Compile(`(?:^|[^A-Za-z0-9_])` + regexp.QuoteMeta(a.Path) + `(?:[^A-Za-z0-9_]|$)`)
		if err != nil {
			continue
		}
		var devs []string
		for _, t := range templates {
			if re.MatchString(text[t]) {
				devs = append(devs, t)
			}
		}
		if len(devs) > 0 {
			sort.Strings(devs)
			out[a.Path] = devs
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// --- devices ------------------------------------------------------------------

// flattenFeaturePaths turns a device's `features.<module>` subtree into dotted
// paths that are declared SUPPORTED. netlab writes these four ways:
//
//	mpls: {vpn: True, ldp: True}      -> mpls.vpn, mpls.ldp
//	vrf:  {ospfv2: True}              -> vrf.ospfv2
//	evpn: {bundle: [vlan_aware]}      -> evpn.bundle=vlan_aware
//	initial: True                     -> initial
//
// A value of False is a declaration that the feature is NOT supported, so it is
// omitted: absence and explicit-false must mean the same thing to a consumer, or
// two devices that disagree only in verbosity would classify differently.
func flattenFeaturePaths(prefix string, node any) []string {
	switch v := node.(type) {
	case nil:
		return []string{prefix}
	case bool:
		if !v {
			return nil
		}
		return []string{prefix}
	case string:
		return []string{prefix + "=" + v}
	case []any:
		vals := stringList(v)
		if len(vals) == 0 {
			return []string{prefix}
		}
		out := make([]string, 0, len(vals))
		for _, s := range vals {
			out = append(out, prefix+"="+s)
		}
		return out
	}
	m := asMap(node)
	if len(m) == 0 {
		return []string{prefix}
	}
	out := []string{}
	for k, child := range m {
		if strings.HasPrefix(k, "_") {
			continue
		}
		out = append(out, flattenFeaturePaths(prefix+"."+k, child)...)
	}
	return out
}

// readCanBeFalse reads the GLOBAL `can_be_false` declaration from
// netsim/defaults/attributes.yml. netsim/ is derived from ModulesDir the same way
// assertPatchedTree derives it, so there is one convention for locating the tree.
//
// It REFUSES rather than defaulting to empty: an absent declaration would make
// every `<module>: false` look illegal, and a classifier would then file a gap
// issue against correct authoring. A missing key means netlab moved it, which a
// human must see.
func readCanBeFalse(modulesDir string) ([]string, error) {
	netsim := filepath.Dir(strings.TrimRight(modulesDir, string(filepath.Separator)))
	path := filepath.Join(netsim, "defaults", "attributes.yml")
	doc, err := readYAMLMap(path)
	if err != nil {
		return nil, fmt.Errorf("reading global attribute schema %s: %w", path, err)
	}
	vals := stringList(doc["can_be_false"])
	if len(vals) == 0 {
		return nil, fmt.Errorf(
			"%s declares no can_be_false: netlab has moved or renamed the global "+
				"`<module>: false` declaration; classify it before regenerating, or the "+
				"manifest will report correct authoring as unsupported", path)
	}
	sort.Strings(vals)
	return vals, nil
}

func readDevices(dir string) ([]Device, error) {
	providers := providerNames(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading devices dir %s: %w", dir, err)
	}
	out := []Device{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		doc, err := readYAMLMap(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		d := Device{Name: strings.TrimSuffix(e.Name(), ".yml")}
		if parent, ok := doc["parent"].(string); ok {
			d.Parent = parent
		}
		for module, feats := range asMap(doc["features"]) {
			d.Modules = append(d.Modules, module)
			// CARRY THE FEATURE VALUES, not just the module names.
			//
			// This loop used to keep only the keys, so the manifest said "eos
			// supports mpls" and could not say whether it supports mpls.vpn. An SE
			// reported on 2026-09-03 that a capability "won't work no matter how you
			// configure it" -- and netlab HAD the answer in these values
			// (eos.features.mpls = {6pe, bgp, ldp, vpn}) while we discarded it.
			d.Features = append(d.Features, flattenFeaturePaths(module, feats)...)
		}
		sort.Strings(d.Modules)
		sort.Strings(d.Features)
		d.InterfaceName = strings.TrimSpace(parseStr(doc["interface_name"]))
		if v, ok := doc["ifindex_offset"].(int); ok {
			off := v
			d.IfindexOffset = &off
		}
		d.MgmtIf = strings.TrimSpace(parseStr(doc["mgmt_if"]))
		d.SupportLevel = strings.TrimSpace(parseStr(asMap(doc["support"])["level"]))
		for _, prov := range providers {
			block := asMap(doc[prov])
			if len(block) == 0 {
				continue
			}
			dp := DeviceProvider{
				InterfaceName: strings.TrimSpace(parseStr(dottedGet(block, "interface.name"))),
				MgmtIf:        strings.TrimSpace(parseStr(block["mgmt_if"])),
			}
			if dp == (DeviceProvider{}) {
				continue
			}
			if d.Providers == nil {
				d.Providers = map[string]DeviceProvider{}
			}
			d.Providers[prov] = dp
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) == 0 {
		return nil, fmt.Errorf("no device definitions found under %s", dir)
	}
	return out, nil
}

// --- kne service map ----------------------------------------------------------

var (
	kneServiceMapStart = regexp.MustCompile(`(?m)^KNE_SERVICE_MAP\b[^=\n]*=\s*\{`)
	kneServiceDevice   = regexp.MustCompile(`^\s*'([^']+)'\s*:\s*\[`)
	kneServiceEntry    = regexp.MustCompile(`'outside'\s*:\s*(\d+)[^}]*'name'\s*:\s*'([^']+)'`)
)

// readKneServices extracts KNE_SERVICE_MAP from the provider source. The map is
// a plain Python dict literal and is parsed as text on purpose: importing the
// provider would drag in netlab itself, and a second hand-copy of the ports is
// exactly the drift this manifest exists to prevent. An absent or empty map
// FAILS: it means an unpatched tree (the map arrives with the patch series) and
// publishing "no node has a web UI" from it would hide every Web UI action.
func readKneServices(path string) (map[string][]KneService, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading kne provider %s: %w", path, err)
	}
	text := string(blob)
	loc := kneServiceMapStart.FindStringIndex(text)
	if loc == nil {
		return nil, fmt.Errorf("%s has no KNE_SERVICE_MAP: not a patched netlab tree", path)
	}
	out := map[string][]KneService{}
	depth := 1
	current := ""
	for _, line := range strings.Split(text[loc[1]:], "\n") {
		code := line
		if i := strings.Index(code, "#"); i >= 0 {
			code = code[:i]
		}
		if depth == 1 {
			if m := kneServiceDevice.FindStringSubmatch(code); m != nil {
				current = m[1]
				if _, ok := out[current]; !ok {
					out[current] = []KneService{}
				}
			}
		}
		if current != "" {
			for _, m := range kneServiceEntry.FindAllStringSubmatch(code, -1) {
				port, convErr := strconv.Atoi(m[1])
				if convErr != nil {
					return nil, fmt.Errorf("%s: bad port %q for %s", path, m[1], current)
				}
				out[current] = append(out[current], KneService{Port: port, Name: m[2]})
			}
		}
		depth += strings.Count(code, "{") - strings.Count(code, "}")
		if depth <= 0 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: KNE_SERVICE_MAP parsed to nothing", path)
	}
	return out, nil
}

// --- yaml helpers -------------------------------------------------------------

func readYAMLMap(path string) (map[string]any, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var doc any
	if err := yaml.Unmarshal(blob, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return asMap(doc), nil
}

// asMap handles both map shapes yaml.v3 can produce. Numeric and boolean keys
// (netlab has `6pe:`) still surface as map[any]any.
func asMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out
	}
	return nil
}

// isTrue reads netlab's boolean flags (`_required: True`), which YAML delivers as
// a bool; a quoted string is tolerated because the flag's meaning is unambiguous.
func isTrue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(strings.TrimSpace(b), "true")
	}
	return false
}

func stringList(v any) []string {
	switch l := v.(type) {
	case []any:
		out := make([]string, 0, len(l))
		for _, x := range l {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case string:
		return []string{l}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// providerNames are netlab's own providers: the file names under
// netsim/providers (clab, kne, libvirt, ...), a sibling of the devices dir. A
// device's `<provider>:` block is only read for those, so a device key that
// merely happens to hold a map (`routing:`, `attributes:`) is never mistaken
// for one.
func providerNames(devicesDir string) []string {
	netsim := filepath.Dir(strings.TrimRight(devicesDir, string(filepath.Separator)))
	entries, err := os.ReadDir(filepath.Join(netsim, "providers"))
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yml") {
			out = append(out, strings.TrimSuffix(e.Name(), ".yml"))
		}
	}
	sort.Strings(out)
	return out
}

// dottedGet reads `a.b` from a map that may spell it either way netlab accepts:
// a literal "a.b" key or a nested a: {b: ...}.
func dottedGet(m map[string]any, path string) any {
	if v, ok := m[path]; ok {
		return v
	}
	head, rest, found := strings.Cut(path, ".")
	if !found {
		return nil
	}
	if child := asMap(m[head]); child != nil {
		return dottedGet(child, rest)
	}
	return nil
}

func parseStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func sortAttributes(as []Attribute) {
	sort.Slice(as, func(i, j int) bool {
		a, b := as[i], as[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return strings.Join(a.ValidValues, ",") < strings.Join(b.ValidValues, ",")
	})
}

// --- core attributes ------------------------------------------------------------

// readCore reads netsim/defaults/attributes.yml of the PATCHED tree (derived
// from the modules dir, like readCanBeFalse).
//
// Every top-level key is classified, and an unclassifiable one fails the run: a
// dict section is a scope, a `_`-prefixed dict is a reusable shape, a list is a
// control switch. Anything else is a netlab change a human must look at.
func readCore(modulesDir string) (*Core, error) {
	netsim := filepath.Dir(strings.TrimRight(modulesDir, string(filepath.Separator)))
	path := filepath.Join(netsim, "defaults", "attributes.yml")
	doc, err := readYAMLMap(path)
	if err != nil {
		return nil, fmt.Errorf("reading core attributes %s: %w", path, err)
	}
	core := &Core{Control: map[string][]string{}, Types: map[string]string{}}
	names := make([]string, 0, len(doc))
	for k := range doc {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		body := doc[name]
		switch {
		case strings.HasPrefix(name, "_"):
			sh := CoreShape{Name: name}
			if m := asMap(body); m != nil {
				if d, ok := m["_description"]; ok {
					sh.Description = strings.TrimSpace(fmt.Sprint(d))
				}
			}
			sh.Attributes = flattenAttributes("shape:"+name, "", body)
			sortAttributes(sh.Attributes)
			core.Shapes = append(core.Shapes, sh)
		case asMap(body) != nil:
			sc := CoreScope{Name: name, Attributes: flattenAttributes(name, "", body)}
			sortAttributes(sc.Attributes)
			core.Scopes = append(core.Scopes, sc)
		default:
			if _, isList := body.([]any); !isList {
				return nil, fmt.Errorf("%s: section %q is neither a dict (scope / shape) nor a list (control switch); classify it in readCore before regenerating", path, name)
			}
			core.Control[name] = stringList(body)
			sort.Strings(core.Control[name])
		}
	}
	// THE TYPE VOCABULARY: every type name a core attribute references, and what
	// netlab does with it.
	validators, err := validatorNames(netsim)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	collectTypeNames(doc, used)
	for t := range used {
		switch {
		case validators[t]:
			core.Types[t] = "validator"
		case doc[t] != nil || doc["_"+t] != nil:
			core.Types[t] = "shape"
		default:
			core.Types[t] = "special"
		}
	}
	return core, nil
}

// collectTypeNames gathers type names from `type:`, `_subtype:`, `_keytype:`,
// `_alt_types:` and from bare string leaves (`exec: _v_option`, `ipv4: str`).
func collectTypeNames(node any, out map[string]bool) {
	switch v := node.(type) {
	case map[string]any, map[any]any:
		for k, child := range asMap(v) {
			switch k {
			case "type", "_subtype", "_keytype":
				if s, ok := child.(string); ok {
					out[s] = true
				}
			case "_alt_types":
				for _, s := range stringList(child) {
					out[s] = true
				}
			default:
				if s, ok := child.(string); ok && !strings.HasPrefix(k, "_") {
					out[s] = true
				}
			}
			collectTypeNames(child, out)
		}
	case []any:
		for _, child := range v {
			collectTypeNames(child, out)
		}
	}
}

var (
	validatorDefRE   = regexp.MustCompile(`(?m)^def must_be_([A-Za-z0-9_]+)\(`)
	validatorRegisRE = regexp.MustCompile(`register_type\(\s*['"]([A-Za-z0-9_]+)['"]`)
)

// validatorNames are the types netlab validates: the must_be_<name> functions of
// netsim/data/types.py (plus literal register_type calls). netlab resolves a
// type by looking that name up, so this is its own vocabulary, not a copy.
func validatorNames(netsim string) (map[string]bool, error) {
	blob, err := os.ReadFile(filepath.Join(netsim, "data", "types.py"))
	if err != nil {
		return nil, fmt.Errorf("reading netlab type validators: %w", err)
	}
	out := map[string]bool{}
	for _, re := range []*regexp.Regexp{validatorDefRE, validatorRegisRE} {
		for _, m := range re.FindAllStringSubmatch(string(blob), -1) {
			out[m[1]] = true
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("found no must_be_* validators in netsim/data/types.py; the type vocabulary cannot be derived")
	}
	return out, nil
}

// resolveCopyRequests applies netlab's `copy:` / `merge:` attribute requests the
// way netsim/augment/config.py copy_merge_attributes does, so the manifest
// publishes the attribute netlab validates rather than the request for it.
//
// `bgp.node.activate: {copy: global}` means "node.activate has the definition
// global.activate has". Walking it as a plain map published `activate.copy` with
// type "global" -- 58 attribute paths no author can write -- and lost the real
// type and valid values. Faithful to netlab, including its limit: only an entry
// DIRECTLY under a namespace is resolved (attr[ns][name]); a request nested
// deeper (bonding.mode under interface) is left as written, because netlab
// leaves it too. The parity probe runs netlab's own function over the same
// file and fails on any difference.
func resolveCopyRequests(attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return attrs
	}
	out := make(map[string]any, len(attrs))
	for ns, body := range attrs {
		if m := asMap(body); m != nil {
			cp := make(map[string]any, len(m))
			for k, v := range m {
				cp[k] = v
			}
			out[ns] = cp
		} else {
			out[ns] = body
		}
	}
	nsNames := make([]string, 0, len(out))
	for ns := range out {
		nsNames = append(nsNames, ns)
	}
	sort.Strings(nsNames)
	// A request whose source is itself still a request waits for a later pass;
	// four passes is far more chain depth than netlab's files have, and an
	// unresolved request stays visible (its `copy` key) rather than vanishing.
	for pass := 0; pass < 4; pass++ {
		changed := false
		for _, ns := range nsNames {
			nsMap, ok := out[ns].(map[string]any)
			if !ok {
				continue
			}
			for name, entry := range nsMap {
				em := asMap(entry)
				if em == nil {
					continue
				}
				srcNS, _ := em["copy"].(string)
				isCopy := srcNS != ""
				if !isCopy {
					srcNS, _ = em["merge"].(string)
				}
				if srcNS == "" {
					continue
				}
				srcMap, ok := out[srcNS].(map[string]any)
				if !ok {
					continue
				}
				srcEntry, ok := srcMap[name]
				if !ok || hasCopyRequest(srcEntry) {
					continue
				}
				if isCopy {
					nsMap[name] = srcEntry
				} else {
					rest := make(map[string]any, len(em))
					for k, v := range em {
						if k != "merge" {
							rest[k] = v
						}
					}
					nsMap[name] = mergeMaps(srcEntry, rest)
				}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return out
}

func hasCopyRequest(v any) bool {
	m := asMap(v)
	if m == nil {
		return false
	}
	_, c := m["copy"].(string)
	_, g := m["merge"].(string)
	return c || g
}

// mergeMaps is Box's `a + b`: b wins, maps merge recursively.
func mergeMaps(a, b any) any {
	am, bm := asMap(a), asMap(b)
	if am == nil || bm == nil {
		return b
	}
	out := make(map[string]any, len(am)+len(bm))
	for k, v := range am {
		out[k] = v
	}
	for k, v := range bm {
		if existing, ok := out[k]; ok {
			out[k] = mergeMaps(existing, v)
		} else {
			out[k] = v
		}
	}
	return out
}

// dropNilLeaves removes every key whose value is null, recursively, keeping the
// (possibly emptied) maps that held them.
//
// This is what NETLAB does to a plugin's defaults: merge_plugin_defaults folds
// them into the topology with Box addition, and Box addition discards null
// leaves (`node: {bonding: {mode: }}` arrives as `node: {bonding: {}}`). Modules
// are read from file, not merged this way, and keep their nil leaves. The
// manifest reports what netlab validates against, so plugin attributes get the
// same treatment; the parity probe loads them through load_plugin_defaults and
// fails if the two ever disagree.
func dropNilLeaves(node any) any {
	m := asMap(node)
	if m == nil {
		return node
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if v == nil {
			continue
		}
		out[k] = dropNilLeaves(v)
	}
	return out
}

// ossManifestSource is the provenance line of the public manifest.
const ossManifestSource = "netlab/netsim/{modules,extra,devices} + patched netsim/defaults/attributes.yml global (generated from upstream netlab with the public patch series applied)"
