package intentrules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/forwardnetworks/skyforge/intentrules/internal/netlabref"
)

// Rule is one declarative check.
type Rule struct {
	ID          string
	Code        string
	Class       string // only for a code netlab's classifier does not own
	Kind        string
	Description string
	Needs       []string
	Where       *Pred
	Assert      *Pred
	AssertNot   *Pred
	Message     string
	Subjects    *Subjects
}

// Subjects names the related objects a finding is about.
type Subjects struct {
	Rel   string
	Where *Pred
}

type ruleFile struct {
	Version int `yaml:"rules_version"`
	Rules   []struct {
		ID          string   `yaml:"id"`
		Code        string   `yaml:"code"`
		Class       string   `yaml:"class"`
		Kind        string   `yaml:"kind"`
		Description string   `yaml:"description"`
		Needs       []string `yaml:"needs"`
		Where       *Pred    `yaml:"where"`
		Assert      *Pred    `yaml:"assert"`
		AssertNot   *Pred    `yaml:"assert_not"`
		Message     string   `yaml:"message"`
		Subjects    *struct {
			Rel   string `yaml:"rel"`
			Where *Pred  `yaml:"where"`
		} `yaml:"subjects"`
	} `yaml:"rules"`
}

// LoadRules parses a rules file and checks every kind, attribute, relationship and code it names against the schema, the sidecar
// and netlab's classifier.
func LoadRules(name string, data []byte, s *Schema, sc *Sidecar) ([]*Rule, error) {
	var raw ruleFile
	if err := strictDecode(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if raw.Version != 1 {
		return nil, fmt.Errorf("%s: rules_version must be 1", name)
	}
	seen := map[string]bool{}
	var out []*Rule
	for _, r := range raw.Rules {
		if r.ID == "" || r.Code == "" || r.Kind == "" || r.Message == "" {
			return nil, fmt.Errorf("%s: a rule needs id, code, kind and message (%q)", name, r.ID)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("%s: rule id %s is used twice", name, r.ID)
		}
		seen[r.ID] = true
		if (r.Assert == nil) == (r.AssertNot == nil) {
			return nil, fmt.Errorf("%s: rule %s needs exactly one of assert and assert_not", name, r.ID)
		}
		if _, _, _, owned := netlabref.RuleText(r.Code); !owned {
			if _, ok := sc.Findings[r.Code]; !ok {
				return nil, fmt.Errorf("%s: rule %s: code %s is neither in netlab's classifier nor in the sidecar findings, so it has no why or fix", name, r.ID, r.Code)
			}
		} else if r.Class != "" {
			return nil, fmt.Errorf("%s: rule %s: code %s is netlab's; its class comes from the classifier, do not restate it", name, r.ID, r.Code)
		}
		for _, p := range []*Pred{r.Where, r.Assert, r.AssertNot} {
			if p == nil {
				continue
			}
			if err := p.check(s, r.Kind); err != nil {
				return nil, fmt.Errorf("%s: rule %s: %w", name, r.ID, err)
			}
		}
		rule := &Rule{ID: r.ID, Code: r.Code, Class: r.Class, Kind: r.Kind, Description: strings.TrimSpace(r.Description), Needs: r.Needs,
			Where: r.Where, Assert: r.Assert, AssertNot: r.AssertNot, Message: r.Message}
		if r.Subjects != nil {
			k := s.Kinds[r.Kind]
			rel := k.Rel(r.Subjects.Rel)
			if rel == nil {
				return nil, fmt.Errorf("%s: rule %s: subjects names %q, which %s does not have", name, r.ID, r.Subjects.Rel, r.Kind)
			}
			if r.Subjects.Where != nil {
				if err := r.Subjects.Where.check(s, rel.Peer); err != nil {
					return nil, fmt.Errorf("%s: rule %s: subjects: %w", name, r.ID, err)
				}
			}
			rule.Subjects = &Subjects{Rel: r.Subjects.Rel, Where: r.Subjects.Where}
		}
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
