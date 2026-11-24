package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// Substitution is a card slot of an original list that a frontier table
// replaced by a basic land, and why.
type Substitution struct {
	Seat   int    `json:"seat"`
	Card   string `json:"card"`
	Count  int    `json:"count"`
	Basic  string `json:"basic"`
	Reason string `json:"reason"`
}

// frontier rewrites decks in place into the nearest lists the running kernel
// deals. First every card the set does not play gives its slots to the deck's
// most common basic land. Then, while the kernel still refuses the table over
// an interaction between cards, the first card the refusal names goes the
// same way. The order of the remaining cards never changes, so the result is
// a function of the original lists and the engine alone.
func frontier(decks []lab.Deck, allow []string) ([]Substitution, error) {
	set, err := allowed(allow)
	if err != nil {
		return nil, err
	}
	var subs []Substitution
	replace := func(seat int, card, reason string) error {
		d := &decks[seat]
		basic := commonBasic(d.Cards)
		if basic < 0 {
			return fmt.Errorf("%s: no basic land to stand in for %s", d.Name, card)
		}
		name := d.Cards[basic].Name
		kept := d.Cards[:0:0]
		n := 0
		for _, c := range d.Cards {
			if c.Name == card {
				n += c.Count
				continue
			}
			kept = append(kept, c)
		}
		for i := range kept {
			if kept[i].Name == name {
				kept[i].Count += n
			}
		}
		d.Cards = kept
		subs = append(subs, Substitution{Seat: seat, Card: card, Count: n, Basic: name, Reason: reason})
		return nil
	}
	for seat := range decks {
		var gaps [][2]string
		for _, c := range decks[seat].Cards {
			switch i, ok := set.Lookup(c.Name); {
			case !ok:
				gaps = append(gaps, [2]string{c.Name, "not in card set " + CardSet})
			case !set.Supported()[i]:
				gaps = append(gaps, [2]string{c.Name, set.Reason(i)})
			}
		}
		for _, g := range gaps {
			if err := replace(seat, g[0], g[1]); err != nil {
				return nil, err
			}
		}
	}
	for range 64 {
		su, err := lab.Open(labConfig(decks, 1, allow))
		if err == nil {
			su.Close()
			return subs, nil
		}
		var inter *cardset.InteractionRefusal
		if !errors.As(err, &inter) {
			return nil, err
		}
		seat, card := holder(decks, inter.Cards)
		if seat < 0 {
			return nil, fmt.Errorf("interaction refusal names no card of the table: %w", err)
		}
		if err := replace(seat, card, "interaction "+inter.Reason+" among "+strings.Join(inter.Cards, ", ")); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("frontier: the table still does not deal after 64 substitutions")
}

// holder is the first of cards some deck holds, and the first seat holding it.
func holder(decks []lab.Deck, cards []string) (int, string) {
	for _, card := range cards {
		for seat, d := range decks {
			for _, c := range d.Cards {
				if c.Name == card {
					return seat, card
				}
			}
		}
	}
	return -1, ""
}

// DeckJSON is deck d as a gorge deck file, with seat's substitutions in its
// notes. A frontier deck's hash is the hash of these bytes.
func DeckJSON(d lab.Deck, subs []Substitution, seat int) []byte {
	type entry struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	out := struct {
		Name      string  `json:"name"`
		Format    string  `json:"format"`
		Commander string  `json:"commander"`
		Notes     string  `json:"notes,omitempty"`
		Cards     []entry `json:"cards"`
	}{Name: d.Name, Format: "commander"}
	if len(d.Commanders) > 0 {
		out.Commander = d.Commanders[0]
	}
	var notes []string
	for _, s := range subs {
		if s.Seat == seat {
			notes = append(notes, fmt.Sprintf("%d %s -> %s", s.Count, s.Card, s.Basic))
		}
	}
	if len(notes) > 0 {
		out.Notes = "replaced by basic lands: " + strings.Join(notes, "; ")
	}
	for _, c := range d.Cards {
		out.Cards = append(out.Cards, entry{c.Name, c.Count})
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	return append(raw, '\n')
}
