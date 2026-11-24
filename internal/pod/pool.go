package pod

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/lab"
)

// The deck pool is the engine's own set of Commander precons: the lists it
// ships for its lockstep runs, with a train and an eval split. They are read
// from the engine checkout, not copied here, so the pool is pinned by the
// engine revision; the report carries a digest of what was read.
//
// The engine built those lists by replacing cards its kernel refused with
// basic lands. On top of that, a card the kernel lowers but nobody has
// evidence for is replaced the same way here, unless decks/allow.txt vouches
// for it: an agent that learns from these games would otherwise learn to
// exploit whatever such a card gets wrong.

// PoolSet is the dirac card set the pool's decks are dealt from.
const PoolSet = "edh"

const poolArchive = EngineDir + "/dirac/oracle/testdata/slices/edh/decks.tar.gz"

// PoolDeck is one deck of the pool.
type PoolDeck struct {
	Deck  lab.Deck
	Split string // train or eval
}

// PoolInfo identifies the pool a run drew its tables from.
type PoolInfo struct {
	Source        string `json:"source"`
	CardSet       string `json:"card_set"`
	Decks         int    `json:"decks"`
	Train         int    `json:"train"`
	Eval          int    `json:"eval"`
	Digest        string `json:"digest"`         // over the deck files and the split, as read
	ReplacedCards int    `json:"replaced_cards"` // distinct unverified cards replaced by basic lands
	ReplacedSlots int    `json:"replaced_slots"`
	// SkippedCards and SkippedSlots count what decks/pool-skip.txt replaced:
	// cards that slow the kernel down for every table holding them.
	SkippedCards int `json:"skipped_cards"`
	SkippedSlots int `json:"skipped_slots"`
	// Excluded names the decks left out because their commander is itself
	// unverified or on the skip list: there is no playing such a deck without
	// the card.
	Excluded []string `json:"excluded"`
}

// Pool is the loaded deck pool, sorted by deck name.
type Pool struct {
	Decks []PoolDeck
	Info  PoolInfo
}

const poolSkipFile = "decks/pool-skip.txt"

// readNames reads a list file: one card name per line, # comments.
func readNames(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out, nil
}

// LoadPool reads the pool from the engine checkout under root.
func LoadPool(root string, allow []string) (*Pool, error) {
	skipList, err := readNames(filepath.Join(root, filepath.FromSlash(poolSkipFile)))
	if err != nil {
		return nil, err
	}
	skip := map[string]bool{}
	for _, n := range skipList {
		skip[n] = true
	}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(poolArchive)))
	if err != nil {
		return nil, fmt.Errorf("deck pool: %w (run `make setup`)", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for tr := tar.NewReader(gz); ; {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[path.Clean(h.Name)] = raw
	}
	split := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(files["pool.tsv"]))
	for sc.Scan() {
		if cols := strings.Split(sc.Text(), "\t"); len(cols) >= 2 && cols[0] != "deck" {
			split[cols[0]] = cols[1]
		}
	}
	var names []string
	for name := range files {
		if strings.HasPrefix(name, "decks/") && strings.HasSuffix(name, ".json") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	set, err := allowedIn(PoolSet, allow)
	if err != nil {
		return nil, err
	}
	p := &Pool{Info: PoolInfo{Source: poolArchive, CardSet: PoolSet, Excluded: []string{}}}
	digest := sha256.New()
	replaced := map[string]bool{}
	skipped := map[string]bool{}
	for _, name := range names {
		stem := strings.TrimSuffix(path.Base(name), ".json")
		d, err := lab.ParseDeck(files[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if d.Name == "" {
			d.Name = stem
		}
		sp := split[stem]
		if sp != "train" && sp != "eval" {
			return nil, fmt.Errorf("deck pool: %s has split %q in pool.tsv", stem, sp)
		}
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(digest, "%s\t%s\t%s\n", stem, sp, hex.EncodeToString(sum[:]))
		var gaps []string
		commander := false
		for _, c := range d.Cards {
			i, ok := set.Lookup(c.Name)
			unplayed := !ok || !set.Supported()[i]
			if unplayed {
				gaps = append(gaps, c.Name)
			}
			if unplayed || skip[c.Name] {
				for _, cmd := range d.Commanders {
					commander = commander || cmd == c.Name
				}
			}
		}
		if commander {
			p.Info.Excluded = append(p.Info.Excluded, stem)
			continue
		}
		for _, card := range gaps {
			n, _, err := replaceCard(&d, card)
			if err != nil {
				return nil, err
			}
			replaced[card] = true
			p.Info.ReplacedSlots += n
		}
		var slow []string
		for _, c := range d.Cards {
			if skip[c.Name] {
				slow = append(slow, c.Name)
			}
		}
		for _, card := range slow {
			n, _, err := replaceCard(&d, card)
			if err != nil {
				return nil, err
			}
			skipped[card] = true
			p.Info.SkippedSlots += n
		}
		p.Decks = append(p.Decks, PoolDeck{Deck: d, Split: sp})
		if sp == "train" {
			p.Info.Train++
		} else {
			p.Info.Eval++
		}
	}
	p.Info.Decks, p.Info.ReplacedCards, p.Info.SkippedCards = len(p.Decks), len(replaced), len(skipped)
	p.Info.Digest = hex.EncodeToString(digest.Sum(nil))
	if p.Info.Decks < 4 {
		return nil, fmt.Errorf("deck pool: %d decks in %s", p.Info.Decks, poolArchive)
	}
	return p, nil
}

// In lists the indexes of the decks of split: train, eval or all.
func (p *Pool) In(split string) ([]int, error) {
	if split != "train" && split != "eval" && split != "all" {
		return nil, fmt.Errorf("unknown pool split %q (train, eval, all)", split)
	}
	var out []int
	for i, d := range p.Decks {
		if split == "all" || d.Split == split {
			out = append(out, i)
		}
	}
	return out, nil
}

// replaceCard gives every slot of card in d to d's most common basic land,
// keeping the order of the other cards, and reports how many slots and which
// basic.
func replaceCard(d *lab.Deck, card string) (int, string, error) {
	basic := commonBasic(d.Cards)
	if basic < 0 {
		return 0, "", fmt.Errorf("%s: no basic land to stand in for %s", d.Name, card)
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
	return n, name, nil
}
