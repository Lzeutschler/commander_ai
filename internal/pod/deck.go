package pod

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/dirac/lab"
)

// DeckInfo identifies a deck file as it was played.
type DeckInfo struct {
	Seat       int      `json:"seat"`
	File       string   `json:"file"` // slash path relative to the repo root
	Name       string   `json:"name"`
	Commanders []string `json:"commanders"`
	Cards      int      `json:"cards"` // total copies, commander included
	SHA256     string   `json:"sha256"`
}

// LoadDeck reads the deck at rel (relative to root) and hashes the file's bytes.
func LoadDeck(root, rel string) (lab.Deck, DeckInfo, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return lab.Deck{}, DeckInfo{}, err
	}
	d, err := lab.ParseDeck(raw)
	if err != nil {
		return lab.Deck{}, DeckInfo{}, fmt.Errorf("%s: %w", rel, err)
	}
	sum := sha256.Sum256(raw)
	info := DeckInfo{File: rel, Name: d.Name, Commanders: d.Commanders, SHA256: hex.EncodeToString(sum[:])}
	for _, c := range d.Cards {
		info.Cards += c.Count
	}
	return d, info, nil
}

// LoadTable reads every seat of t as it is on disk.
func LoadTable(root string, t Table) ([]lab.Deck, []DeckInfo, error) {
	decks := make([]lab.Deck, len(t.Seats))
	infos := make([]DeckInfo, len(t.Seats))
	for seat, rel := range t.Seats {
		d, info, err := LoadDeck(root, rel)
		if err != nil {
			return nil, nil, err
		}
		info.Seat = seat
		decks[seat], infos[seat] = d, info
	}
	return decks, infos, nil
}

// PlayedTable is the lists a run of t deals: the files themselves for a
// frozen table, the derived lists and what was substituted for a frontier
// table. allow names unverified cards to count as playable.
func PlayedTable(root string, t Table, allow []string) ([]lab.Deck, []DeckInfo, []Substitution, error) {
	decks, infos, err := LoadTable(root, t)
	if err != nil || !t.Frontier {
		return decks, infos, nil, err
	}
	subs, err := frontier(decks, allow)
	if err != nil {
		return nil, nil, nil, err
	}
	for seat := range decks {
		raw := DeckJSON(decks[seat], subs, seat)
		sum := sha256.Sum256(raw)
		infos[seat].SHA256 = hex.EncodeToString(sum[:])
	}
	return decks, infos, subs, nil
}
