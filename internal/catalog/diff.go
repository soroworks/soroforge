package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// ChangeKind says how an item differs between two interfaces.
type ChangeKind string

const (
	// Added items exist only in the newer interface.
	Added ChangeKind = "added"
	// Removed items exist only in the older interface.
	Removed ChangeKind = "removed"
	// Changed items exist in both with a different signature.
	Changed ChangeKind = "changed"
)

// Change is one difference between two contract interfaces.
type Change struct {
	Kind ChangeKind `json:"kind"`

	// Item is what changed: "fn", "struct", "union", "enum", "error" or
	// "event".
	Item string `json:"item"`
	Name string `json:"name"`

	// Signature is the item in the newer interface, or in the older one for
	// a removal. Previous is the older signature of a changed item.
	Signature string `json:"signature"`
	Previous  string `json:"previous,omitempty"`

	// Breaking marks a change that can make an existing caller's invocation
	// fail: a removed or changed function, or a removed or changed type that
	// callers encode as arguments. Additions never break, and event changes
	// do not affect invocations.
	Breaking bool `json:"breaking"`
}

// Diff compares two interfaces and returns their differences: functions
// first, then types, then events, each in name order. It is a pure function,
// so the same two interfaces always produce the same changes.
func Diff(from, to Interface) []Change {
	return diffItems(signatures(from), signatures(to))
}

// HasBreaking reports whether any change is breaking.
func HasBreaking(changes []Change) bool {
	for _, c := range changes {
		if c.Breaking {
			return true
		}
	}
	return false
}

// itemOrder ranks item kinds in the order Diff reports them.
var itemOrder = map[string]int{"fn": 0, "struct": 1, "union": 2, "enum": 3, "error": 4, "event": 5}

// item is one named, signature-bearing element of an interface.
type item struct {
	kind      string
	name      string
	signature string
}

func (i item) key() string { return i.kind + " " + i.name }

// breaks reports whether removing or changing an item of this kind can fail
// an existing invocation.
func breaks(kind string) bool { return kind != "event" }

func diffItems(from, to []item) []Change {
	before := make(map[string]item, len(from))
	for _, it := range from {
		before[it.key()] = it
	}
	after := make(map[string]item, len(to))
	for _, it := range to {
		after[it.key()] = it
	}

	var changes []Change
	for k, old := range before {
		cur, ok := after[k]
		switch {
		case !ok:
			changes = append(changes, Change{
				Kind: Removed, Item: old.kind, Name: old.name,
				Signature: old.signature, Breaking: breaks(old.kind),
			})
		case cur.signature != old.signature:
			changes = append(changes, Change{
				Kind: Changed, Item: cur.kind, Name: cur.name,
				Signature: cur.signature, Previous: old.signature, Breaking: breaks(cur.kind),
			})
		}
	}
	for k, cur := range after {
		if _, ok := before[k]; !ok {
			changes = append(changes, Change{Kind: Added, Item: cur.kind, Name: cur.name, Signature: cur.signature})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if itemOrder[a.Item] != itemOrder[b.Item] {
			return itemOrder[a.Item] < itemOrder[b.Item]
		}
		return a.Name < b.Name
	})
	return changes
}

// signatures flattens an interface into its named items.
func signatures(in Interface) []item {
	var items []item
	for _, f := range in.Functions {
		items = append(items, item{"fn", f.Name, FunctionSignature(f)})
	}
	for _, s := range in.Types.Structs {
		items = append(items, item{"struct", s.Name, fmt.Sprintf("struct %s { %s }", s.Name, params(s.Fields))})
	}
	for _, u := range in.Types.Unions {
		cases := make([]string, len(u.Cases))
		for i, c := range u.Cases {
			cases[i] = c.Name
			if len(c.Values) > 0 {
				cases[i] += "(" + types(c.Values) + ")"
			}
		}
		items = append(items, item{"union", u.Name, fmt.Sprintf("union %s { %s }", u.Name, strings.Join(cases, ", "))})
	}
	for _, e := range in.Types.Enums {
		items = append(items, item{"enum", e.Name, fmt.Sprintf("enum %s { %s }", e.Name, enumCases(e.Cases))})
	}
	for _, e := range in.Types.ErrorEnums {
		items = append(items, item{"error", e.Name, fmt.Sprintf("error %s { %s }", e.Name, enumCases(e.Cases))})
	}
	for _, ev := range in.Events {
		items = append(items, item{"event", ev.Name, fmt.Sprintf("event %s(%s)", ev.Name, params(ev.Params))})
	}
	return items
}

// FunctionSignature renders a function the way SoroVault does:
// fn name(a: T, b: U) -> R.
func FunctionSignature(f Function) string {
	sig := fmt.Sprintf("fn %s(%s)", f.Name, params(f.Inputs))
	if len(f.Outputs) > 0 {
		sig += " -> " + f.Outputs[0].Display
	}
	return sig
}

func params(ps []Param) string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name + ": " + p.Type.Display
	}
	return strings.Join(out, ", ")
}

func types(ts []Type) string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Display
	}
	return strings.Join(out, ", ")
}

func enumCases(cs []EnumCase) string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = fmt.Sprintf("%s = %d", c.Name, c.Value)
	}
	return strings.Join(out, ", ")
}
