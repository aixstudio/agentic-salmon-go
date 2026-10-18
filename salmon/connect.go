package salmon

import (
	"fmt"
	"sort"
	"strings"
)

type ConnectAgent struct{}

func permits(s EntityKind, r RelationKind, o EntityKind) bool {
	return s == FoodPortion && ((r == LocatedIn && o == CookingEquipment) || (r == HasVisibleFeature && o == ObservableFeature))
}
func (ConnectAgent) Run(p Perception) (ConnectedEvent, error) {
	out := ConnectedEvent{Type: "food_preparation_assessment", Ontology: Ontology{"agentic-salmon-kitchen", "1.0.0"}, Entities: []Entity{}, Relationships: []Relationship{}}
	mentions := map[string]EntityMention{}
	canonical := map[string]string{}
	entities := map[string]*Entity{}
	counts := map[EntityKind]int{}
	if len(p.Entities) == 0 {
		return out, fmt.Errorf("Connect requires entity mentions")
	}
	for _, m := range p.Entities {
		if strings.TrimSpace(m.ID) == "" {
			return out, fmt.Errorf("empty mention ID")
		}
		if _, ok := mentions[m.ID]; ok {
			return out, fmt.Errorf("duplicate mention ID: %s", m.ID)
		}
		if m.Kind != FoodPortion && m.Kind != CookingEquipment && m.Kind != ObservableFeature {
			return out, fmt.Errorf("unknown entity kind: %s", m.Kind)
		}
		mentions[m.ID] = m
		id := "food_portions_1"
		if m.Kind != FoodPortion {
			counts[m.Kind]++
			id = fmt.Sprintf("%s_%d", m.Kind, counts[m.Kind])
		}
		canonical[m.ID] = id
		if e, ok := entities[id]; ok {
			if !contains(e.Claims, m.Claim) {
				e.Claims = append(e.Claims, m.Claim)
			}
		} else {
			entities[id] = &Entity{ID: id, Kind: m.Kind, Label: m.Label, Claims: []string{m.Claim}}
		}
	}
	for _, r := range p.Relationships {
		s, okS := mentions[r.Subject]
		o, okO := mentions[r.Object]
		if !okS || !okO || !permits(s.Kind, r.Kind, o.Kind) {
			return out, fmt.Errorf("ontology rejects relationship %s %s %s", r.Subject, r.Kind, r.Object)
		}
		out.Relationships = append(out.Relationships, Relationship{canonical[r.Subject], r.Kind, canonical[r.Object], r.Claim})
	}
	for _, e := range entities {
		sort.Strings(e.Claims)
		out.Entities = append(out.Entities, *e)
	}
	sort.Slice(out.Entities, func(i, j int) bool { return out.Entities[i].ID < out.Entities[j].ID })
	sort.Slice(out.Relationships, func(i, j int) bool {
		a, b := out.Relationships[i], out.Relationships[j]
		return a.Subject+string(a.Kind)+a.Object < b.Subject+string(b.Kind)+b.Object
	})
	suffix := "unbound"
	if len(p.ImageSHA256) >= 12 {
		suffix = p.ImageSHA256[:12]
	}
	out.ID = "food-preparation-assessment:" + suffix
	return out, nil
}
func contains(items []string, s string) bool {
	for _, item := range items {
		if item == s {
			return true
		}
	}
	return false
}
