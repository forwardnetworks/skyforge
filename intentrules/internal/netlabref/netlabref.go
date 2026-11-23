// Package netlabref renders netlab's documentation corpus and capability
// manifest into the text a person or a model reads.
//
// WHY IT IS ITS OWN PACKAGE. The tools that call it live in package skyforge,
// which is an Encore service: `go test` cannot run it. Pure rendering that
// decides what an SE is told has to live where a test can reach it -- the lesson
// internal/netlabdiag records, where an assertion written after a user hit an
// empty error toast TWICE had never once executed.
package netlabref

import (
	"fmt"
	"sort"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcapsgen"
	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabdocs"
)

// Docs searches or reads netlab's own documentation.
func Docs(search, path string) string {
	search, path = strings.TrimSpace(search), strings.TrimSpace(path)

	if path != "" {
		body, ok := netlabdocs.Read(path)
		if !ok {
			// NAME WHAT EXISTS. A bare "not found" invites the reader to guess another
			// plausible path; the near misses are what it actually needs.
			stem := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(path, ".md"), ".txt"), "/")
			if i := strings.LastIndex(stem, "/"); i >= 0 {
				stem = stem[i+1:]
			}
			return fmt.Sprintf("no such netlab doc %q. Closest matches: %s\nUse search to locate a page.",
				path, formatList(netlabdocs.Search(stem), 10))
		}
		return fmt.Sprintf("netlab doc %s (from the patched netlab tree)\n\n%s", path, body)
	}

	if search == "" {
		return "netlab docs corpus: " + formatList(docPaths(), 40) +
			"\n\nCall again with search=<terms> or path=<doc> to read one."
	}
	hits := netlabdocs.Search(strings.Fields(search)...)
	if len(hits) == 0 {
		// An empty result must not read as "netlab does not document this".
		return fmt.Sprintf("no netlab doc mentions all of %q. Try fewer or different terms; "+
			"the corpus has %d docs and is listed by calling this tool with no arguments.",
			search, len(netlabdocs.List()))
	}
	return fmt.Sprintf("netlab docs matching %q:\n%s\n\nRead one with path=<doc>.",
		search, formatList(hits, 25))
}

func docPaths() []string {
	entries := netlabdocs.List()
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Path)
	}
	sort.Strings(out)
	return out
}

func formatList(paths []string, max int) string {
	if len(paths) == 0 {
		return "(none)"
	}
	shown, suffix := paths, ""
	if len(shown) > max {
		shown = shown[:max]
		suffix = fmt.Sprintf("\n  ... and %d more", len(paths)-max)
	}
	return "\n  " + strings.Join(shown, "\n  ") + suffix
}

// Capabilities answers what netlab can be asked to do, for a module or a device.
func Capabilities(module, device string) string {
	m := netlabcaps.Manifest()
	var b strings.Builder

	if mod := strings.TrimSpace(module); mod != "" {
		renderModule(&b, m, mod)
	}
	if dev := strings.TrimSpace(device); dev != "" {
		renderDevice(&b, m, dev)
	}
	if b.Len() == 0 {
		fmt.Fprintf(&b, "netlab capability manifest (patched tree): %d modules, %d plugins, %d devices, %d Skyforge top-level blocks.\n"+
			"Modules: %s\nSkyforge top-level blocks (written at the top of the topology, not under module:): %s\n"+
			"Pass module=<name> or device=<key>.\n",
			len(m.Modules), len(m.Plugins), len(m.Devices), len(m.SkyforgeGlobals),
			strings.Join(ModuleNames(m), ", "), strings.Join(netlabcaps.SkyforgeGlobalNames(), ", "))
	}
	return b.String()
}

func renderModule(b *strings.Builder, m netlabcapsgen.Manifest, mod string) {
	for _, x := range m.Modules {
		if !strings.EqualFold(x.Name, mod) {
			continue
		}
		fmt.Fprintf(b, "module %s\n", x.Name)
		if len(x.Requires) > 0 {
			fmt.Fprintf(b, "  requires: %s\n", strings.Join(x.Requires, ", "))
		}
		scopes := map[string][]string{}
		for _, a := range x.Attributes {
			desc := a.Path
			if a.Type != "" {
				desc += " (" + a.Type + ")"
			}
			if len(a.ValidValues) > 0 {
				desc += " one of [" + strings.Join(a.ValidValues, ", ") + "]"
			}
			scopes[a.Scope] = append(scopes[a.Scope], desc)
		}
		keys := make([]string, 0, len(scopes))
		for k := range scopes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sort.Strings(scopes[k])
			fmt.Fprintf(b, "  scope %s: %s\n", k, strings.Join(scopes[k], ", "))
		}
		// The idioms an author reaches for when switching a module off on one link.
		if len(m.CanBeFalse) > 0 {
			fmt.Fprintf(b, "  `%s: false` is legal at scope: %s\n", x.Name, strings.Join(m.CanBeFalse, ", "))
		}
		if len(x.CanBeTrue) > 0 {
			fmt.Fprintf(b, "  `%s: true` is legal at scope: %s\n", x.Name, strings.Join(x.CanBeTrue, ", "))
		}
		if len(x.ControlSections) > 0 {
			fmt.Fprintf(b, "  (netlab framework switches, NOT authorable: %s)\n",
				strings.Join(x.ControlSections, ", "))
		}
		return
	}
	// A PLUGIN ASKED FOR AS A MODULE IS NOT AN UNKNOWN NAME.
	//
	// `netlab_capabilities module=ipsec` used to answer "no netlab module named
	// ipsec" and then list the modules — a technically true sentence that reads
	// as "this platform cannot do IPsec". It is a plugin, it exists, and it has
	// shipped for months. An author read that answer, concluded IPsec was
	// unsupported, and hand-wrote Cisco crypto CLI into three nodes.
	//
	// So a name that exists in the OTHER namespace is answered from that
	// namespace instead of being reported missing.
	if netlabcaps.HasPlugin(mod) {
		renderPlugin(b, m, mod)
		// A name can be BOTH a plugin and a top-level block: the plugin is
		// asked for as a module, and the block is where an author declares
		// its data. Answering only the plugin half says "no device renders it"
		// and never mentions the block, which reads as "not a thing here".
		if g, ok := netlabcaps.SkyforgeGlobal(mod); ok {
			b.WriteString("\n")
			renderSkyforgeGlobal(b, g)
		}
		return
	}
	// A TOP-LEVEL BLOCK IS NOT AN UNKNOWN NAME EITHER.
	//
	// Blocks such as `otg:` are neither modules nor plugins: they are written
	// at the top of the topology and validated by netlab from the patched
	// attribute schema. Answering "no netlab module named otg" for one of
	// them is the same wrong sentence as for a plugin, and nothing could say
	// what fields go in it.
	if g, ok := netlabcaps.SkyforgeGlobal(mod); ok {
		renderSkyforgeGlobal(b, g)
		return
	}
	fmt.Fprintf(b, "no netlab module named %q. Modules: %s\n", mod, strings.Join(ModuleNames(m), ", "))
	if near := netlabcaps.NearestNames(mod, netlabcaps.SkyforgeGlobalNames()); len(near) > 0 {
		fmt.Fprintf(b, "  (Skyforge top-level blocks with similar names — written at the top of the topology, not under `module:`: %s)\n", strings.Join(near, ", "))
	}
	if near := netlabcaps.NearestNames(mod, netlabcaps.PluginNames()); len(near) > 0 {
		fmt.Fprintf(b, "  (plugins with similar names — `plugin:`, not `module:`: %s)\n", strings.Join(near, ", "))
	}
}

// renderSkyforgeGlobal answers for one of the top-level blocks Skyforge owns.
// Only the field shape is rendered, and only from the manifest: the block's
// semantics (what a peering's `speaker` must be, what an AP's `uplink.port` is
// named after) live in netlab_docs, where a human edits them.
func renderSkyforgeGlobal(b *strings.Builder, g *netlabcapsgen.SkyforgeGlobal) {
	fmt.Fprintf(b, "%s  (a SKYFORGE top-level block — write it at the top of the topology as `%s:`, not under `module:` or `plugin:`; "+
		"netlab validates it from the runtime defaults, Skyforge consumes it)\n", g.Name, g.Name)
	paths := make([]string, 0, len(g.Attributes))
	for _, a := range g.Attributes {
		desc := a.Path
		if a.Type != "" {
			desc += " (" + a.Type + ")"
		}
		if len(a.ValidValues) > 0 {
			desc += " one of [" + strings.Join(a.ValidValues, ", ") + "]"
		}
		paths = append(paths, desc)
	}
	sort.Strings(paths)
	fmt.Fprintf(b, "  fields (`[]` marks one entry of a list): %s\n", strings.Join(paths, ", "))
	fmt.Fprintf(b, "  see netlab_docs for what each field means and a worked example (`netlab_docs %s`).\n", g.Name)
}

// renderPlugin answers for a plugin: which devices can actually run it, and
// which are declared-but-unrenderable or rendered-but-undeclared. Both halves
// are stated, because a plugin that a device declares and nothing renders is a
// silent no-op and a plugin with a template and no declaration is a hard reject,
// and an author needs to tell those apart.
func renderPlugin(b *strings.Builder, m netlabcapsgen.Manifest, name string) {
	fmt.Fprintf(b, "plugin %s  (a PLUGIN — write it in `plugin: [ %s ]`, not in `module:`)\n", name, name)
	// The plugin's OWN attribute declarations: names, types, nesting, required.
	// Without them an author guesses the shape of the plugin's block, and a wrong
	// guess can surface as a Python traceback from inside the plugin.
	if attrs, ok := netlabcaps.PluginAttributes(name); ok && len(attrs) > 0 {
		fmt.Fprintf(b, "  attribute types netlab declares for it (from its defaults.yml; `required` means netlab refuses the plugin without it; a `[]` marks a list entry): %s\n",
			Describe(attrsFromManifest(attrs)))
		if doc := "plugins/" + name + ".md"; docExists(doc) {
			fmt.Fprintf(b, "  read what each means: netlab_docs path=%s\n", doc)
		}
	}
	// Per-ATTRIBUTE coverage: an attribute can be legal on every device the plugin runs on and rendered by only some of them.
	// Stating it here is what keeps a designer or an assistant from offering a key the target device accepts and drops.
	for _, g := range netlabcaps.PluginAttributeGaps(name) {
		fmt.Fprintf(b, "  CAUTION %s is named only by the %s template(s), not by %s: netlab accepts it on those devices and may render nothing "+
			"(template evidence; a compile prints a warning for the known cases). Prefer the module-level equivalent on them.\n",
			g.Path, strings.Join(g.NamedOn, "/"), strings.Join(g.NotNamedOn, ", "))
	}
	var supported, broken []string
	gates := false
	for _, d := range m.Devices {
		s, ok := netlabcaps.DevicePluginSupport(d.Name, name)
		if !ok {
			continue
		}
		gates = s.Gates
		switch {
		case s.Supported():
			via := d.Name
			if s.Inherited() {
				via = fmt.Sprintf("%s (via %s)", d.Name, s.RendersVia)
			}
			supported = append(supported, via)
		case s.Blocked():
			// ONLY the provable class. A gating plugin that declares a device and
			// renders it from Python (mlag.vtep) is working, not broken, and
			// telling an author to file it wastes their time and the tracker's.
			broken = append(broken, d.Name)
		}
	}
	if len(supported) > 0 {
		fmt.Fprintf(b, "  devices that RENDER it: %s\n", strings.Join(supported, ", "))
	} else if !gates {
		// A plugin with no per-device declaration and no per-device template (a
		// topology-expanding plugin) is not "unrunnable everywhere": it works on the
		// topology, not on a device type.
		b.WriteString("  devices that RENDER it: not device-specific — the plugin neither gates on a device type nor ships per-device templates (it rewrites the topology or other plugins' data).\n")
	} else {
		b.WriteString("  devices that RENDER it: NONE — the plugin exists but no device can currently run it.\n")
	}
	if len(broken) > 0 {
		fmt.Fprintf(b, "  devices netlab REJECTS despite shipping a template for them: %s\n", strings.Join(broken, ", "))
		b.WriteString("  That is a PLATFORM GAP, not a device limitation — please file it rather than hand-writing config.\n")
	}
	b.WriteString("  NOTE: plugin support is declared per device TYPE, not per image, and a plugin can still " +
		"reject a node for reasons no manifest can see (ipsec on asav also needs an explicit tunnel `ifname`).\n")
}

func renderDevice(b *strings.Builder, m netlabcapsgen.Manifest, dev string) {
	for _, d := range m.Devices {
		if !strings.EqualFold(d.Name, dev) {
			continue
		}
		fmt.Fprintf(b, "device %s\n", d.Name)
		if d.Parent != "" {
			fmt.Fprintf(b, "  inherits: %s\n", d.Parent)
		}
		fmt.Fprintf(b, "  modules: %s\n", strings.Join(d.Modules, ", "))
		if len(d.Features) > 0 {
			fmt.Fprintf(b, "  features declared supported: %s\n", strings.Join(d.Features, ", "))
			b.WriteString("  NOTE: declared per device TYPE, not per image. A feature listed here can " +
				"still fail on a particular image version; that needs a certification run, not this manifest.\n")
		}
		renderDevicePlugins(b, d.Name)
		return
	}
	fmt.Fprintf(b, "no netlab device named %q.\n", dev)
}

// renderDevicePlugins states, for one device, which plugins it renders AND which
// it does not.
//
// ABSENCE IS STATED, NEVER OMITTED, and that is the entire point of this
// function. Before it existed, `netlab_capabilities device=iol` listed modules
// and features and said NOTHING about plugins — so an author asking "can this
// device do IPsec" got silence, read the silence as "no", and hand-wrote crypto
// CLI that the `ipsec` plugin generates for that exact device. Silence is what a
// manifest exists to replace; a manifest that omits what a device cannot do is
// not a manifest, it is a rumour.
func renderDevicePlugins(b *strings.Builder, dev string) {
	all, ok := netlabcaps.DevicePlugins(dev)
	if !ok {
		return
	}
	var yes, gaps []string
	for _, s := range all {
		switch {
		case s.Supported():
			n := s.Plugin
			if s.Inherited() {
				n = fmt.Sprintf("%s (via %s)", s.Plugin, s.RendersVia)
			}
			yes = append(yes, n)
		case s.Blocked():
			// The plugin ships a template for this device and does not declare it,
			// so netlab rejects the node. The author did nothing wrong and cannot
			// fix it in their topology. Nothing weaker than Blocked() belongs
			// here: seven working upstream plugins were reported as gaps on iol
			// before this used the provable condition.
			gaps = append(gaps, s.Plugin)
		}
	}
	if len(yes) > 0 {
		fmt.Fprintf(b, "  plugins this device RENDERS: %s\n", strings.Join(yes, ", "))
	} else {
		b.WriteString("  plugins this device RENDERS: none\n")
	}
	if len(gaps) > 0 {
		fmt.Fprintf(b, "  plugins that REJECT this device despite shipping a template for it: %s\n", strings.Join(gaps, ", "))
		b.WriteString("  Those are PLATFORM GAPS, not device limitations — please file them rather than hand-writing config.\n")
	}
	b.WriteString("  Every other plugin does NOT declare this device: it is not omitted, it is absent. " +
		"Ask `netlab_capabilities module=<plugin>` for one plugin's device coverage.\n")
}

// ModuleNames lists the manifest's modules, sorted.
func ModuleNames(m netlabcapsgen.Manifest) []string {
	out := make([]string, 0, len(m.Modules))
	for _, x := range m.Modules {
		out = append(out, x.Name)
	}
	sort.Strings(out)
	return out
}
