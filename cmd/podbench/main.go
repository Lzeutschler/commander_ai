// Command podbench plays fixed four-seat Commander tables on the dirac kernel.
//
//	podbench run   -table adjusted -n 1000 -out results/baseline
//	podbench cards -table original -out results/baseline
//	podbench repro -table mirror -seed 17 -trace 30
//	podbench export -table frontier -allow decks/allow.txt -out /tmp/pool
//	podbench arena -candidate search:25 -n 1600 -out results/arena
//
// Exit status: 0 the table was played (or the report written), 1 an error,
// 2 bad usage, 3 the kernel refused to deal the table.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Lzeutschler/commander_ai/internal/pod"
)

const exitRefused = 3

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(os.Args[2:])
	case "cards":
		err = cmdCards(os.Args[2:])
	case "repro":
		err = cmdRepro(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "arena":
		err = cmdArena(os.Args[2:])
	default:
		usage()
	}
	var ref refused
	if errors.As(err, &ref) {
		os.Exit(exitRefused)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "podbench:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: podbench run|cards|repro|export|arena [flags]   (tables: %s)\n", strings.Join(pod.TableNames(), ", "))
	os.Exit(2)
}

type refused struct{}

func (refused) Error() string { return "table refused" }

// common holds the flags every subcommand takes.
type common struct {
	fs    *flag.FlagSet
	table *string
	root  *string
	allow *string
}

func newCommon(name, defTable string) common {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	return common{fs,
		fs.String("table", defTable, "table: "+strings.Join(pod.TableNames(), ", ")),
		fs.String("root", "", "repo root (default: found from the working directory)"),
		fs.String("allow", "", "file naming unverified cards to play anyway, one per line (decks/allow.txt)")}
}

func (c common) open(args []string) (string, pod.Table, []string, error) {
	_ = c.fs.Parse(args)
	t, err := pod.TableByName(*c.table)
	if err != nil {
		return "", t, nil, err
	}
	root := *c.root
	if root == "" {
		if root, err = pod.FindRoot(); err != nil {
			return "", t, nil, err
		}
	}
	var allow []string
	if *c.allow != "" {
		raw, err := os.ReadFile(*c.allow)
		if err != nil {
			return "", t, nil, err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
				allow = append(allow, line)
			}
		}
	}
	return root, t, allow, pod.UseCorpus(root)
}

func cmdRun(args []string) error {
	c := newCommon("run", "adjusted")
	def := pod.DefaultOptions(pod.Table{})
	n := c.fs.Int("n", def.Games, "games")
	seed := c.fs.Uint64("seed", def.FirstSeed, "first seed; games use seed, seed+1, ...")
	workers := c.fs.Int("workers", def.Workers, "parallel games")
	maxSteps := c.fs.Int("max-steps", def.MaxSteps, "decisions before a game counts as truncated")
	mull := c.fs.Int("mulligans", def.Mulligans, "London mulligans allowed per player")
	out := c.fs.String("out", "", "directory for <table>.json and <table>.timing.json")
	games := c.fs.String("games", "", "also write every game as a JSON line to this file")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	opts := pod.Options{Table: t, FirstSeed: *seed, Games: *n, Workers: *workers, MaxSteps: *maxSteps, Mulligans: *mull, Allow: allow}
	rep, rows, tim, err := pod.Run(root, opts)
	if err != nil {
		return err
	}
	fmt.Printf("table %s: %s\n", t.Name, t.About)
	fmt.Printf("engine %s%s  cards %s\n", short(rep.Provenance.EngineRev), dirty(rep.Provenance.EngineDirty), short(rep.Provenance.CardsRev))
	for _, d := range rep.Decks {
		fmt.Printf("  seat %d  %-22s %3d cards  sha256 %s\n", d.Seat, d.Name, d.Cards, d.SHA256[:12])
	}
	printSubstitutions(rep.Substitutions)
	if *out != "" {
		if err := writeJSON(filepath.Join(*out, t.Name+".json"), rep); err != nil {
			return err
		}
	}
	if rep.Status == "refused" {
		fmt.Printf("REFUSED (%s): %s\n", rep.Refusal.Kind, rep.Refusal.Error)
		return refused{}
	}
	s := rep.Summary
	fmt.Printf("seeds %d..%d  finished %d/%d  wins by seat %v  draws %d\n",
		opts.FirstSeed, opts.FirstSeed+uint64(opts.Games)-1, s.Finished, s.Games, s.Wins, s.Draws)
	fmt.Printf("turns %d  decisions %d  games digest %s\n", s.Turns, s.Decisions, s.GamesSHA256[:16])
	if len(rep.Aborts) > 0 {
		fmt.Printf("aborts %d: %s\n", len(rep.Aborts), countsOf(s.AbortsBy))
		for i, a := range rep.Aborts {
			if i == 10 {
				fmt.Printf("  ... %d more (see the report)\n", len(rep.Aborts)-i)
				break
			}
			fmt.Printf("  seed %-6d %s %s %s turn %d after %d decisions %s\n", a.Seed, a.Outcome, a.Reason, a.Class, a.Turns, a.Decisions, a.Error)
		}
	}
	fmt.Printf("timing (%s, %d workers): %.3fs play, %.1f finished games/s, %.0f decisions/s\n",
		tim.Host.CPU, tim.Workers, tim.PlaySeconds, tim.FinishedPerSecond, tim.DecisionsPerSecond)
	if *out != "" {
		if err := writeJSON(filepath.Join(*out, t.Name+".timing.json"), tim); err != nil {
			return err
		}
	}
	if *games != "" {
		var b strings.Builder
		enc := json.NewEncoder(&b)
		for _, r := range rows {
			_ = enc.Encode(r)
		}
		if err := os.WriteFile(*games, []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func cmdCards(args []string) error {
	c := newCommon("cards", "original")
	out := c.fs.String("out", "", "directory for cards-<table>.json")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	rep, err := pod.Cards(root, t, allow)
	if err != nil {
		return err
	}
	fmt.Printf("table %s: %d card slots the kernel does not play\n", t.Name, rep.GapSlots)
	for _, d := range rep.Decks {
		fmt.Printf("seat %d  %s: %d\n", d.Seat, d.Name, d.GapSlots)
		for _, g := range d.Gaps {
			fmt.Printf("    %-30s %s\n", g.Card, g.Reason)
		}
	}
	if rep.Interaction != nil {
		fmt.Printf("with those replaced by basic lands the table is still refused (%s):\n    %s\n", rep.Interaction.Kind, rep.Interaction.Error)
	} else {
		fmt.Println("with those replaced by basic lands the table deals")
	}
	if *out != "" {
		return writeJSON(filepath.Join(*out, "cards-"+t.Name+".json"), rep)
	}
	return nil
}

func cmdRepro(args []string) error {
	c := newCommon("repro", "adjusted")
	def := pod.DefaultOptions(pod.Table{})
	seed := c.fs.Uint64("seed", 1, "the seed to replay")
	maxSteps := c.fs.Int("max-steps", def.MaxSteps, "decisions before the game counts as truncated")
	mull := c.fs.Int("mulligans", def.Mulligans, "London mulligans allowed per player")
	trace := c.fs.Int("trace", 0, "print the last N decisions (-1: all)")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	var lines []string
	var fn func(step, turn, seat int, action string)
	if *trace != 0 {
		fn = func(step, turn, seat int, action string) {
			lines = append(lines, fmt.Sprintf("  #%-5d turn %-3d seat %d  %s", step, turn, seat, action))
			if *trace > 0 && len(lines) > *trace {
				lines = lines[1:]
			}
		}
	}
	opts := pod.Options{Table: t, MaxSteps: *maxSteps, Mulligans: *mull, Allow: allow}
	row, err := pod.Replay(root, opts, *seed, fn)
	if err != nil {
		return err
	}
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Printf("table %s seed %d: %s", t.Name, row.Seed, row.Outcome)
	if row.Outcome == "won" {
		fmt.Printf(" by seat %d", row.Winner)
	}
	if row.Reason != "" {
		fmt.Printf(" (%s, %s)", row.Reason, row.Class)
	}
	if row.Error != "" {
		fmt.Printf(" (%s)", row.Error)
	}
	fmt.Printf(", turn %d, %d decisions, position %s\n", row.Turns, row.Decisions, row.Hash)
	return nil
}

// cmdArena measures a pilot: the candidate in one seat against three heuristic
// bots, every deck from every seat, against the heuristic bot itself on the
// same seeds.
func cmdArena(args []string) error {
	c := newCommon("arena", "adjusted")
	def := pod.DefaultOptions(pod.Table{})
	cand := c.fs.String("candidate", "hbot", "the pilot to measure: hbot, random or search:<simulations>")
	n := c.fs.Int("n", 1600, "games, rounded up to a multiple of 16")
	seed := c.fs.Uint64("seed", 100001, "first seed")
	workers := c.fs.Int("workers", def.Workers, "parallel games")
	maxSteps := c.fs.Int("max-steps", def.MaxSteps, "decisions before a game counts as truncated")
	mull := c.fs.Int("mulligans", def.Mulligans, "London mulligans allowed per player")
	out := c.fs.String("out", "", "directory for arena-<candidate>.json and its timing file")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	rep, tim, err := pod.Arena(root, pod.ArenaOptions{Table: t, Candidate: *cand, FirstSeed: *seed, Games: *n,
		Workers: *workers, MaxSteps: *maxSteps, Mulligans: *mull, Allow: allow})
	if err != nil {
		return err
	}
	fmt.Printf("arena on table %s: %s in one seat against three hbot, %d games, seeds from %d\n", t.Name, rep.Candidate, rep.Config.Games, rep.Config.FirstSeed)
	fmt.Printf("  %-22s %8s %22s %22s\n", "candidate pilots", "games", "hbot in that seat", rep.Candidate)
	line := func(d pod.DeckArena) {
		fmt.Printf("  %-22s %8d %6.1f%% [%4.1f, %4.1f] %8.1f%% [%4.1f, %4.1f]\n", d.Deck, d.Candidate.Games,
			100*d.Control.Rate, 100*d.Control.Low, 100*d.Control.High, 100*d.Candidate.Rate, 100*d.Candidate.Low, 100*d.Candidate.High)
	}
	for _, d := range rep.ByDeck {
		line(d)
	}
	line(rep.Pooled)
	fmt.Printf("gain over hbot on the same games: %+.1f points [%+.1f, %+.1f]; %d candidate games unfinished\n",
		100*rep.Gain, 100*rep.GainLow, 100*rep.GainHigh, rep.Unfinished)
	fmt.Printf("timing (%s, %d workers): %.1f candidate games/s\n", tim.Host.CPU, tim.Workers, tim.CandidateGamesPerSec)
	if *out != "" {
		name := "arena-" + strings.ReplaceAll(rep.Candidate, ":", "-")
		if err := writeJSON(filepath.Join(*out, name+".json"), rep); err != nil {
			return err
		}
		return writeJSON(filepath.Join(*out, name+".timing.json"), tim)
	}
	return nil
}

// cmdExport writes the lists a table deals as a deck pool the engine's own
// tools read: <out>/decks/<name>.json and <out>/pool.tsv. `make lockstep`
// plays that pool on both engines and compares them decision by decision.
func cmdExport(args []string) error {
	c := newCommon("export", "frontier")
	out := c.fs.String("out", "", "directory to write (required)")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	if *out == "" {
		return errors.New("export: -out is required")
	}
	decks, infos, subs, err := pod.PlayedTable(root, t, allow)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(*out, "decks"), 0o755); err != nil {
		return err
	}
	var pool strings.Builder
	pool.WriteString("deck\tsplit\tcolors\tmain_colors\tcards\n")
	seen := map[string]bool{}
	for seat, d := range decks {
		if seen[d.Name] {
			return fmt.Errorf("export: table %s seats %s twice; a pool holds distinct decks", t.Name, d.Name)
		}
		seen[d.Name] = true
		if err := os.WriteFile(filepath.Join(*out, "decks", d.Name+".json"), pod.DeckJSON(d, subs, seat), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(&pool, "%s\ttrain\tC\tC\t%d\n", d.Name, infos[seat].Cards)
	}
	if err := os.WriteFile(filepath.Join(*out, "pool.tsv"), []byte(pool.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("table %s: %d decks written to %s\n", t.Name, len(decks), *out)
	printSubstitutions(subs)
	return nil
}

func printSubstitutions(subs []pod.Substitution) {
	if len(subs) == 0 {
		return
	}
	n := 0
	for _, s := range subs {
		n += s.Count
	}
	fmt.Printf("%d card slots replaced by basic lands:\n", n)
	for _, s := range subs {
		fmt.Printf("  seat %d  %-30s -> %-8s %s\n", s.Seat, s.Card, s.Basic, s.Reason)
	}
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func countsOf(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s %d", k, m[k])
	}
	return strings.Join(parts, ", ")
}

func short(rev string) string {
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

func dirty(d bool) string {
	if d {
		return " (modified)"
	}
	return ""
}
