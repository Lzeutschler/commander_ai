// Package pod runs four-seat Commander tables on the dirac kernel from frozen
// deck lists and records what was played, on which revisions, and how it ended.
package pod

import (
	"fmt"
	"sort"
)

// CardSet is the dirac card pool every table is dealt from.
const CardSet = "cmd5"

// Table is a named, fixed seating of deck files (paths relative to the repo root).
type Table struct {
	Name  string
	About string
	Seats []string
	// Frontier derives the played lists from Seats instead of playing them as
	// they are: every card slot the running kernel cannot play is replaced by a
	// basic land (frontier.go). The other tables are frozen.
	Frontier bool
}

var tables = map[string]Table{
	"original": {
		Name:  "original",
		About: "the four unmodified Foundations Commander precons",
		Seats: []string{
			"decks/original/foundations-calling-all-angels.json",
			"decks/original/foundations-keen-engineering.json",
			"decks/original/foundations-reign-of-dragons.json",
			"decks/original/foundations-wretched-ranks.json",
		},
	},
	"adjusted": {
		Name:  "adjusted",
		About: "the same four precons with 23 card slots replaced by basic lands",
		Seats: []string{
			"decks/adjusted/foundations-calling-all-angels.json",
			"decks/adjusted/foundations-keen-engineering.json",
			"decks/adjusted/foundations-reign-of-dragons.json",
			"decks/adjusted/foundations-wretched-ranks.json",
		},
	},
	"frontier": {
		Name:  "frontier",
		About: "the four original precons, less whatever the running kernel cannot play yet",
		Seats: []string{
			"decks/original/foundations-calling-all-angels.json",
			"decks/original/foundations-keen-engineering.json",
			"decks/original/foundations-reign-of-dragons.json",
			"decks/original/foundations-wretched-ranks.json",
		},
		Frontier: true,
	},
	"mirror": {
		Name:  "mirror",
		About: "four copies of the unmodified Wretched Ranks",
		Seats: []string{
			"decks/original/foundations-wretched-ranks.json",
			"decks/original/foundations-wretched-ranks.json",
			"decks/original/foundations-wretched-ranks.json",
			"decks/original/foundations-wretched-ranks.json",
		},
	},
}

// TableByName returns a fixed table.
func TableByName(name string) (Table, error) {
	t, ok := tables[name]
	if !ok {
		return Table{}, fmt.Errorf("unknown table %q (have %v)", name, TableNames())
	}
	return t, nil
}

// TableNames lists the fixed tables, sorted.
func TableNames() []string {
	names := make([]string, 0, len(tables))
	for n := range tables {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
