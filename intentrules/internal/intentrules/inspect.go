package intentrules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabref"
	"gopkg.in/yaml.v3"
)

// Engine is the loaded model: schema, sidecar and rules.
type Engine struct {
	Schema  *Schema
	Sidecar *Sidecar
	Rules   []*Rule
}

var (
	defaultOnce   sync.Once
	defaultEngine *Engine
	defaultErr    error
)

// Default returns the engine built from the embedded files. The files are part of the binary, so an error here is a defect in
// the model (a test loads them), never a run-time condition.
func Default() (*Engine, error) {
	if err := injectedLoadError(); err != nil {
		return nil, err
	}
	defaultOnce.Do(func() { defaultEngine, defaultErr = loadEmbedded() })
	return defaultEngine, defaultErr
}

func loadEmbedded() (*Engine, error) {
	files, err := embeddedFiles("schema", ".yml")
	if err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("expected one schema file under schema/, found %d", len(files))
	}
	raw, err := embedded.ReadFile(files[0])
	if err != nil {
		return nil, err
	}
	s, err := LoadSchema(files[0], raw)
	if err != nil {
		return nil, err
	}
	scFiles, err := embeddedFiles("sidecar", ".skyforge.yml")
	if err != nil {
		return nil, err
	}
	merged := &Sidecar{Annotations: map[string]*Annotation{}, Findings: map[string]FindingText{}}
	for _, f := range scFiles {
		b, _ := embedded.ReadFile(f)
		sc, err := LoadSidecar(f, b, s)
		if err != nil {
			return nil, err
		}
		for k, v := range sc.Annotations {
			merged.Annotations[k] = v
		}
		for k, v := range sc.Findings {
			merged.Findings[k] = v
		}
	}
	ruleFiles, err := embeddedFiles("sidecar/rules", ".rules.yml")
	if err != nil {
		return nil, err
	}
	e := &Engine{Schema: s, Sidecar: merged}
	for _, f := range ruleFiles {
		b, _ := embedded.ReadFile(f)
		rs, err := LoadRules(f, b, s, merged)
		if err != nil {
			return nil, err
		}
		e.Rules = append(e.Rules, rs...)
	}
	return e, nil
}

// Options turn rules off. A key is a rule id, or a finding code (which turns off every rule and constraint that reports it).
type Options struct {
	Disabled map[string]bool
}

// Result is what the layer found and what it could not look at.
type Result struct {
	Findings []netlabref.Finding
	// Evaluated lists the rule ids and schema constraint groups that ran; Skipped says why something did not.
	Evaluated []string
	Skipped   map[string]string
}

// InspectYAML judges a topology document given as text. A document that does not parse yields no findings and a skip: parse
// errors are the policy preflight's and netlab's to report.
func (e *Engine) InspectYAML(text string, opts Options) Result {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil || len(doc.Content) == 0 {
		return Result{Skipped: map[string]string{"document": "the document did not parse as YAML"}}
	}
	return e.Inspect(doc.Content[0], opts)
}

// Inspect judges a parsed topology document.
func (e *Engine) Inspect(root *yaml.Node, opts Options) Result {
	maybePanicForTest()
	g := Load(e.Schema, root)
	res := Result{Skipped: map[string]string{}}
	for k, v := range g.Skipped {
		res.Skipped[k] = v
	}
	var found []netlabref.Finding
	// 1. the constraints the schema states
	res.Evaluated = append(res.Evaluated, "schema-constraints")
	for _, v := range e.Schema.Constraints(g) {
		if f, ok := e.violationFinding(g, v); ok && !opts.Disabled[f.Code] {
			found = append(found, f)
		}
	}
	// 2. the declarative rules
	for _, r := range e.Rules {
		if opts.Disabled[r.ID] || opts.Disabled[r.Code] {
			res.Skipped[r.ID] = "disabled"
			continue
		}
		res.Evaluated = append(res.Evaluated, r.ID)
		found = append(found, e.evalRule(g, r)...)
	}
	res.Findings = dedupe(found)
	sort.SliceStable(res.Findings, func(i, j int) bool {
		if ri, rj := classRank(res.Findings[i].Class), classRank(res.Findings[j].Class); ri != rj {
			return ri < rj
		}
		return res.Findings[i].Path < res.Findings[j].Path
	})
	sort.Strings(res.Evaluated)
	return res
}

func classRank(c string) int {
	switch c {
	case netlabref.ClassWillNotWork:
		return 0
	case netlabref.ClassLikelyWrong:
		return 1
	}
	return 2
}

func dedupe(in []netlabref.Finding) []netlabref.Finding {
	seen := map[string]bool{}
	var out []netlabref.Finding
	for _, f := range in {
		key := f.Code + "|" + f.Path + "|" + f.Message
		if !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}

// textFor returns class, isError, why and fix for a code: netlab's classifier owns the text of a code it knows, the sidecar the
// text of a code only this layer emits.
func (e *Engine) textFor(code, ruleClass string) (class string, isError bool, why, fix string, ok bool) {
	if _, w, f, owned := netlabref.RuleText(code); owned {
		return netlabref.FindingClass(code), netlabref.FindingIsError(code), w, f, true
	}
	if t, found := e.Sidecar.Findings[code]; found {
		class = t.Class
		if ruleClass != "" {
			class = ruleClass
		}
		return class, false, t.Why, t.Fix, true
	}
	return "", false, "", "", false
}

var templateRE = regexp.MustCompile(`\{([a-z_]+)\}`)

func fill(tmpl string, o *Object, extra map[string]string) string {
	return templateRE.ReplaceAllStringFunc(tmpl, func(m string) string {
		key := m[1 : len(m)-1]
		if v, ok := extra[key]; ok {
			return v
		}
		if v, ok := o.Attrs[key]; ok && v != nil {
			return str(v)
		}
		return m
	})
}

func (e *Engine) annotationFor(kind, member string) *Annotation {
	if a := e.Sidecar.annotation(kind, member); a != nil {
		return a
	}
	if k := e.Schema.Kinds[kind]; k != nil {
		for _, parent := range k.InheritFrom {
			if a := e.Sidecar.annotation(parent, member); a != nil {
				return a
			}
		}
	}
	return nil
}

func (e *Engine) violationFinding(g *Graph, v Violation) (netlabref.Finding, bool) {
	// A constraint is reported only where the sidecar names the code for it. The loader is a model of netlab, not netlab's own
	// validation: netlab checks value ranges and name patterns itself (and lets a topology extend them), so an unmapped violation
	// is the model disagreeing with netlab, never a finding.
	var code, msgTmpl string
	if a := e.annotationFor(v.Object.Kind, v.Member); a != nil {
		code = a.Findings[v.Constraint]
		msgTmpl = a.Messages[v.Constraint]
	}
	if code == "" {
		return netlabref.Finding{}, false
	}
	class, isErr, why, fix, ok := e.textFor(code, "")
	if !ok {
		return netlabref.Finding{}, false
	}
	msg := fmt.Sprintf("%s: %s", e.Schema.Kinds[v.Object.Kind].Label, v.Detail)
	if msgTmpl != "" {
		msg = fill(msgTmpl, v.Object, map[string]string{"count": fmt.Sprint(v.Count), "path": v.Object.Path})
	}
	subject := str(v.Object.Attrs["name"])
	if subject == "" {
		subject = str(v.Object.Attrs["node"])
	}
	if subject == "" {
		subject = str(v.Object.Attrs["key"])
	}
	return netlabref.Finding{Code: code, Class: class, IsError: isErr, Module: "intentrules", Subject: nonEmpty(subject), Message: msg, Why: why, Fix: fix,
		Path: v.Object.Path, Source: netlabref.SourceIntentRules}, true
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func (e *Engine) evalRule(g *Graph, r *Rule) []netlabref.Finding {
	var out []netlabref.Finding
	for _, o := range g.Objects {
		if !e.Schema.isA(o.Kind, r.Kind) {
			continue
		}
		if r.Where != nil && !r.Where.eval(e.Schema, g, o) {
			continue
		}
		fire := false
		if r.Assert != nil {
			fire = !r.Assert.eval(e.Schema, g, o)
		} else {
			fire = r.AssertNot.eval(e.Schema, g, o)
		}
		if !fire {
			continue
		}
		class, isErr, why, fix, ok := e.textFor(r.Code, r.Class)
		if !ok {
			continue
		}
		extra := map[string]string{}
		var subjects []string
		if r.Subjects != nil {
			peers := related(e.Schema, g, o, r.Subjects.Rel, r.Subjects.Where)
			for _, p := range peers {
				subjects = append(subjects, str(p.Attrs["name"]))
			}
			sort.Strings(subjects)
			extra["subjects"] = strings.Join(subjects, ", ")
		}
		if len(subjects) == 0 {
			subjects = nonEmpty(str(o.Attrs["name"]))
		}
		out = append(out, netlabref.Finding{Code: r.Code, Class: class, IsError: isErr, Module: "intentrules", Subject: subjects,
			Message: fill(r.Message, o, extra), Why: why, Fix: fix, Path: o.Path, Source: netlabref.SourceIntentRules})
	}
	return out
}

// ---- merging with a compile --------------------------------------------------------------------------------------------------

// textOwned lists netlab messages that a static code stands for but that netlab's classifier has no row for (they classify as
// netlab-error). Without this, a duplicate the compile already reported would be reported twice under two names.
//
// complete says netlab's check is exhaustive for the code: a compile that succeeded proves the defect is absent. It is false for
// node-name-duplicate, where netlab refuses a repeated key in a mapping but silently keeps the last of two same-named nodes in a
// node list, so a good compile proves nothing about it.
var textOwned = []struct {
	code     string
	re       *regexp.Regexp
	complete bool
}{
	{"node-name-duplicate", regexp.MustCompile(`^(?:[A-Za-z]+) in [A-Za-z0-9_.\-]+: Duplicate section in YAML file`), false},
	{"schema-unique", regexp.MustCompile(`^(?:[A-Za-z]+) in [A-Za-z0-9_.\-]+: Duplicate linkid`), true},
	{"module-requires", regexp.MustCompile(`Module \S+ on device \S+ \(node \S+\) requires \S+ module`), true},
	{"link-unknown-node", regexp.MustCompile(`Interface \S+ refers to an unknown node|Interface data \S+ refers to an unknown node`), true},
	{"link-endpoints", regexp.MustCompile(`(?:Point-to-point link \S+ needs exactly two nodes|No valid nodes on link|More than one node connected to a stub link|Loopback link \S+ can have a single node attached)`), true},
}

// NetlabOwnedCodes returns the finding codes the compile output already carries, whether netlab's classifier names them or this
// package maps them from netlab's text.
func NetlabOwnedCodes(netlabLines []string) map[string]bool {
	owned := map[string]bool{}
	for _, f := range netlabref.ClassifyNetlabOutput(netlabLines, true) {
		owned[f.Code] = true
	}
	for _, line := range netlabLines {
		l := netlabref.StripLogPrefix(line)
		for _, t := range textOwned {
			if t.re.MatchString(l) {
				owned[t.code] = true
			}
		}
	}
	return owned
}

// NetlabChecksCode reports whether netlab's own check for a code is exhaustive: its classifier has a row for the code, or this
// package maps netlab's text to it and says the check is complete. After a compile that succeeded, a finding for such a code that the
// compile did not make is not a defect.
func NetlabChecksCode(code string) bool {
	if _, _, _, ok := netlabref.RuleText(code); ok {
		return true
	}
	for _, t := range textOwned {
		if t.code == code && t.complete {
			return true
		}
	}
	return false
}

// MergeWithCompile drops every static finding whose code the compile already reported. netlab's compile sees the resolved model,
// this layer only the authored document: the compile wins, and what remains is what netlab did not say. When the compile succeeded
// (compileSucceeded) netlab ran its checks, so a code whose netlab check is exhaustive (NetlabChecksCode) that it did not report is also
// dropped: that finding is this layer's model of netlab being wrong. Codes netlab does not check exhaustively are kept.
func MergeWithCompile(static []netlabref.Finding, netlabLines []string, compileSucceeded bool) []netlabref.Finding {
	owned := NetlabOwnedCodes(netlabLines)
	var out []netlabref.Finding
	for _, f := range static {
		if owned[f.Code] || (compileSucceeded && NetlabChecksCode(f.Code)) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// ---- the designer's inline issues ---------------------------------------------------------------------------------------------

// DesignerIssue is a finding shaped for the designer's inline diagnostics (skyforge.NetlabTopologyIssue): a severity, a path into
// the document and one sentence.
type DesignerIssue struct {
	Severity string
	Path     string
	Message  string
}

// DesignerIssues judges a topology document for the designer's policy preflight. Every issue is a warning: this layer reads the
// authored document only, so it never blocks a save or a launch (the compile decides), and informational findings are not shown.
func (e *Engine) DesignerIssues(topologyYAML string) []DesignerIssue {
	res := e.InspectYAML(topologyYAML, Options{})
	var out []DesignerIssue
	for _, f := range res.Findings {
		if f.Class == netlabref.ClassInformational {
			continue
		}
		msg := fmt.Sprintf("Skyforge intent rule %s: %s.", f.Code, strings.TrimSuffix(f.Message, "."))
		if f.Fix != "" {
			msg += " " + f.Fix
		}
		out = append(out, DesignerIssue{Severity: "warning", Path: f.Path, Message: msg})
	}
	return out
}
