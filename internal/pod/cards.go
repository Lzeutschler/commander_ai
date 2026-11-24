package pod

import (
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// CardGap is a deck line the kernel does not play.
type CardGap struct {
	Card   string `json:"card"`
	Count  int    `json:"count"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// DeckGaps lists one seat's unplayable lines.
type DeckGaps struct {
	DeckInfo
	GapSlots int       `json:"gap_slots"`
	Gaps     []CardGap `json:"gaps"`
}

// CardReport says which cards of a table the kernel does not play and, with
// those swapped for basic lands, whether it would deal the rest.
type CardReport struct {
	Schema     int        `json:"schema"`
	Table      string     `json:"table"`
	Provenance Provenance `json:"provenance"`
	GapSlots   int        `json:"gap_slots"`
	Decks      []DeckGaps `json:"decks"`
	// Interaction is the kernel's first objection to the table once every gap
	// above is replaced by the deck's most common basic land. Others may hide
	// behind it. Nil when the reduced table deals.
	Interaction *Refusal `json:"interaction,omitempty"`
}

// Cards builds the report for t.
func Cards(root string, t Table) (CardReport, error) {
	decks, infos, err := LoadTable(root, t)
	if err != nil {
		return CardReport{}, err
	}
	prov, err := ReadProvenance(root)
	if err != nil {
		return CardReport{}, err
	}
	set, err := cardset.ByName(CardSet)
	if err != nil {
		return CardReport{}, err
	}
	rep := CardReport{Schema: Schema, Table: t.Name, Provenance: prov}
	reduced := make([]lab.Deck, len(decks))
	for seat, d := range decks {
		dg := DeckGaps{DeckInfo: infos[seat], Gaps: []CardGap{}}
		kept := make([]cardset.Entry, 0, len(d.Cards))
		for _, c := range d.Cards {
			i, ok := set.Lookup(c.Name)
			switch {
			case !ok:
				dg.Gaps = append(dg.Gaps, CardGap{c.Name, c.Count, "absent", "not in card set " + CardSet})
			case !set.Supported()[i]:
				dg.Gaps = append(dg.Gaps, CardGap{c.Name, c.Count, set.Status(i).String(), set.Reason(i)})
			default:
				kept = append(kept, c)
				continue
			}
			dg.GapSlots += c.Count
		}
		if basic := commonBasic(kept); basic >= 0 {
			kept[basic].Count += dg.GapSlots
		}
		reduced[seat] = lab.Deck{Name: d.Name, Cards: kept, Commanders: d.Commanders}
		rep.GapSlots += dg.GapSlots
		rep.Decks = append(rep.Decks, dg)
	}
	su, err := lab.Open(labConfig(reduced, 1))
	if err != nil {
		rep.Interaction = refusalOf(err)
	} else {
		su.Close()
	}
	return rep, nil
}

// commonBasic is the index of the basic land with the most copies, or -1.
func commonBasic(cards []cardset.Entry) int {
	best := -1
	for i, c := range cards {
		if cardset.IsSynthBasic(c.Name) && (best < 0 || c.Count > cards[best].Count) {
			best = i
		}
	}
	return best
}
