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

// LoadTable reads every seat of t.
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
