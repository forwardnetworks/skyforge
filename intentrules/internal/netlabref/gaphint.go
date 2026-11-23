package netlabref

// gaphint.go — classify a netlab compile failure and emit the right signal.
//
// Every class below needs a DIFFERENT response, and the cost of confusing two of
// them is asymmetric. A typo filed as a netlab modelling gap puts an issue on a
// colleague's queue for a misspelling; a real gap treated as a typo gets worked
// around and copied into the next lab. So the server computes the class from
// evidence and the caller acts on it -- nobody is asked to guess whether
// netlab models something.
//
// MEASURED, NOT ASSUMED. Each pattern below is netlab's own text, captured from
// the real compiler on 2026-09-03:
//
//	IncorrectAttr in links: Invalid link attribute 'eigpr' found in links[1]
//	MissingValue in links: Interface data links[2].interfaces[2] is missing a "node" attribute
//	IncorrectAttr in links: links[2].r1 uses an attribute from module eigrp which is not enabled in nodes.r1
//
// The first is a typo. An earlier version of this file classified it
// not-modelled and instructed filing a gap issue against netlab -- for a
// misspelling. That is the exact failure this file exists to prevent, and it was
// caught by running the real compiler rather than reasoning about it.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabcaps"
)

// The FILING decision. Deliberately three values: the rule stays "file
// only on not-modelled", and richer detail travels in Class and Reason.
const (
	GapHintAuthorError = "author-error"
	GapHintNotModelled = "not-modelled"
	GapHintUnknown     = "unknown"
)

// The CLASS. What actually went wrong, so the caller can say something useful
// instead of "netlab rejected it".
const (
	ClassTypo        = "typo"         // a near-miss of a real name
	ClassNotEnabled  = "not-enabled"  // real module, absent from the node's module: list
	ClassWrongShape  = "wrong-shape"  // right idea, wrong YAML form
	ClassUnmodelled  = "unmodelled"   // netlab genuinely has no such thing
	ClassUnresolved  = "unresolved"   // could not tell
	ClassPluginCrash = "plugin-crash" // a plugin's Python crashed on the values it was given
)

type Hint struct {
	Hint       string // the filing decision
	Class      string // what went wrong
	Reason     string // human-readable, names the fix
	Suggestion string // the corrected name, when there is one
}

var (
	reModuleNamed  = regexp.MustCompile(`attribute from module ([a-z0-9_.]+)`)
	reInvalidAttr  = regexp.MustCompile(`Invalid \w+ attribute '([A-Za-z0-9_.]+)'`)
	reAttrQuoted   = regexp.MustCompile(`attribute '([A-Za-z0-9_.]+)'`)
	reUnknownNode  = regexp.MustCompile(`unknown node ([A-Za-z0-9_.-]+)`)
	reMissingNode  = regexp.MustCompile(`is missing a "node" attribute`)
	reNotEnabledIn = regexp.MustCompile(`not enabled in (\S+)`)
)

// editDistance is Levenshtein, bounded by max. NearestNames in netlabcaps is
// substring-based and cannot see a TRANSPOSITION: "eigpr" neither contains nor
// is contained by "eigrp", so it returns nothing for the most common typo shape
// there is. This closes that hole rather than replacing it.
func editDistance(a, b string, max int) int {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return 0
	}
	if abs(len(a)-len(b)) > max {
		return max + 1
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// nearest returns the closest known names to want, by edit distance then by the
// substring matching netlabcaps already does. Two is the ceiling: beyond that a
// "did you mean" is noise, and suggesting a wrong name is worse than none.
func nearest(want string, candidates []string) []string {
	type sc struct {
		name string
		d    int
	}
	var out []sc
	for _, c := range candidates {
		if d := editDistance(want, c, 2); d <= 2 {
			out = append(out, sc{c, d})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].d != out[j].d {
			return out[i].d < out[j].d
		}
		return out[i].name < out[j].name
	})
	names := make([]string, 0, len(out))
	for _, s := range out {
		names = append(names, s.name)
	}
	// NO SUBSTRING FALLBACK HERE. netlabcaps.NearestNames also matches plain
	// containment, which is right for "show me related names" and WRONG for "did
	// you mean": "quantumrouting" contains "routing" and is eight characters
	// longer, so a containment match would call a genuinely unmodelled module a
	// misspelling of a real one and suppress a gap that should be filed. Caught
	// by the test, not by inspection.
	if len(names) > 3 {
		names = names[:3]
	}
	return names
}

// related returns names worth mentioning WITHOUT claiming a misspelling. Used
// only in the unmodelled message, to give the reader somewhere to look.
func related(want string, candidates []string) []string {
	n := netlabcaps.NearestNames(want, candidates)
	if len(n) > 3 {
		n = n[:3]
	}
	return n
}

// Classify turns netlab's error text into a class, a filing decision and a fix.
func Classify(netlabErrors string) Hint {
	text := strings.TrimSpace(netlabErrors)
	if text == "" {
		return Hint{GapHintUnknown, ClassUnresolved, "no netlab error text to classify", ""}
	}
	// An empty manifest knows nothing, and "absent from an empty list" is not
	// evidence of absence. Filing on that is the false-gap failure itself.
	if !netlabcaps.Loaded() {
		return Hint{GapHintUnknown, ClassUnresolved,
			"the netlab capability manifest is not loaded, so nothing can be said about what netlab models", ""}
	}
	modules := netlabcaps.ModuleNames()

	// 0. A plugin's Python crashed. Neither a typo nor a proven gap: the plugin was
	//    handed values of a type it does not take (the common case) or has a bug
	//    (the rare one), and only the declared types separate the two. Filing on
	//    that guess is the false-gap failure; so the filing decision stays unknown
	//    and the reason says what to compare.
	if IsPluginCrashText(text) {
		return Hint{GapHintUnknown, ClassPluginCrash,
			"a netlab plugin crashed on the values it was given. Compare the values under its block with the attribute " +
				"types in the message (netlab_capabilities module=<plugin> lists them). Only if every value matches the " +
				"declared types is this a plugin defect, and then it needs a human to confirm before filing.", ""}
	}

	// 1. A module netlab NAMES is modelled by definition; it is merely not
	//    enabled on that node.
	if m := reModuleNamed.FindStringSubmatch(text); len(m) == 2 {
		mod := m[1]
		where := ""
		if w := reNotEnabledIn.FindStringSubmatch(text); len(w) == 2 {
			where = " on " + w[1]
		}
		if netlabcaps.HasModule(mod) {
			return Hint{GapHintAuthorError, ClassNotEnabled, fmt.Sprintf(
				"module %q exists in netlab but is not enabled%s. Add it to that node's `module:` list, "+
					"or to the topology-level `module:`.", mod, where), mod}
		}
		return classifyUnknownName(mod, modules, "module")
	}

	// 2. An invalid ATTRIBUTE name. This is where a typo lands, and where the
	//    previous version wrongly filed a gap.
	if m := reInvalidAttr.FindStringSubmatch(text); len(m) == 2 {
		return classifyUnknownName(m[1], modules, "attribute")
	}

	// 3. A module name in NODE position: the list-form-with-attributes mistake.
	//    Right intent, wrong YAML shape -- never a gap.
	if m := reUnknownNode.FindStringSubmatch(text); len(m) == 2 {
		name := m[1]
		if netlabcaps.HasModule(name) {
			return Hint{GapHintAuthorError, ClassWrongShape, fmt.Sprintf(
				"%q is a netlab MODULE, not a node. `%s: false` is a link/interface attribute and needs the "+
					"DICTIONARY link form; in the list form YAML makes it a sibling of the node name, so netlab "+
					"reads it as another node. See links.md via netlab_docs.", name, name), name}
		}
		if s := nearest(name, modules); len(s) > 0 {
			return Hint{GapHintAuthorError, ClassTypo, fmt.Sprintf(
				"the topology refers to a node %q that it does not define; did you mean the module %q, written "+
					"in the DICTIONARY link form?", name, s[0]), s[0]}
		}
		return Hint{GapHintAuthorError, ClassWrongShape, fmt.Sprintf(
			"the topology refers to a node %q that it does not define", name), ""}
	}

	// 4. A link whose interface list lost its node: the same shape mistake seen
	//    from the other side.
	if reMissingNode.MatchString(text) {
		return Hint{GapHintAuthorError, ClassWrongShape,
			"a link interface has no node. This is the list-form-with-attributes mistake: attributes require the " +
				"DICTIONARY link form, and in the list form YAML makes the attribute a sibling of the node name. " +
				"netlab's own hint line shows the malformed value. See links.md via netlab_docs.", ""}
	}

	// 5. Any other quoted attribute.
	if m := reAttrQuoted.FindStringSubmatch(text); len(m) == 2 {
		attr := m[1]
		root := attr
		if i := strings.Index(root, "."); i > 0 {
			root = root[:i]
		}
		if netlabcaps.HasModule(root) {
			return Hint{GapHintAuthorError, ClassNotEnabled, fmt.Sprintf(
				"module %q exists; check the scope and spelling of %q with netlab_capabilities module=%s",
				root, attr, root), root}
		}
		return classifyUnknownName(root, modules, "attribute")
	}

	return Hint{GapHintUnknown, ClassUnresolved,
		"netlab's error did not name a module or attribute this can resolve", ""}
}

// classifyUnknownName is the TYPO-vs-GAP decision, and the whole reason this
// file was rewritten. A name close to a real one is a misspelling and must never
// be filed as a netlab gap.
func classifyUnknownName(name string, modules []string, kind string) Hint {
	if s := nearest(name, modules); len(s) > 0 {
		return Hint{GapHintAuthorError, ClassTypo, fmt.Sprintf(
			"no netlab %s is called %q -- did you mean %q? (netlab modules: %s)",
			kind, name, s[0], strings.Join(s, ", ")), s[0]}
	}
	msg := fmt.Sprintf("netlab has no %s called %q, and nothing is a near-miss of it.", kind, name)
	if r := related(name, modules); len(r) > 0 {
		msg += fmt.Sprintf(" Names that merely share text with it (NOT suggested spellings): %s.",
			strings.Join(r, ", "))
	}
	msg += " Confirm with netlab_capabilities before filing."
	return Hint{GapHintNotModelled, ClassUnmodelled, msg, ""}
}

// GapHintFor is the compatibility shim for callers that only need the filing
// decision.
func GapHintFor(netlabErrors string) (string, string) {
	h := Classify(netlabErrors)
	return h.Hint, h.Reason
}
