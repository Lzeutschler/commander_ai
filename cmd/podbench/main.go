// Command podbench plays fixed four-seat Commander tables on the dirac kernel.
//
//	podbench run   -table adjusted -n 1000 -out results/baseline
//	podbench cards -table original -out results/baseline
//	podbench repro -table mirror -seed 17 -trace 30
//	podbench export -table frontier -allow decks/allow.txt -out /tmp/pool
//	podbench arena -candidate search:25 -n 1600 -out results/arena
//	podbench arena -pool eval -candidate net:models/v0.caiv:25
//	podbench compare results/arena/arena-pool-all-net-models_v3_caiv-100.json results/arena/arena-pool-all-hsearch-100.json
//	podbench record -split train -pilot hbot -n 4000 -out data/v0
//	podbench record -pilot explore:0.15:net:models/v3.caiv:25,hbot,hbot,hbot -keep 0 -out data/v4
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
	"strconv"
	"strings"

	"github.com/Lzeutschler/commander_ai/internal/learn"
	"github.com/Lzeutschler/commander_ai/internal/pod"
	"github.com/adams-shaun/gorge/dirac/lab"
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
	case "compare":
		err = cmdCompare(os.Args[2:])
	case "record":
		err = cmdRecord(os.Args[2:])
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
	fmt.Fprintf(os.Stderr, "usage: podbench run|cards|repro|export|arena|compare|record [flags]   (tables: %s)\n", strings.Join(pod.TableNames(), ", "))
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
	cand := c.fs.String("candidate", "hbot", "the pilot to measure: hbot, random, search:<simulations>, hsearch:<simulations> (the same search through this repository's pilot) or net:<value network file>:<simulations>")
	split := c.fs.String("pool", "", "draw a new table per game from the engine's precon pool; the candidate's deck comes from this split: train, eval or all (default: the fixed -table)")
	n := c.fs.Int("n", 1600, "games (a fixed table rounds up to a multiple of 16)")
	seed := c.fs.Uint64("seed", 100001, "first seed")
	workers := c.fs.Int("workers", def.Workers, "parallel games")
	maxSteps := c.fs.Int("max-steps", def.MaxSteps, "decisions before a game counts as truncated")
	mull := c.fs.Int("mulligans", def.Mulligans, "London mulligans allowed per player")
	out := c.fs.String("out", "", "directory for arena-<candidate>.json and its timing file")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	mk, err := pilotOf(*cand)
	if err != nil {
		return err
	}
	rep, tim, err := pod.Arena(root, pod.ArenaOptions{Table: t, Pool: *split, Candidate: *cand, Make: mk, FirstSeed: *seed, Games: *n,
		Workers: *workers, MaxSteps: *maxSteps, Mulligans: *mull, Allow: allow})
	if err != nil {
		return err
	}
	label := strings.NewReplacer(":", "-", "/", "_", ".", "_").Replace(rep.Candidate)
	name := "arena-" + label
	line := func(d pod.DeckArena) {
		fmt.Printf("  %-22s %8d %6.1f%% [%4.1f, %4.1f] %8.1f%% [%4.1f, %4.1f]\n", d.Deck, d.Candidate.Games,
			100*d.Control.Rate, 100*d.Control.Low, 100*d.Control.High, 100*d.Candidate.Rate, 100*d.Candidate.Low, 100*d.Candidate.High)
	}
	if p := rep.Pool; p != nil {
		name = "arena-pool-" + rep.CandidateDecks + "-" + label
		fmt.Printf("arena on drawn tables: %s with a deck of split %q against three hbot with any of %d precons (%d train, %d eval), %d games, seeds from %d\n",
			rep.Candidate, rep.CandidateDecks, p.Decks, p.Train, p.Eval, rep.Config.Games, rep.Config.FirstSeed)
		fmt.Printf("pool %s: replaced by basic lands %d unverified cards (%d slots) and %d slow cards (%d slots); %d decks left out for their commander\n",
			p.Digest[:12], p.ReplacedCards, p.ReplacedSlots, p.SkippedCards, p.SkippedSlots, len(p.Excluded))
		fmt.Printf("%d drawn tables refused and redrawn: %s\n", total(rep.RefusedTables), countsOf(rep.RefusedTables))
		fmt.Printf("  %-22s %8s %22s %22s\n", "candidate pilots", "games", "hbot in that seat", rep.Candidate)
	} else {
		fmt.Printf("arena on table %s: %s in one seat against three hbot, %d games, seeds from %d\n", t.Name, rep.Candidate, rep.Config.Games, rep.Config.FirstSeed)
		fmt.Printf("  %-22s %8s %22s %22s\n", "candidate pilots", "games", "hbot in that seat", rep.Candidate)
		for _, d := range rep.ByDeck {
			line(d)
		}
	}
	line(rep.Pooled)
	fmt.Printf("gain over hbot on the same games: %+.1f points [%+.1f, %+.1f]; unfinished games: %d candidate, %d control\n",
		100*rep.Gain, 100*rep.GainLow, 100*rep.GainHigh, rep.Unfinished, rep.ControlUnfinished)
	fmt.Printf("timing (%s, %d workers): %.1f candidate games/s\n", tim.Host.CPU, tim.Workers, tim.CandidateGamesPerSec)
	if *out != "" {
		if err := writeJSON(filepath.Join(*out, name+".json"), rep); err != nil {
			return err
		}
		return writeJSON(filepath.Join(*out, name+".timing.json"), tim)
	}
	return nil
}

// cmdCompare sets two arena reports over the same games side by side: the
// first candidate's win rate minus the second's, pair by pair.
func cmdCompare(args []string) error {
	if len(args) != 2 {
		return errors.New("compare: want two arena reports")
	}
	var reps [2]pod.ArenaReport
	for i, path := range args {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &reps[i]); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	cmp, err := pod.CompareArena(reps[0], reps[1])
	if err != nil {
		return fmt.Errorf("compare: %w", err)
	}
	fmt.Printf("%d games: %s wins %.1f%%, %s wins %.1f%%\n", cmp.Games, cmp.A, 100*cmp.RateA, cmp.B, 100*cmp.RateB)
	fmt.Printf("%s over %s on the same games: %+.1f points [%+.1f, %+.1f]; only the first won %d, only the second %d\n",
		cmp.A, cmp.B, 100*cmp.Gain, 100*cmp.Low, 100*cmp.High, cmp.OnlyA, cmp.OnlyB)
	return nil
}

// pilotOf builds the pilots this command adds to the arena's own: the tree
// search through this repository's pilot, over the engine's heuristic leaf
// (hsearch:<simulations>) or a value network (net:<file>:<simulations>, or
// net:<file>:<simulations>:<spread> to stretch the network's output for the
// search, learn.Evaluator.Spread). For any other name it returns nil and the
// arena resolves the name itself.
func pilotOf(spec string) (func() lab.Policy, error) {
	var net *learn.ValueNet
	var sims string
	spread := 1.0
	switch {
	case strings.HasPrefix(spec, "hsearch:"):
		sims = strings.TrimPrefix(spec, "hsearch:")
	case strings.HasPrefix(spec, "net:"):
		parts := strings.Split(spec, ":")
		if len(parts) != 3 && len(parts) != 4 {
			return nil, fmt.Errorf("candidate %q: want net:<file>:<simulations>[:<spread>]", spec)
		}
		var err error
		if net, err = learn.LoadValueNet(parts[1]); err != nil {
			return nil, err
		}
		sims = parts[2]
		if len(parts) == 4 {
			if spread, err = strconv.ParseFloat(parts[3], 64); err != nil || spread < 1 {
				return nil, fmt.Errorf("candidate %q: bad spread", spec)
			}
		}
	default:
		return nil, nil
	}
	n, err := strconv.Atoi(sims)
	if err != nil || n < 1 {
		return nil, fmt.Errorf("candidate %q: bad simulation count", spec)
	}
	reg, err := learn.Registry()
	if err != nil {
		return nil, err
	}
	if _, err := learn.NewSearchPolicy(n, net, reg); err != nil {
		return nil, err
	}
	return func() lab.Policy {
		p, _ := learn.NewSearchPolicy(n, net, reg)
		p.Spread = spread
		return p
	}, nil
}

// cmdRecord plays games at tables drawn from one split of the deck pool and
// writes sampled positions with their outcomes: the training data of a value
// network (train/train_value.py).
func cmdRecord(args []string) error {
	c := newCommon("record", "adjusted")
	def := pod.DefaultOptions(pod.Table{})
	split := c.fs.String("split", "train", "pool split every deck is drawn from: train, eval or all")
	pilot := c.fs.String("pilot", "hbot", "who plays: one pilot for all four seats, or four with commas between that are dealt to the seats afresh each game. "+
		"A pilot is hbot, hsearch:<simulations>, net:<file>:<simulations>, or explore:<share>:<pilot> for a pilot that answers that share of its decisions at random (ehbot:<share> is explore:<share>:hbot)")
	keepArg := c.fs.String("keep", "", "with four pilots: the positions in -pilot (from 0, commas between) whose seats are recorded; empty records every seat")
	n := c.fs.Int("n", 4000, "games")
	seed := c.fs.Uint64("seed", 500001, "first seed (keep it apart from the arena's)")
	every := c.fs.Int("every", 40, "sample every N-th decision of a game")
	workers := c.fs.Int("workers", def.Workers, "parallel games")
	maxSteps := c.fs.Int("max-steps", def.MaxSteps, "decisions before a game counts as truncated")
	mull := c.fs.Int("mulligans", def.Mulligans, "London mulligans allowed per player")
	out := c.fs.String("out", "", "directory to write (required)")
	root, _, allow, err := c.open(args)
	if err != nil {
		return err
	}
	if *out == "" {
		return errors.New("record: -out is required")
	}
	var pilots []func() lab.Policy
	for _, spec := range strings.Split(*pilot, ",") {
		mk, err := recordPilot(spec)
		if err != nil {
			return err
		}
		pilots = append(pilots, mk)
	}
	var keep []bool
	if *keepArg != "" {
		keep = make([]bool, len(pilots))
		for _, k := range strings.Split(*keepArg, ",") {
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(pilots) {
				return fmt.Errorf("record: -keep %q names no pilot of %d", k, len(pilots))
			}
			keep[i] = true
		}
	}
	meta, err := learn.Record(root, learn.RecordOptions{Split: *split, Pilots: pilots, Keep: keep, PilotName: *pilot, FirstSeed: *seed, Games: *n,
		Every: *every, Workers: *workers, MaxSteps: *maxSteps, Mulligans: *mull, Allow: allow, Out: *out})
	if err != nil {
		return err
	}
	fmt.Printf("recorded %d positions from %d of %d games (%s decks, pilot %s) to %s: %d features each, %.1f%% wins\n",
		meta.Records, meta.Finished, meta.Games, meta.Split, meta.Pilot, *out, meta.Width, 100*float64(meta.Positive)/float64(max(1, meta.Records)))
	return nil
}

// recordPilot reads one pilot of `podbench record`.
func recordPilot(spec string) (func() lab.Policy, error) {
	if eps, ok := strings.CutPrefix(spec, "ehbot:"); ok {
		spec = "explore:" + eps + ":hbot"
	}
	if rest, ok := strings.CutPrefix(spec, "explore:"); ok {
		share, inner, ok := strings.Cut(rest, ":")
		eps, err := strconv.ParseFloat(share, 64)
		if !ok || err != nil || eps <= 0 || eps >= 1 {
			return nil, fmt.Errorf("record: pilot %q: want explore:<share of random answers>:<pilot>", spec)
		}
		mk, err := recordPilot(inner)
		if err != nil {
			return nil, err
		}
		return func() lab.Policy { return &learn.Exploring{Policy: mk(), Eps: eps} }, nil
	}
	if spec == "hbot" {
		return lab.HBot, nil
	}
	mk, err := pilotOf(spec)
	if err != nil {
		return nil, err
	}
	if mk == nil {
		return nil, fmt.Errorf("record: unknown pilot %q", spec)
	}
	return mk, nil
}

// cmdExport writes the lists a table deals as a deck pool the engine's own
// tools read: <out>/decks/<name>.json and <out>/pool.tsv. `make lockstep`
// plays that pool on both engines and compares them decision by decision.
func cmdExport(args []string) error {
	c := newCommon("export", "frontier")
	out := c.fs.String("out", "", "directory to write (required)")
	asPool := c.fs.Bool("pool", false, "export the precon pool as the arena plays it instead of a table")
	root, t, allow, err := c.open(args)
	if err != nil {
		return err
	}
	if *out == "" {
		return errors.New("export: -out is required")
	}
	if *asPool {
		pool, err := pod.LoadPool(root, allow)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(*out, "decks"), 0o755); err != nil {
			return err
		}
		var tsv strings.Builder
		tsv.WriteString("deck\tsplit\tcolors\tmain_colors\tcards\n")
		for _, d := range pool.Decks {
			if err := os.WriteFile(filepath.Join(*out, "decks", d.Deck.Name+".json"), pod.DeckJSON(d.Deck, nil, 0), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(&tsv, "%s\t%s\tC\tC\t100\n", d.Deck.Name, d.Split)
		}
		if err := os.WriteFile(filepath.Join(*out, "pool.tsv"), []byte(tsv.String()), 0o644); err != nil {
			return err
		}
		fmt.Printf("precon pool: %d decks written to %s\n", len(pool.Decks), *out)
		return nil
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

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
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
