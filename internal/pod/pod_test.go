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
			opts.Allow = want.Config.Allow
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
		got, err := Cards(r, tab, want.Allow)
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

// The deck pool loads from the engine checkout: every deck is a 100-card
// Commander deck of cards the kernel plays, both splits are there, and an
// arena on drawn tables gives the same report whatever the worker count.
func TestPoolArena(t *testing.T) {
	r := root(t)
	if err := UseCorpus(r); err != nil {
		t.Skip(err)
	}
	if _, err := cardset.ByName(PoolSet); err != nil {
		t.Skip("no card tables (run `make setup`): ", err)
	}
	pool, err := LoadPool(r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pool.Info.Train < 20 || pool.Info.Eval < 4 {
		t.Fatalf("pool has %d train and %d eval decks", pool.Info.Train, pool.Info.Eval)
	}
	set, err := cardset.ByName(PoolSet)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range pool.Decks {
		n := 0
		for _, c := range d.Deck.Cards {
			n += c.Count
			if i, ok := set.Lookup(c.Name); !ok || !set.Supported()[i] {
				t.Errorf("%s still holds %s, which the kernel does not play", d.Deck.Name, c.Name)
			}
		}
		if n != 100 || len(d.Deck.Commanders) == 0 {
			t.Errorf("%s: %d cards, commanders %v", d.Deck.Name, n, d.Deck.Commanders)
		}
	}
	tab, _ := TableByName("adjusted")
	play := func(workers int) []byte {
		rep, _, err := Arena(r, ArenaOptions{Table: tab, Pool: "eval", Candidate: "random", FirstSeed: 7, Games: 48,
			Workers: workers, MaxSteps: 20000, Mulligans: 1})
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(rep)
		return raw
	}
	a, b := play(1), play(4)
	if string(a) != string(b) {
		t.Errorf("arena on drawn tables differs between 1 and 4 workers:\n%s\n%s", a, b)
	}

	// The same games against themselves: no gain. With one more win on one
	// side: that game alone.
	var rep ArenaReport
	if err := json.Unmarshal(a, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Outcomes) != 48 {
		t.Fatalf("outcomes of %d games, want 48", len(rep.Outcomes))
	}
	if cmp, err := CompareArena(rep, rep); err != nil || cmp.Gain != 0 || cmp.OnlyA != 0 || cmp.OnlyB != 0 {
		t.Errorf("a run against itself: %+v, %v", cmp, err)
	}
	better := rep
	i := strings.IndexAny(rep.Outcomes, "LU")
	better.Outcomes = rep.Outcomes[:i] + "W" + rep.Outcomes[i+1:]
	if cmp, err := CompareArena(better, rep); err != nil || cmp.OnlyA != 1 || cmp.OnlyB != 0 || cmp.Gain <= 0 {
		t.Errorf("one more win: %+v, %v", cmp, err)
	}
	other := rep
	other.Config.FirstSeed++
	if _, err := CompareArena(rep, other); err == nil {
		t.Error("runs from different seeds were compared")
	}
}
