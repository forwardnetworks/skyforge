package netlabcaps

import (
	"fmt"
	"sort"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcapsgen"
)

// PER-DEVICE PLUGIN COVERAGE, and why it is computed rather than read.
//
// The manifest records, per plugin, two SEPARATE lists: `Devices` (the devices
// that declare `features.<plugin>` in the plugin's defaults.yml) and `Renders`
// (the devices with a non-empty `<plugin-dir>/<device>.j2`). Neither list is the
// answer to "can THIS device do this plugin", because BOTH resolve along the
// device parent chain at netlab runtime:
//
//   - `features` is merged from the parent device into the child, so `iol`
//     (parent: ios) carries ios's `features.ipsec` even though the ipsec plugin
//     names only `ios`.
//   - the config-template lookup walks the parent chain too, so an `ios.j2`
//     alone IS found for an `iol` node.
//
// MEASURED 2026-09-07 against the pinned netlab (f64bc4f3, patch series applied):
// deleting `netsim/extra/nat/iol.j2` renders BYTE-IDENTICAL NAT on an iol node,
// and `defaults.devices.iol.features.ipsec` resolves to ios's value. A comment in
// in the patch series asserted the opposite for both halves; it has been
// corrected in place. That false claim cost a day: it made `ipsec` on `iol` look
// like a platform gap, and a detector keyed on "is there an ipsec/iol.j2 file"
// would have reported that phantom gap with total confidence.
//
// So this file keys on (plugin, device) -> declared? renderable? RESOLVED
// THROUGH THE PARENT CHAIN, never on the presence of a `<device>.j2`. The
// iol/ipsec case is a NEGATIVE control in the tests: it must report NO gap.

// PluginSupport is the resolved verdict for one (device, plugin) pair.
//
// Declared and Renders are deliberately separate. They fail in opposite
// directions and a caller must be able to tell them apart:
//
//   - Declared && !Renders -> netlab ACCEPTS the topology and emits nothing.
//     The lab comes up green and does not do the thing. Silent no-op.
//   - !Declared && Renders -> netlab REJECTS the node ("does not support"),
//     while a purpose-built template for it sits unused in the tree.
//
// DeclaredVia/RendersVia name the device the answer actually came from, which is
// the device itself for a direct declaration and an ancestor otherwise. A reader
// who cannot see that `iol` got its answer from `ios` will re-derive the phantom
// gap above.
type PluginSupport struct {
	Plugin string
	Device string

	Declared bool
	Renders  bool

	// Gates records whether the plugin performs a per-device feature check at
	// all — i.e. whether its defaults.yml has a `devices:` block. It changes the
	// MEANING of the two booleans above, so it is part of the verdict rather
	// than a detail of the detector.
	Gates bool

	// DeclaredVia / RendersVia is the device in the parent chain that supplied
	// the answer. Empty when the corresponding boolean is false.
	DeclaredVia string
	RendersVia  string
}

// Supported reports whether netlab will run this plugin on this device.
//
// THE ANSWER DEPENDS ON WHETHER THE PLUGIN GATES, and conflating the two cases
// produced seven confident false positives on `iol` alone:
//
//   - A GATING plugin (one with a `devices:` block) admits a node iff the device
//     is declared. Rendering may then happen from a `<device>.j2` OR entirely
//     from Python via api.node_config — `mlag.vtep` declares two devices and
//     ships no templates at all — so the DECLARATION is the verdict.
//   - A NON-GATING plugin performs no feature check, so no device is ever
//     "undeclared" there; a template is what makes it render. bgp.policy,
//     bgp.session, ospf.areas and tunnel.gre are all in this bucket, and judging
//     them by declaration reports every working device as broken.
func (s PluginSupport) Supported() bool {
	if s.Gates {
		return s.Declared
	}
	return s.Renders
}

// Blocked reports the one state this package can PROVE is broken: a gating
// plugin that ships a template for this device and does not declare it, so
// netlab rejects the node with "does not support" while its own purpose-built
// template sits unused. This is the only condition worth calling a platform gap
// to an author's face, and the only one worth filing without a human first.
func (s PluginSupport) Blocked() bool { return s.Gates && s.Renders && !s.Declared }

// Inherited reports whether either half of the answer came from an ancestor
// rather than the device itself.
func (s PluginSupport) Inherited() bool {
	via := s.supportVia()
	return via != "" && via != s.Device
}

// Explain renders the verdict as one sentence naming the plugin and the device.
//
// ABSENCE IS STATED, NEVER OMITTED. The whole reason this package grew a
// per-device plugin surface is that the capability manifest answered "does this
// device do IPsec" with silence, an author read the silence as "unsupported",
// and hand-wrote CLI that a netlab primitive already generates. A manifest that
// omits what a device cannot do is not a manifest, it is a rumour.
func (s PluginSupport) Explain() string {
	switch {
	case s.Blocked():
		return fmt.Sprintf("%s: a template for %s exists but %s does NOT declare the feature — netlab will reject the node with \"does not support\"", s.Plugin, s.RendersVia, s.Device)
	case s.Supported():
		via := ""
		if s.Inherited() {
			via = fmt.Sprintf(" (inherited from %s)", s.supportVia())
		}
		return fmt.Sprintf("%s: SUPPORTED on %s%s", s.Plugin, s.Device, via)
	case s.Gates && s.Declared && !s.Renders:
		return fmt.Sprintf("%s: %s declares it and no template renders it; it may still render from the plugin API", s.Plugin, s.Device)
	default:
		return fmt.Sprintf("%s: NOT declared by %s", s.Plugin, s.Device)
	}
}

// supportVia names the device the supporting half actually came from.
func (s PluginSupport) supportVia() string {
	if s.Gates {
		return s.DeclaredVia
	}
	return s.RendersVia
}

// deviceChain returns the device and its ancestors, nearest first. The second
// value is false when the device is not in the manifest at all — an unknown
// device is UNKNOWN, not unsupported, and collapsing the two is how a typo gets
// reported as a platform gap.
func deviceChain(device string) ([]string, bool) {
	load()
	name := strings.TrimSpace(device)
	var chain []string
	seen := map[string]bool{}
	for name != "" && !seen[name] {
		seen[name] = true
		d, ok := index.devices[name]
		if !ok {
			break
		}
		chain = append(chain, name)
		name = strings.TrimSpace(d.Parent)
	}
	return chain, len(chain) > 0
}

// DevicePluginSupport resolves one (device, plugin) pair through the parent
// chain. The second return value is false when the device or the plugin is not
// in the manifest, or the manifest did not load.
func DevicePluginSupport(device, plugin string) (PluginSupport, bool) {
	load()
	if !Loaded() {
		return PluginSupport{}, false
	}
	plugin = strings.TrimSpace(plugin)
	p, ok := index.plugins[plugin]
	if !ok {
		return PluginSupport{}, false
	}
	chain, ok := deviceChain(device)
	if !ok {
		return PluginSupport{}, false
	}
	return resolve(p, chain), true
}

func resolve(p *netlabcapsgen.Plugin, chain []string) PluginSupport {
	declared := map[string]bool{}
	for _, d := range p.Devices {
		declared[d] = true
	}
	renders := map[string]bool{}
	for _, d := range p.Renders {
		renders[d] = true
	}
	out := PluginSupport{Plugin: p.Name, Device: chain[0], Gates: len(p.Devices) > 0}
	for _, d := range chain {
		if !out.Declared && declared[d] {
			out.Declared, out.DeclaredVia = true, d
		}
		if !out.Renders && renders[d] {
			out.Renders, out.RendersVia = true, d
		}
	}
	return out
}

// DevicePlugins returns the resolved verdict for EVERY plugin in the manifest
// against one device, sorted by plugin name. The second return value is false
// when the device is unknown or the manifest did not load.
//
// Every plugin appears, including the ones the device does not declare. That is
// the point: the caller is expected to render the negatives, because the failure
// this replaces was a reader concluding "unsupported" from a list that simply
// did not mention the plugin.
func DevicePlugins(device string) ([]PluginSupport, bool) {
	load()
	if !Loaded() {
		return nil, false
	}
	chain, ok := deviceChain(device)
	if !ok {
		return nil, false
	}
	out := make([]PluginSupport, 0, len(manifest.Plugins))
	for i := range manifest.Plugins {
		out = append(out, resolve(&manifest.Plugins[i], chain))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Plugin < out[j].Plugin })
	return out, true
}

// DevicePluginsSupporting returns only the plugins this device can actually run.
// Use it for a short "what CAN this device do" line; use DevicePlugins when the
// answer must also state what it cannot.
func DevicePluginsSupporting(device string) []string {
	all, ok := DevicePlugins(device)
	if !ok {
		return nil
	}
	out := []string{}
	for _, s := range all {
		if s.Supported() {
			out = append(out, s.Plugin)
		}
	}
	return out
}

// --- coverage gaps ----------------------------------------------------------

// GapKind names the shape of a coverage gap. The two real kinds fail in opposite
// directions and need different fixes, so they are never collapsed.
type GapKind string

const (
	// GapDeclaredNotRendered: the device declares the feature and nothing renders
	// it. netlab validates the author's config, reports the feature supported,
	// and emits nothing. The lab comes up green and does not do the thing.
	GapDeclaredNotRendered GapKind = "declared-not-rendered"

	// GapRenderedNotDeclared: a template for the device exists and the device does
	// not declare the feature, so the plugin rejects the node with "does not
	// support" while its own purpose-built template sits unused.
	GapRenderedNotDeclared GapKind = "rendered-not-declared"

	// GapOrphanTemplate: a `<name>.j2` whose `<name>` is not a netlab device at
	// all. Nothing can ever reach it. Reported separately because the fix is to
	// rename or delete the template, not to add a declaration.
	GapOrphanTemplate GapKind = "orphan-template"
)

// CoverageGap is one (plugin, device) pair the manifest can prove is broken.
//
// It carries what an ISSUE needs to be actionable rather than a notification:
// which plugin, which device, which half is missing, and the files a fix would
// touch. A detector that cannot fill these in must stay silent — a tracker full
// of vague issues is worse than the silence this replaces.
type CoverageGap struct {
	Plugin string
	Device string
	Kind   GapKind

	// Files a fix would touch, relative to the netlab tree root.
	Files []string
}

// Fileable reports whether this gap is certain enough to open an issue for
// WITHOUT a human first looking at it.
//
// Only GapRenderedNotDeclared and GapOrphanTemplate qualify, and the asymmetry
// is measured, not stylistic:
//
//   - RenderedNotDeclared is provable from the manifest alone. The plugin gates
//     on a `devices:` declaration, a template for this device exists, and no
//     device in its parent chain is declared. netlab WILL reject the node.
//   - OrphanTemplate is provable too: the name is not a device in this tree.
//   - DeclaredNotRendered is NOT provable, because a plugin can render through
//     the plugin API instead of a `<device>.j2`. `mlag.vtep` declares two devices
//     and ships NO templates at all, rendering entirely from Python via
//     api.node_config(); `ipsec` itself writes node.ipsec.tunnels the same way.
//     Filing that class would open issues against working plugins.
//
// So DeclaredNotRendered is reported to a human and never auto-filed. A detector
// confident enough to file and wrong is the failure this whole surface exists to
// avoid.
func (g CoverageGap) Fileable() bool {
	return g.Kind == GapRenderedNotDeclared || g.Kind == GapOrphanTemplate
}

// Title is the stable, human-first summary. It is also what the issue de-dupe
// hashes, so it must depend ONLY on the gap identity (plugin, device, kind) and
// never on when or where the gap was noticed.
func (g CoverageGap) Title() string {
	switch g.Kind {
	case GapDeclaredNotRendered:
		return fmt.Sprintf("netlab %s plugin: %s declares the feature but nothing renders it", g.Plugin, g.Device)
	case GapRenderedNotDeclared:
		return fmt.Sprintf("netlab %s plugin: template for %s exists but %s does not declare the feature", g.Plugin, g.Device, g.Device)
	case GapOrphanTemplate:
		return fmt.Sprintf("netlab %s plugin: template %s.j2 names no netlab device", g.Plugin, g.Device)
	}
	return fmt.Sprintf("netlab %s plugin: coverage gap on %s", g.Plugin, g.Device)
}

// Detail is the body an issue or a warning shows: what is wrong, what it costs,
// and what a fix looks like.
func (g CoverageGap) Detail() string {
	var b strings.Builder
	pdir := "netsim/extra/" + strings.ReplaceAll(g.Plugin, ".", "/")
	switch g.Kind {
	case GapDeclaredNotRendered:
		fmt.Fprintf(&b, "Device %q declares features.%s in %s/defaults.yml, and neither it nor any device in its parent chain has a template in %s/.\n\n", g.Device, g.Plugin, pdir, pdir)
		b.WriteString("EFFECT: netlab validates the author's attributes, reports the feature supported, and emits no configuration. The lab comes up green and does not do what was asked.\n\n")
		fmt.Fprintf(&b, "FIX: add %s/%s.j2, or remove the declaration if the device genuinely cannot do it.\n", pdir, g.Device)
	case GapRenderedNotDeclared:
		fmt.Fprintf(&b, "A template %s/%s.j2 exists, but neither %q nor any device in its parent chain declares features.%s in %s/defaults.yml.\n\n", pdir, g.Device, g.Device, g.Plugin, pdir)
		b.WriteString("EFFECT: the plugin rejects the node at compile time with \"does not support\", so the template can never run. An author reads that as \"the platform cannot do this\" and hand-writes device CLI instead.\n\n")
		fmt.Fprintf(&b, "FIX: add a `%s:` entry under `devices:` in %s/defaults.yml declaring features.%s.\n", g.Device, pdir, g.Plugin)
	case GapOrphanTemplate:
		fmt.Fprintf(&b, "%s/%s.j2 exists, but %q is not a netlab device in this tree.\n\n", pdir, g.Device, g.Device)
		b.WriteString("EFFECT: nothing can ever resolve this template. It is dead weight that reads as coverage.\n\n")
		b.WriteString("FIX: rename it to the real device name, or delete it.\n")
	}
	return b.String()
}

// PluginCoverageGaps returns every (plugin, device) pair the manifest can prove
// is broken, sorted and deterministic.
//
// SCOPE, stated because a gate that passes over the wrong scope proves nothing:
// this reads ONLY the generated manifest. It can see a declaration without a
// template and a template without a declaration. It CANNOT see a template that
// renders the wrong thing, a feature that fails on a particular image, or a
// plugin whose Python rejects a node for a reason other than the feature check
// (ipsec on asav additionally fails on "Need explicit interface name (ifname) for
// tunnel interface", which no manifest can predict). Those need a compile or a
// certification run.
func PluginCoverageGaps() []CoverageGap {
	load()
	if !Loaded() {
		return nil
	}
	var out []CoverageGap
	for i := range manifest.Plugins {
		p := &manifest.Plugins[i]

		// ONLY PLUGINS THAT GATE ON A DECLARATION CAN HAVE THIS KIND OF GAP.
		//
		// A plugin's defaults.yml exists ONLY to declare per-device support and is
		// optional (see netlabcapsgen.readPlugin). A plugin with no `devices:`
		// block performs no feature check, so "device X is not declared" is not a
		// defect there — it is the normal state of every device.
		//
		// Without this guard the detector reported bgp.policy and bgp.session as
		// broken on eos, ios, junos, frr and ten others: 100+ confident, wrong
		// findings from working upstream plugins, which the auto-filer would have
		// turned into 100+ issues. The declaration block IS the gate; no block, no
		// gap.
		if len(p.Devices) == 0 {
			continue
		}
		pdir := "netsim/extra/" + strings.ReplaceAll(p.Name, ".", "/")

		// A template naming a device that does not exist can never resolve.
		for _, r := range p.Renders {
			if skipTemplateName(r) {
				continue
			}
			if _, ok := index.devices[r]; !ok {
				out = append(out, CoverageGap{
					Plugin: p.Name, Device: r, Kind: GapOrphanTemplate,
					Files: []string{pdir + "/" + r + ".j2"},
				})
			}
		}

		// The two real halves, resolved per declaring/rendering device through
		// its own chain. Only devices the plugin NAMES are candidates: every
		// other device in the tree is legitimately uninvolved, and reporting
		// those would bury the four real findings under two thousand non-events.
		cand := map[string]bool{}
		for _, d := range p.Devices {
			cand[d] = true
		}
		for _, d := range p.Renders {
			if skipTemplateName(d) {
				continue
			}
			cand[d] = true
		}
		// `none` is netlab's validation stand-in, not a lab device. Every gating
		// plugin declares it ("# for validation" in evpn.multihoming's own
		// defaults.yml), so reporting it turns a real finding list into noise.
		delete(cand, "none")
		names := make([]string, 0, len(cand))
		for d := range cand {
			names = append(names, d)
		}
		sort.Strings(names)

		for _, d := range names {
			chain, ok := deviceChain(d)
			if !ok {
				continue // orphan template, already reported above
			}
			s := resolve(p, chain)
			switch {
			case s.Blocked():
				out = append(out, CoverageGap{
					Plugin: p.Name, Device: d, Kind: GapRenderedNotDeclared,
					Files: []string{pdir + "/defaults.yml"},
				})
			case s.Declared && !s.Renders:
				out = append(out, CoverageGap{
					Plugin: p.Name, Device: d, Kind: GapDeclaredNotRendered,
					Files: []string{pdir + "/defaults.yml", pdir + "/" + d + ".j2"},
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plugin != out[j].Plugin {
			return out[i].Plugin < out[j].Plugin
		}
		if out[i].Device != out[j].Device {
			return out[i].Device < out[j].Device
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// skipTemplateName reports whether a `<name>.j2` in a plugin directory is
// something OTHER than a per-device config template, and so can never be
// evidence of device coverage either way.
//
// Two conventions, both learned by getting them wrong first:
//
//   - a leading underscore marks a Jinja macro include (`_route_map_ios.j2`,
//     `_sample_bfd_template.j2`), included by other templates and never resolved
//     as a device name;
//   - `<device>-<provider>.j2` is a provider variant (`linux-clab.j2` is the
//     linux device under the clab provider), so the device is the part before
//     the dash and the file is not an orphan.
func skipTemplateName(name string) bool {
	if strings.HasPrefix(name, "_") {
		return true
	}
	if i := strings.LastIndex(name, "-"); i > 0 {
		if _, ok := index.devices[name[:i]]; ok {
			return true
		}
	}
	return false
}

// PluginForConstruct maps a well-known device-config construct to the netlab
// plugin that generates it. It is deliberately a SMALL, EXPLICIT table over the
// plugins our patch series ships, not a general attempt to understand CLI: a
// detector that guesses will be wrong, and a wrong detector that is confident
// enough to warn (or to file an issue) is worse than no detector.
//
// Returns "" when nothing in the table matches, which is the common case and
// must stay silent.
func PluginForConstruct(construct string) string {
	switch strings.ToLower(strings.TrimSpace(construct)) {
	case "ipsec", "crypto", "vpn":
		return "ipsec"
	case "nat":
		return "nat"
	case "firewall", "firewall.policy":
		return "firewall.policy"
	case "loadbalancer", "load-balancer":
		return "loadbalancer"
	}
	return ""
}

// --- per-attribute render coverage -------------------------------------------

// AttributeGap is one plugin attribute that some device templates name and others do not.
//
// WHY. `Declared`/`Renders` answer "does this device run the plugin"; the plugin's attribute schema answers "is this key
// legal". Neither says "does THIS device render THIS key". firewall.zonebased.rules is legal on every zonebased device
// and rendered only by panos: the other three accept it, compile with no error, and drop it (the rule
// existed nowhere on the device). The manifest then listed `rules` among fortios/vsrx/vyos "renders",
// which is how a designer or an assistant came to suggest it.
//
// EVIDENCE, NOT PROOF. The input is a word-bounded text match of the dotted path in each device template
// (netlabcapsgen.Plugin.AttributeRenders), so it cannot see an attribute rendered through Python-derived data. Only
// NODE-scope attributes are reported, because a template reads those by their full path; an interface-scope key
// (firewall.zone on panos) is consumed through derived data and would be a false claim. Treat a gap as "check the
// compile findings", never as a refusal.
type AttributeGap struct {
	Plugin     string
	Path       string
	NamedOn    []string // templates (devices) that name the attribute
	NotNamedOn []string // devices the plugin renders for whose templates do not
}

// PluginAttributeGaps returns the node-scope attributes of one plugin that only some of its device templates name.
// Sorted by path. Nil when the plugin is unknown, the manifest did not load, or every template agrees.
func PluginAttributeGaps(plugin string) []AttributeGap {
	load()
	if !Loaded() {
		return nil
	}
	p, ok := index.plugins[strings.TrimSpace(plugin)]
	if !ok || len(p.AttributeRenders) == 0 {
		return nil
	}
	scope := map[string]string{}
	for _, a := range p.Attributes {
		scope[a.Path] = a.Scope
	}
	var devs []string
	for _, d := range p.Renders {
		if d == "none" || skipTemplateName(d) || strings.Contains(d, ".initial") {
			continue
		}
		devs = append(devs, d)
	}
	var out []AttributeGap
	for path, named := range p.AttributeRenders {
		if scope[path] != "node" {
			continue
		}
		has := map[string]bool{}
		for _, d := range named {
			has[d] = true
		}
		var missing []string
		for _, d := range devs {
			covered := has[d]
			if chain, ok := deviceChain(d); ok && !covered {
				for _, c := range chain {
					covered = covered || has[c]
				}
			}
			if !covered {
				missing = append(missing, d)
			}
		}
		if len(missing) == 0 {
			continue
		}
		sort.Strings(missing)
		out = append(out, AttributeGap{Plugin: p.Name, Path: path, NamedOn: append([]string(nil), named...), NotNamedOn: missing})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
