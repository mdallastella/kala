// Package tags implements composite tag normalisation and namespace rules.
//
// A tag is a colon-separated path such as "project:kala:api". Its first
// segment is its namespace. Namespaces may be defined per user with a list of
// ordered values; an exclusive namespace allows at most one of its values per
// task, may have a default value, and marks some values as terminal (the task
// is considered closed while it carries one).
package tags

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Reserved names cannot be used as namespaces because the filter and quick-add
// languages use them as keywords.
var Reserved = map[string]bool{"due": true, "remind": true, "closed": true, "sort": true, "is": true}

// MaxLen is the maximum length of a tag name.
const MaxLen = 128

// Normalize trims and lowercases a tag and validates its syntax.
func Normalize(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#")))
	if s == "" {
		return "", fmt.Errorf("empty tag")
	}
	if len(s) > MaxLen {
		return "", fmt.Errorf("tag %q is too long", s)
	}
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`,"#@!`, r) {
			return "", fmt.Errorf("tag %q contains invalid character %q", s, r)
		}
	}
	for seg := range strings.SplitSeq(s, ":") {
		if seg == "" {
			return "", fmt.Errorf("tag %q has an empty segment", s)
		}
	}
	if ns, _ := Split(s); Reserved[ns] && strings.Contains(s, ":") {
		return "", fmt.Errorf("%q is a reserved keyword and cannot be a tag namespace", ns)
	}
	return s, nil
}

// NormalizeAll normalises a list of tags, dropping duplicates.
func NormalizeAll(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	for _, t := range in {
		n, err := Normalize(t)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out, nil
}

// Split returns the namespace (first segment) and the rest of a tag. For a tag
// without colon, the namespace is the whole tag and value is empty.
func Split(tag string) (ns, value string) {
	ns, value, _ = strings.Cut(tag, ":")
	return ns, value
}

// InSubtree reports whether tag equals root or descends from it.
func InSubtree(tag, root string) bool {
	return tag == root || strings.HasPrefix(tag, root+":")
}

// Value is one allowed value of a namespace.
type Value struct {
	Value    string `json:"value"`
	Terminal bool   `json:"terminal"`
}

// Namespace is a user-defined namespace definition. Values are ordered.
type Namespace struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Exclusive bool    `json:"exclusive"`
	Default   string  `json:"default_value,omitempty"`
	Values    []Value `json:"values"`
}

// Has reports whether v is a defined value.
func (n *Namespace) Has(v string) bool {
	return slices.ContainsFunc(n.Values, func(x Value) bool { return x.Value == v })
}

// Terminal reports whether v is a terminal value.
func (n *Namespace) Terminal(v string) bool {
	return slices.ContainsFunc(n.Values, func(x Value) bool { return x.Value == v && x.Terminal })
}

// FirstTerminal returns the first terminal value.
func (n *Namespace) FirstTerminal() (string, bool) {
	for _, v := range n.Values {
		if v.Terminal {
			return v.Value, true
		}
	}
	return "", false
}

// Validate checks a namespace definition.
func (n *Namespace) Validate() error {
	if strings.Contains(n.Name, ":") {
		return fmt.Errorf("namespace name cannot contain ':'")
	}
	if _, err := Normalize(n.Name); err != nil {
		return err
	}
	if Reserved[n.Name] {
		return fmt.Errorf("%q is a reserved keyword", n.Name)
	}
	seen := map[string]bool{}
	for i, v := range n.Values {
		nv, err := Normalize(v.Value)
		if err != nil {
			return err
		}
		if strings.Contains(nv, ":") {
			return fmt.Errorf("namespace value %q cannot contain ':'", v.Value)
		}
		if seen[nv] {
			return fmt.Errorf("duplicate value %q", nv)
		}
		seen[nv] = true
		n.Values[i].Value = nv
	}
	if n.Default != "" && !n.Has(n.Default) {
		return fmt.Errorf("default %q is not one of the values", n.Default)
	}
	if n.Default != "" && !n.Exclusive {
		return fmt.Errorf("only exclusive namespaces can have a default")
	}
	if n.Exclusive && len(n.Values) == 0 {
		return fmt.Errorf("an exclusive namespace needs at least one value")
	}
	return nil
}

// Rules is the set of namespace definitions of one user, keyed by name.
type Rules map[string]*Namespace

// Apply returns the tag set obtained by removing and then adding tags to
// current, enforcing exclusivity and defaults. All inputs must be normalised.
func (r Rules) Apply(current, add, remove []string) ([]string, error) {
	set := slices.Clone(current)
	set = slices.DeleteFunc(set, func(t string) bool { return slices.Contains(remove, t) })
	addedNS := map[string]string{}
	for _, t := range add {
		ns, v := Split(t)
		if def, ok := r[ns]; ok && def.Exclusive && v != "" {
			if !def.Has(v) {
				return nil, fmt.Errorf("%q is not a value of namespace %q (allowed: %s)", v, ns, def.valueList())
			}
			if prev, dup := addedNS[ns]; dup && prev != t {
				return nil, fmt.Errorf("namespace %q is exclusive: cannot add both %q and %q", ns, prev, t)
			}
			addedNS[ns] = t
			set = slices.DeleteFunc(set, func(x string) bool { return strings.HasPrefix(x, ns+":") })
		}
		if !slices.Contains(set, t) {
			set = append(set, t)
		}
	}
	set = r.withDefaults(set)
	slices.Sort(set)
	return set, nil
}

// Validate checks that a complete tag set respects exclusivity.
func (r Rules) Validate(set []string) error {
	count := map[string]int{}
	for _, t := range set {
		ns, v := Split(t)
		def, ok := r[ns]
		if !ok || !def.Exclusive {
			continue
		}
		if v == "" || !def.Has(v) {
			return fmt.Errorf("%q is not a value of namespace %q", t, ns)
		}
		count[ns]++
		if count[ns] > 1 {
			return fmt.Errorf("namespace %q is exclusive but the task would have several values", ns)
		}
	}
	return nil
}

func (r Rules) withDefaults(set []string) []string {
	for name, def := range r {
		if !def.Exclusive || def.Default == "" {
			continue
		}
		if !slices.ContainsFunc(set, func(t string) bool { return strings.HasPrefix(t, name+":") }) {
			set = append(set, name+":"+def.Default)
		}
	}
	return set
}

// Defaults returns the default tags of a new task.
func (r Rules) Defaults() []string {
	set := r.withDefaults(nil)
	slices.Sort(set)
	return set
}

// Terminal reports whether set contains a terminal value.
func (r Rules) Terminal(set []string) bool {
	for _, t := range set {
		ns, v := Split(t)
		if def, ok := r[ns]; ok && def.Terminal(v) {
			return true
		}
	}
	return false
}

func (n *Namespace) valueList() string {
	vs := make([]string, len(n.Values))
	for i, v := range n.Values {
		vs[i] = v.Value
	}
	return strings.Join(vs, ", ")
}

// Rename maps tag to its new name when renaming the subtree from → to.
func Rename(tag, from, to string) (string, bool) {
	if !InSubtree(tag, from) {
		return tag, false
	}
	return to + strings.TrimPrefix(tag, from), true
}

// Seed returns the namespaces created for every new user.
func Seed() []Namespace {
	return []Namespace{
		{Name: "status", Exclusive: true, Default: "open", Values: []Value{{Value: "open"}, {Value: "waiting"}, {Value: "closed", Terminal: true}}},
		{Name: "priority", Exclusive: true, Values: []Value{{Value: "high"}, {Value: "medium"}, {Value: "low"}}},
		{Name: "project"},
	}
}
