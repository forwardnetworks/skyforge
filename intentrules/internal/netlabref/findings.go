package netlabref

// findings.go -- the STATIC layer's finding classifier.
//
// netlab compiles a great many topologies that cannot work, and rejects others with a message that names the
// symptom and not the cause. This turns what `netlab create` printed ("Warning in bgp: ...", "IncorrectValue in
// groups: ...") into findings a person or a model can act on: what it means, how bad it is, how to fix it.
//
// ONE TABLE, EVERY SURFACE. The validate path (portal, designer, CLI, API, assistant tools) all
// read the same NetlabValidationResult.Diagnostics, and Diagnostics are built here. A new footgun is one row in
// findingRules, not a change in five places, and the surfaces cannot disagree.
//
// THREE CLASSES, and the split is the point:
//
//	will-not-work   netlab compiled it, and the lab it describes cannot do what its modules say (iBGP/EVPN with no IGP:
//	                the sessions never come up). Shown first, and the class a deploy gate may block on.
//	likely-wrong    netlab ignored or dropped something the author wrote (a module attribute on a link nobody uses,
//	                a module pruned from a node). Usually a mistake, sometimes intended: shown, never blocking.
//	informational   housekeeping (a module removed because the node uses no VRFs, a renamed attribute). Not shown in the
//	                diagnostics; counted for telemetry.
//
// netlab ERRORS are a fourth case: the compile failed, so they are not a class, but the common ones get the same
// treatment (a cause and a fix) because "Cannot use module attribute in group X that has no members" does not say that
// the plugin which would have created the members is not enabled.
//
// The patterns are netlab's own text, measured on the pinned, patched tree (a check script renders
// them live and fails when a pattern stops matching, so a netlab bump cannot silently blind this table). Rules are
// matched in order, first match wins; an unmatched warning is "likely-wrong" (netlab only warns when it did something
// the author may not expect), an unmatched error becomes "netlab-error" carrying netlab's own words.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	ClassWillNotWork   = "will-not-work"
	ClassLikelyWrong   = "likely-wrong"
	ClassInformational = "informational"
)

// Finding is one thing worth telling the author about a compile.
type Finding struct {
	Code    string   // stable, kebab-case; the telemetry label
	Class   string   // ClassWillNotWork | ClassLikelyWrong | ClassInformational
	IsError bool     // netlab refused the topology (the compile failed) rather than warned
	Module  string   // the netlab module/plugin that reported it ("bgp", "groups", ...)
	Subject []string // nodes, groups or attributes named, de-duplicated and sorted
	Message string   // what netlab said, with the subjects folded in
	Why     string   // why it matters, generic
	Fix     string   // what to change, generic
	// Path locates the finding in the authored document ("nodes.r1", "links[2]") so the designer can attach it inline.
	// netlab's own output names objects, not positions, so it is empty on findings that came from a compile.
	Path string
	// Source says who produced the finding: SourceNetlab (a compile printed it) or SourceIntentRules (the static
	// rules layer, internal/intentrules, read the document before any compile). Empty reads as SourceNetlab.
	Source string
}

const (
	SourceNetlab      = "netlab"
	SourceIntentRules = "intentrules"
)

// RuleText returns the title template, why and fix text of a code in the table, for the static rules layer: a code that
// both layers can emit has ONE text, here, so the two can never disagree about what it means. ok is false for a code the
// table does not know.
func RuleText(code string) (title, why, fix string, ok bool) {
	r := ruleByCode(code)
	if r == nil {
		return "", "", "", false
	}
	return r.title, r.why, r.fix, true
}

type findingRule struct {
	code    string
	class   string
	warning bool           // matches "Warning in" lines; false matches the error categories
	cat     string         // when set, the netlab error category the rule is limited to (MissingDependency, ...)
	re      *regexp.Regexp // against the text after "<Cat> in <module>: "; (?P<s>...) captures the subject
	title   string         // %s is the subject list; "" uses netlab's text
	why     string
	fix     string
}

func rx(s string) *regexp.Regexp { return regexp.MustCompile(s) }

var findingRules = []findingRule{
	// ---- will-not-work (compiled, cannot work) ----
	{code: "ibgp-without-igp", class: ClassWillNotWork, warning: true,
		re:    rx(`^Node (?P<s>\S+) has IBGP sessions but no IGP`),
		title: "iBGP or EVPN sessions on %s but no IGP module",
		why:   "iBGP and EVPN sessions run between loopback addresses. Without an IGP nothing advertises the loopbacks, so the sessions never come up and no overlay or route exchange follows. netlab compiles it anyway.",
		fix:   "Add an IGP (ospf, isis or eigrp) to the module list of these nodes. A node's own module: list replaces the topology-level list, and a group list is not guaranteed to inherit it (upstream netlab replaces it), so name the full set for each role (for example spines: [ ospf, bgp, evpn ]). If static routes really carry the loopbacks, say so with defaults.bgp.warnings.missing_igp: False."},
	{code: "evpn-extended-communities-off", class: ClassWillNotWork, warning: true,
		re:    rx(`^Extended BGP communities are not enabled on \S+ BGP sessions on (?P<s>\S+)`),
		title: "EVPN needs extended communities, which are off for the session type EVPN uses on %s",
		why:   "EVPN routes ride in BGP extended communities. If the session type that carries EVPN has them disabled the routes are dropped and no VXLAN tunnel is built.",
		fix:   "Enable extended communities for that session type (bgp.community.ibgp / bgp.community.ebgp: [ standard, extended ]) or move EVPN to the session type that has them (evpn.session)."},
	{code: "vpn-without-mpls-vpn", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Node (?P<s>\S+) has VRFs and the bgp\+mpls modules but no mpls\.vpn`),
		title: "VRFs with bgp and mpls on %s but no mpls.vpn",
		why:   "Without mpls.vpn no VPN address family is configured, so VRF routes are not exchanged between the PEs (expected only when EVPN or a custom template carries the VRFs).",
		fix:   "Set mpls.vpn on these nodes (or their group) for the VPN flavour you want (see netlab's hint)."},
	// ---- likely-wrong (netlab ignored or dropped something) ----
	{code: "igp-on-no-interface", class: ClassLikelyWrong, warning: true,
		re:    rx(`^(?P<s>\S+) does not use (?:ospf|isis|eigrp|ripv2) on any non-loopback interface or VRF`),
		title: "an IGP module is listed on %s but runs on none of its links, so netlab removed it",
		why:   "A node whose links do not enable the IGP has no IGP. On a node that also runs iBGP or EVPN this is the same lab-cannot-work case as a missing IGP.",
		fix:   "Give the node a link, or check the link/VLAN/VRF is in the IGP's scope (ospf.area, isis.area), or take the IGP out of the node's module list if it is not meant to run one."},
	{code: "module-attribute-on-unused-link", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Module attribute\(s\) \S+ are used on link (?P<s>\S+) but no attached nodes use`),
		title: "module attributes on link %s that none of its nodes run",
		why:   "The attributes have no effect: none of the nodes on the link has the module enabled.",
		fix:   "Add the module to the module list of the nodes on that link, or remove the attributes."},
	{code: "attribute-ignored-on-vlan-trunk", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Do not use attribute\(s\) \S+ on VLAN trunk interface \((?P<s>[^)]*)\)`),
		title: "attributes set on a VLAN trunk interface (%s) are ignored",
		why:   "Per-interface attributes on a trunk are not applied; they belong on the VLAN.",
		fix:   "Move the attributes to the VLAN definition or to a routed interface."},
	{code: "custom-config-missing", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Custom configuration \S+ is missing for device \S+ \(nodes (?P<s>[^)]*)\)`),
		title: "custom configuration is missing for the device type of %s",
		why:   "The custom configuration template does not exist for this device, so that part of the lab is not configured.",
		fix:   "Provide the template for the device or remove the custom configuration."},
	{code: "attribute-ignored", class: ClassLikelyWrong, warning: true,
		re:    rx(`(?:is not a dictionary, ignoring|cost is ignored|attribute ignored|not rendered|will likely not work)`),
		title: "",
		why:   "netlab ignored a value that was written, so the lab will not match the topology.",
		fix:   "Fix the value so netlab can use it, or remove it."},
	{code: "ipsec-or-role-unsupported", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Node (?P<s>\S+)\([^)]*\) (?:cannot have role|has load-balancing)`),
		title: "",
		why:   "The device does not support what was requested for this node.",
		fix:   "Choose a role/device combination netlab lists as supported."},
	{code: "evpn-segment-mismatch", class: ClassLikelyWrong, warning: true,
		re:    rx(`^EVPN (?:Ethernet Segment|ESI-LAG) "(?P<s>[^"]+)"`),
		title: "",
		why:   "An EVPN Ethernet Segment is used on one node only or its LACP settings differ between the nodes, so multihoming will not behave as drawn.",
		fix:   "Use the segment on both multihomed nodes with identical LACP settings."},
	{code: "ebgp-over-evpn-vlan-removed", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Removing an EBGP session between (?P<s>\S+) and`),
		title: "",
		why:   "An EBGP session over an EVPN-enabled VLAN was removed.",
		fix:   "Run the EBGP session over a routed link instead."},

	{code: "evpn-svi-ospf-overlay", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Node (?P<s>\S+) runs OSPF on the IRB SVI of EVPN-controlled VLAN`),
		title: "OSPF runs on the IRB SVI of an EVPN-controlled VLAN on %s",
		why:   "netlab enables every routing protocol on the SVI of a VXLAN-extended VLAN, so the leaves form OSPF adjacencies across their own VXLAN tunnel and learn each other's VTEP loopback through it. The route's next hop is reachable only through that tunnel (recursive routing): the tunnel black-holes, the adjacency drops, the route falls back to the underlay, the adjacency re-forms. Hosts lose traffic every 30 to 40 seconds (measured: 77 of 128 probes at 100 percent loss; 0 of 264 with the fix). netlab compiles it without complaint and documents it only under 'Interactions with Routing Protocols' in the EVPN module page.",
		fix:   "Keep OSPF off the VLAN: set vlans.<name>.ospf: false on every EVPN-controlled VLAN (the SVI stays; same-VLAN traffic through the leaves ran with 0 loss in the live proof). Alternatives: vlans.<name>.ospf.passive: true, or vlans.<name>.mode: bridge when the fabric needs no gateway. A bridge-mode VLAN has no SVI, so hosts on it need a gateway elsewhere. If the adjacency across the VLAN is wanted, say so with defaults.evpn.warnings.svi_ospf: False."},

	{code: "zonebased-rules-not-rendered", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Node (?P<s>\S+)\([^)]*\) sets firewall\.zonebased\.rules, which \S+ does not render`),
		title: "firewall.zonebased.rules on %s is accepted but not rendered on that device",
		why:   "An attribute can be legal on a device and rendered by only some of the devices its plugin runs on. netlab accepts firewall.zonebased.rules on this device, compiles with no error and puts nothing in the device config, so the rule you wrote does not exist on the box and the traffic is denied or falls to the default rule. Nothing at boot or in data collection reports it.",
		fix:   "Use the module-level attribute that the device does render for specific rules (firewall.policy.rules, plugin firewall.policy; its rules land above the zone-based default rules), keeping firewall.zonebased for zone membership (firewall.zone) and default_rules. The warning names the devices that do render the attribute."},
	{code: "fabric-near-miss-node-name", class: ClassLikelyWrong, warning: true,
		re:    rx(`^Node (?P<s>\S+) looks like the fabric node \S+, but it is a separate node outside the fabric groups`),
		title: "%s looks like a generated fabric node but is a separate node outside the fabric groups",
		why:   "The fabric plugin generates its own nodes (named by fabric.leaf.name / fabric.spine.name). A hand-declared node written as a role word and a number is a different node: the fabric groups' modules, VLANs and parameters never reach it, and it sits beside the generated twin.",
		fix:   "Use the generated name (nodes: { L1: ... } adds parameters to the generated node), or change fabric.leaf.name / fabric.spine.name to the template you want."},

	// ---- informational (housekeeping, not shown) ----
	{code: "module-pruned-unused", class: ClassInformational, warning: true,
		re:  rx(`^Node (?P<s>\S+) (?:uses no VRFs, removing|does not use any FHRP technology, removing)`),
		why: "netlab removed a module the node did not use."},
	{code: "deprecated-or-renamed", class: ClassInformational, warning: true,
		re:  rx(`(?:is obsolete|has been renamed to|has been replaced by|Initial wait time extended|Netmiko is not installed|obsolete attribute node_data)`),
		why: "A name changed or a default was adjusted; the lab is unaffected."},

	// ---- errors (the compile failed): the common ones get a cause and a fix ----
	{code: "group-module-no-members", class: ClassWillNotWork,
		re:    rx(`^Cannot use "module" attribute in group (?P<s>\S+) that has no direct or indirect members`),
		title: "groups with a module list but no members: %s",
		why:   "A group's module list only applies to its members. Members come from the group's own members list, from nodes that name the group, or from a plugin that generates nodes (the plugin must be listed under plugin:). The usual cause is a plugin block (for example fabric:) written without enabling the plugin, or node names that do not match the group.",
		fix:   "Enable the plugin that generates the group, or list the members, or put group: <name> on the nodes. Remove the group if nothing is meant to be in it."},
	{code: "unknown-topology-attribute", class: ClassWillNotWork,
		re:    rx(`^Invalid topology attribute '(?P<s>[^']+)'`),
		title: "top-level keys netlab does not know: %s",
		why:   "A top-level key that netlab does not know. If it is a plugin's configuration block, the plugin is not enabled and nothing reads the block. Other likely causes: a section netlab does not have (for example a hosts: section; hosts are nodes with a device such as linux) or a misspelled key.",
		fix:   "Add the plugin under plugin: [ ... ] if the key belongs to one; otherwise move the data under nodes:, links:, groups: or defaults:, or correct the spelling."},
	{code: "group-null-value", class: ClassWillNotWork,
		re:    rx(`^Definition of (?:\S+ )?group '(?P<s>[^']+)' must be a dictionary`),
		title: "groups with no value: %s",
		why:   "A group written as `name:` with nothing after it is YAML null, which netlab does not accept as a group definition (a plugin that fills the group crashed on it before netlab reported this).",
		fix:   "Give the group attributes (module, device, members), a members list, or write `name: {}` for an empty group."},
	{code: "missing-device-type", class: ClassWillNotWork,
		re:    rx(`^No device type specified for node (?P<s>\S+) and there is no default device type`),
		title: "no device type for %s",
		why:   "netlab will not guess a device.",
		fix:   "Set defaults.device, or device: on the node or on a group."},
	{code: "link-unknown-node", class: ClassWillNotWork,
		re:    rx(`^Link string \S+ in \S+ refers to an unknown node (?P<s>\S+)`),
		title: "links refer to nodes that are not defined: %s",
		why:   "A link names a node that is not in nodes: (or generated by a plugin).",
		fix:   "Define the node under nodes: with its device, or correct the node name in the link."},
	{code: "prefix-too-small", class: ClassWillNotWork,
		re:    rx(`^Cannot (?:use \S+ prefix \S+ to address|generate gateway IP address on) (?P<s>\S+)`),
		title: "",
		why:   "The prefix has too few addresses for the nodes (and the first-hop gateway) on the link.",
		fix:   "Use a shorter prefix length for that link or pool (addressing.<pool>.prefix), or remove the gateway."},
	{code: "device-module-unsupported", class: ClassWillNotWork,
		re:    rx(`^Device type (?P<s>\S+) \(node \S+\) does not support module|^attribute nodes\.(?P<s2>[^.]+)\.module has invalid value`),
		title: "",
		why:   "The module is not available for that device type (or provider).",
		fix:   "Remove the module from the node's list, or choose a device that supports it."},
	{code: "module-not-enabled", class: ClassWillNotWork,
		re:    rx(`uses an attribute from module (?P<s>\S+) which is not enabled`),
		title: "",
		why:   "An attribute of a module is used on an object whose node does not have the module enabled.",
		fix:   "Add the module to the module list of the node (or group) that uses it. A node's own module: list replaces the topology-level list, so name the full set there."},
	{code: "mandatory-attribute-missing", class: ClassWillNotWork,
		re:    rx(`^Mandatory attribute '(?P<s>[^']+)' is missing`),
		title: "",
		why:   "A module needs this value on every node that runs it.",
		fix:   "Set the attribute on the node, its group or the topology (for BGP: bgp.as). If the node should not run that module at all, take it out of the topology-level module list and name it only on the groups that run it: Skyforge adds the topology-level list to every router."},
	{code: "missing-dependency", class: ClassWillNotWork, cat: "MissingDependency",
		re:    rx(`.`),
		title: "",
		why:   "A module or plugin needs something that is not enabled.",
		fix:   "Enable what netlab names, on the same nodes that use this module."},
}

var (
	reFindingHeader = regexp.MustCompile(`^(Warning|[A-Z][A-Za-z]+) in ([A-Za-z0-9_.\-]+): (.*)$`)
	reHint          = regexp.MustCompile(`^\.\.\. ?(.*)$`)
)

type rawFinding struct {
	warning bool
	cat     string
	module  string
	text    string
	hints   []string
}

// ParseNetlabOutput pulls netlab's "Warning in <module>: ..." and "<Category> in <module>: ..." entries, with their
// "... hint" continuation lines, out of a job log. Lines may carry the kubelet's timestamp prefix.
func ParseNetlabOutput(lines []string) []rawFinding {
	var out []rawFinding
	for _, line := range lines {
		l := StripLogPrefix(line)
		if m := reFindingHeader.FindStringSubmatch(l); m != nil {
			if m[1] == "Fatal" || m[1] == "Errors" {
				continue
			}
			out = append(out, rawFinding{warning: m[1] == "Warning", cat: m[1], module: m[2], text: m[3]})
			continue
		}
		if len(out) > 0 {
			if m := reHint.FindStringSubmatch(l); m != nil {
				out[len(out)-1].hints = append(out[len(out)-1].hints, m[1])
				continue
			}
		}
	}
	return out
}

// ClassifyNetlabOutput classifies everything netlab printed. Findings with the same code are merged (one finding per
// code, with all the nodes/groups it names), will-not-work first. includeInformational keeps the housekeeping class,
// which the diagnostics omit and telemetry counts.
func ClassifyNetlabOutput(lines []string, includeInformational bool) []Finding {
	type acc struct {
		f        Finding
		hints    string
		rule     *findingRule
		subjects map[string]bool
	}
	byKey := map[string]*acc{}
	var order []string
	for _, raw := range ParseNetlabOutput(lines) {
		rule := matchFindingRule(raw)
		var f Finding
		subject := ""
		key := ""
		switch {
		case rule != nil:
			f = Finding{Code: rule.code, Class: rule.class, IsError: !raw.warning, Module: raw.module, Why: rule.why, Fix: rule.fix, Source: SourceNetlab}
			subject = ruleSubject(rule, raw.text)
			key = rule.code
		case raw.warning:
			f = Finding{Code: "netlab-warning", Class: ClassLikelyWrong, Module: raw.module,
				Why: "netlab warned that it changed or skipped something.", Fix: "Read netlab's hint and fix or remove what it names.", Source: SourceNetlab}
			key = "netlab-warning|" + raw.text
		default:
			// An error no rule knows is still the cause of the failure: keep netlab's own words.
			f = Finding{Code: "netlab-error", Class: ClassWillNotWork, IsError: true, Module: raw.module,
				Why: "netlab refused the topology.", Fix: "Fix the attribute or value netlab names and validate again.", Source: SourceNetlab}
			key = "netlab-error|" + raw.text
		}
		if f.Class == ClassInformational && !includeInformational {
			continue
		}
		if a, ok := byKey[key]; ok {
			if subject != "" {
				a.subjects[subject] = true
			}
			continue
		}
		f.Message = raw.text
		if len(raw.hints) > 0 && (f.IsError || rule == nil) {
			f.Message += " -- " + strings.Join(raw.hints, "; ")
		}
		a := &acc{f: f, rule: rule, subjects: map[string]bool{}}
		if f.IsError && len(raw.hints) > 0 {
			a.hints = strings.Join(raw.hints, "; ")
		}
		if subject != "" {
			a.subjects[subject] = true
		}
		byKey[key] = a
		order = append(order, key)
	}
	out := make([]Finding, 0, len(order))
	for _, key := range order {
		a := byKey[key]
		f := a.f
		for s := range a.subjects {
			f.Subject = append(f.Subject, s)
		}
		sort.Strings(f.Subject)
		if a.rule != nil && a.rule.title != "" && len(f.Subject) > 0 {
			f.Message = fmt.Sprintf(a.rule.title, strings.Join(f.Subject, ", "))
			if a.hints != "" {
				f.Message += " -- " + a.hints
			}
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool { return classRank(out[i].Class) < classRank(out[j].Class) })
	return out
}

func classRank(c string) int {
	switch c {
	case ClassWillNotWork:
		return 0
	case ClassLikelyWrong:
		return 1
	}
	return 2
}

func matchFindingRule(raw rawFinding) *findingRule {
	for i := range findingRules {
		r := &findingRules[i]
		if r.warning != raw.warning || (r.cat != "" && r.cat != raw.cat) {
			continue
		}
		if r.re.MatchString(raw.text) {
			return r
		}
	}
	return nil
}

func ruleByCode(code string) *findingRule {
	for i := range findingRules {
		if findingRules[i].code == code {
			return &findingRules[i]
		}
	}
	return nil
}

func ruleSubject(r *findingRule, text string) string {
	m := r.re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	for i, name := range r.re.SubexpNames() {
		if (name == "s" || name == "s2") && i < len(m) && m[i] != "" {
			return m[i]
		}
	}
	return ""
}

// FindingCodes lists every code the table can produce, for telemetry label allow-lists and the coverage gate.
func FindingCodes() []string {
	seen := map[string]bool{"netlab-warning": true, "netlab-error": true}
	codes := []string{"netlab-error", "netlab-warning"}
	for _, r := range findingRules {
		if !seen[r.code] {
			seen[r.code] = true
			codes = append(codes, r.code)
		}
	}
	sort.Strings(codes)
	return codes
}

// FindingIsError reports whether a code is a netlab ERROR (the compile failed) rather than a compiled-with-warning finding.
func FindingIsError(code string) bool {
	if code == "netlab-error" {
		return true
	}
	if r := ruleByCode(code); r != nil {
		return !r.warning
	}
	return false
}

// FindingCodeSet is FindingCodes as a set, for bounded metric labels.
func FindingCodeSet() map[string]bool {
	set := map[string]bool{}
	for _, c := range FindingCodes() {
		set[c] = true
	}
	return set
}

// FindingClass returns the class for a code ("" when unknown).
func FindingClass(code string) string {
	if code == "netlab-warning" {
		return ClassLikelyWrong
	}
	if code == "netlab-error" {
		return ClassWillNotWork
	}
	if r := ruleByCode(code); r != nil {
		return r.class
	}
	return ""
}
