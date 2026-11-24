package pod

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

func root(t *testing.T) string {
	t.Helper()
	r, err := FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The eight lists are frozen: every one is a 100-card Commander deck whose
// bytes match decks/SHA256SUMS. Needs no card corpus.
func TestDecksAreFrozen(t *testing.T) {
	r := root(t)
	want := map[string]string{}
	f, err := os.Open(filepath.Join(r, "decks", "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for sc := bufio.NewScanner(f); sc.Scan(); {
		if sum, file, ok := strings.Cut(sc.Text(), "  "); ok {
			want[file] = sum
		}
	}
	seen := map[string]bool{}
	for _, name := range TableNames() {
		tab, _ := TableByName(name)
		if len(tab.Seats) != 4 {
			t.Errorf("table %s has %d seats", name, len(tab.Seats))
		}
		_, infos, err := LoadTable(r, tab)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range infos {
			seen[d.File] = true
			if d.Cards != 100 || len(d.Commanders) != 1 {
				t.Errorf("%s: %d cards, commanders %v", d.File, d.Cards, d.Commanders)
			}
			if want[d.File] != d.SHA256 {
				t.Errorf("%s: sha256 %s, SHA256SUMS has %q", d.File, d.SHA256, want[d.File])
			}
		}
	}
	if len(seen) != 8 || len(want) != 8 {
		t.Errorf("%d lists in tables, %d in SHA256SUMS, want 8 and 8", len(seen), len(want))
	}
}

// The committed baseline reproduces byte for byte on this checkout: same
// revisions, same decks, same seeds give the same games. After a deliberate
// engine or deck change, regenerate with `make baseline` and review the diff.
func TestBaselineReproduces(t *testing.T) {
	r := root(t)
	if err := UseCorpus(r); err != nil {
		t.Skip(err)
	}
	if _, err := cardset.ByName(CardSet); err != nil {
		t.Skip("no card tables (run `make setup`): ", err)
	}
	for _, name := range TableNames() {
		t.Run(name, func(t *testing.T) {
			var want Report
			raw := readBaseline(t, r, name+".json", &want)
			tab, _ := TableByName(name)
			opts := DefaultOptions(tab)
			opts.FirstSeed, opts.Games = want.Config.FirstSeed, want.Config.Games
			opts.MaxSteps, opts.Mulligans = want.Config.MaxSteps, want.Config.Mulligans
			got, _, _, err := Run(r, opts)
			if err != nil {
				t.Fatal(err)
			}
			compare(t, raw, got)
		})
	}
	t.Run("cards-original", func(t *testing.T) {
		var want CardReport
		raw := readBaseline(t, r, "cards-original.json", &want)
		tab, _ := TableByName("original")
		got, err := Cards(r, tab)
		if err != nil {
			t.Fatal(err)
		}
		compare(t, raw, got)
	})
}

func readBaseline(t *testing.T, root, file string, into any) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "results", "baseline", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatal(err)
	}
	return raw
}

func compare(t *testing.T, want []byte, got any) {
	t.Helper()
	raw, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw)+"\n" != string(want) {
		t.Errorf("differs from the committed baseline; got:\n%s", raw)
	}
}
