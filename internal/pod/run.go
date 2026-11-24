package pod

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// Schema is the version of the report layout.
const Schema = 1

// Options says what to play.
type Options struct {
	Table     Table
	FirstSeed uint64
	Games     int
	Workers   int
	MaxSteps  int // decisions per game before it counts as truncated
	Mulligans int
	// Allow names cards to play although the kernel has no upstream evidence
	// for them. They must lower, and this repository vouches for them itself
	// (decks/allow.txt, make lockstep).
	Allow []string
}

// DefaultOptions are the settings of the 2026-11-24 baseline.
func DefaultOptions(t Table) Options {
	return Options{Table: t, FirstSeed: 1, Games: 1000, Workers: runtime.NumCPU(), MaxSteps: 20000, Mulligans: 1}
}

// RunConfig is the part of Options that decides the games.
type RunConfig struct {
	CardSet   string   `json:"card_set"`
	Surface   string   `json:"surface"`
	Pilot     string   `json:"pilot"`
	Mulligans int      `json:"mulligans"`
	MaxSteps  int      `json:"max_steps"`
	FirstSeed uint64   `json:"first_seed"`
	Games     int      `json:"games"`
	Allow     []string `json:"allow"` // unverified cards played on this repository's own evidence
}

// Refusal is why the kernel would not deal a table.
type Refusal struct {
	Kind   string   `json:"kind"` // card, interaction or other
	Card   string   `json:"card,omitempty"`
	Status string   `json:"status,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Cards  []string `json:"cards,omitempty"`
	Why    string   `json:"why,omitempty"`
	Error  string   `json:"error"`
}

// Summary tallies a run. Finished games ended by the rules (a win or a draw);
// the rest are aborts and have no result.
type Summary struct {
	Games       int            `json:"games"`
	Finished    int            `json:"finished"`
	Wins        []int          `json:"wins"` // by seat
	Draws       int            `json:"draws"`
	Stalled     int            `json:"stalled"`
	Truncated   int            `json:"truncated"`
	Failed      int            `json:"failed"`
	Turns       int64          `json:"turns"`
	Decisions   int64          `json:"decisions"`
	AbortsBy    map[string]int `json:"aborts_by"`
	GamesSHA256 string         `json:"games_sha256"` // over every GameRow as a JSON line
}

// GameRow is how one game ended.
type GameRow struct {
	Seed      uint64 `json:"seed"`
	Outcome   string `json:"outcome"`
	Winner    int    `json:"winner"` // seat, -1 without a winner
	Turns     int    `json:"turns"`
	Decisions int    `json:"decisions"`
	Hash      string `json:"hash"` // final position, every seat's view
	Reason    string `json:"reason,omitempty"`
	Class     string `json:"class,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Report is the reproducible record of a run: a second checkout on the same
// revisions writes the same bytes, whatever its hardware or worker count.
type Report struct {
	Schema     int        `json:"schema"`
	Table      string     `json:"table"`
	Provenance Provenance `json:"provenance"`
	Config     RunConfig  `json:"config"`
	Decks      []DeckInfo `json:"decks"`
	// Substitutions lists what a frontier table replaced; a frozen table has none.
	Substitutions []Substitution `json:"substitutions,omitempty"`
	Status        string         `json:"status"` // played or refused
	Refusal       *Refusal       `json:"refusal,omitempty"`
	Summary       *Summary       `json:"summary,omitempty"`
	Aborts        []GameRow      `json:"aborts,omitempty"`
}

// Timing is the hardware-dependent half of a run.
type Timing struct {
	Schema             int     `json:"schema"`
	Table              string  `json:"table"`
	Host               Host    `json:"host"`
	Workers            int     `json:"workers"`
	SetupSeconds       float64 `json:"setup_seconds"` // building the matchup table, once
	PlaySeconds        float64 `json:"play_seconds"`  // wall clock for all games, after one warm-up game
	Games              int     `json:"games"`
	Finished           int     `json:"finished"`
	FinishedPerSecond  float64 `json:"finished_per_second"`
	DecisionsPerSecond float64 `json:"decisions_per_second"`
	MeasuredAt         string  `json:"measured_at"`
}

// Run plays opts on the frozen lists under root. A table the kernel refuses is
// a Report with Status "refused" and no error; rows is then nil.
func Run(root string, opts Options) (rep Report, rows []GameRow, tim Timing, err error) {
	prov, err := ReadProvenance(root)
	if err != nil {
		return rep, nil, tim, err
	}
	decks, infos, subs, err := PlayedTable(root, opts.Table, opts.Allow)
	if err != nil {
		return rep, nil, tim, err
	}
	rep = Report{
		Schema: Schema, Table: opts.Table.Name, Provenance: prov, Decks: infos, Substitutions: subs,
		Config: RunConfig{CardSet: CardSet, Surface: "full", Pilot: "hbot", Mulligans: opts.Mulligans,
			MaxSteps: opts.MaxSteps, FirstSeed: opts.FirstSeed, Games: opts.Games, Allow: sorted(opts.Allow)},
	}

	t0 := time.Now()
	su, err := lab.Open(labConfig(decks, opts.Mulligans, opts.Allow))
	if err != nil {
		rep.Status, rep.Refusal = "refused", refusalOf(err)
		return rep, nil, tim, nil
	}
	defer su.Close()
	setup := time.Since(t0)

	workers := max(1, min(opts.Workers, opts.Games))
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)

	lab.Play(su.Game(0), pilots(len(decks)), opts.MaxSteps) // warm-up, not timed

	rows = make([]GameRow, opts.Games)
	next := make(chan int)
	var wg sync.WaitGroup
	t1 := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pols := pilots(len(decks))
			for i := range next {
				rows[i] = playOne(su, opts.FirstSeed+uint64(i), pols, opts.MaxSteps)
			}
		}()
	}
	for i := 0; i < opts.Games; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
	play := time.Since(t1)

	sum := summarize(len(decks), rows)
	rep.Status, rep.Summary = "played", &sum
	for _, r := range rows {
		if r.Outcome != "won" && r.Outcome != "drawn" {
			rep.Aborts = append(rep.Aborts, r)
		}
	}
	tim = Timing{
		Schema: Schema, Table: opts.Table.Name, Host: ThisHost(), Workers: workers,
		SetupSeconds: round(setup.Seconds()), PlaySeconds: round(play.Seconds()),
		Games: sum.Games, Finished: sum.Finished,
		FinishedPerSecond:  round(float64(sum.Finished) / play.Seconds()),
		DecisionsPerSecond: round(float64(sum.Decisions) / play.Seconds()),
		MeasuredAt:         time.Now().UTC().Format(time.RFC3339),
	}
	return rep, rows, tim, nil
}

// Replay plays one seed of a table and reports every decision to trace (which
// may be nil): the step, the turn, the deciding seat and the chosen action.
func Replay(root string, opts Options, seed uint64, trace func(step, turn, seat int, action string)) (GameRow, error) {
	decks, _, _, err := PlayedTable(root, opts.Table, opts.Allow)
	if err != nil {
		return GameRow{}, err
	}
	su, err := lab.Open(labConfig(decks, opts.Mulligans, opts.Allow))
	if err != nil {
		return GameRow{}, err
	}
	defer su.Close()
	g := su.Game(seed)
	pols := pilots(len(decks))
	if trace != nil {
		for seat, p := range pols {
			pols[seat] = tracing{p, g, trace}
		}
	}
	return rowOf(g, lab.Play(g, pols, opts.MaxSteps)), nil
}

// tracing reports each pick before it is applied. It forwards Start so the
// wrapped policy still binds to the game.
type tracing struct {
	lab.Policy
	g     *lab.Game
	trace func(step, turn, seat int, action string)
}

func (t tracing) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	a := t.Policy.Pick(s, d)
	t.trace(t.g.Steps(), t.g.Turn(), int(d.Player), t.g.Describe(a))
	return a
}

func (t tracing) Start(g *lab.Game, seat int) {
	if st, ok := t.Policy.(lab.Starter); ok {
		st.Start(g, seat)
	}
}

func labConfig(decks []lab.Deck, mulligans int, allow []string) lab.Config {
	return lab.Config{Set: CardSet, Decks: decks, Commander: true, Full: true, Mulligans: mulligans, Allow: allow}
}

// allowed is the fixed tables' card set with the named unverified cards playable.
func allowed(allow []string) (cardset.Set, error) { return allowedIn(CardSet, allow) }

// allowedIn is card set name with the named unverified cards playable.
func allowedIn(name string, allow []string) (cardset.Set, error) {
	set, err := cardset.ByName(name)
	if err != nil || len(allow) == 0 {
		return set, err
	}
	names := make(map[string]bool, len(allow))
	for _, n := range allow {
		names[n] = true
	}
	return cardset.AllowNamed(set, names), nil
}

// sorted is a sorted copy of names, never nil, so reports list it the same
// way whatever order the caller gave.
func sorted(names []string) []string {
	out := append([]string{}, names...)
	sort.Strings(out)
	return out
}

func pilots(seats int) []lab.Policy {
	pols := make([]lab.Policy, seats)
	for i := range pols {
		pols[i] = lab.HBot()
	}
	return pols
}

func playOne(su *lab.Setup, seed uint64, pols []lab.Policy, maxSteps int) GameRow {
	g := su.Game(seed)
	return rowOf(g, lab.Play(g, pols, maxSteps))
}

func rowOf(g *lab.Game, r lab.Result) GameRow {
	row := GameRow{Seed: r.Seed, Outcome: r.Outcome.String(), Winner: r.Winner, Turns: r.Turns,
		Decisions: r.Decisions, Reason: r.Stall, Class: r.Class, Error: r.Err}
	if r.Outcome != lab.Failed { // a failed game's state is whatever the panic left
		row.Hash = fmt.Sprintf("%016x", g.Hash())
	}
	return row
}

func summarize(seats int, rows []GameRow) Summary {
	s := Summary{Games: len(rows), Wins: make([]int, seats), AbortsBy: map[string]int{}}
	h := sha256.New()
	enc := json.NewEncoder(h)
	for _, r := range rows {
		_ = enc.Encode(r)
		s.Turns += int64(r.Turns)
		s.Decisions += int64(r.Decisions)
		switch r.Outcome {
		case "won":
			s.Wins[r.Winner]++
			s.Finished++
		case "drawn":
			s.Draws++
			s.Finished++
		case "stalled":
			s.Stalled++
			s.AbortsBy[r.Reason]++
		case "truncated":
			s.Truncated++
			s.AbortsBy["truncated"]++
		default:
			s.Failed++
			s.AbortsBy["failed"]++
		}
	}
	s.GamesSHA256 = hex.EncodeToString(h.Sum(nil))
	return s
}

func refusalOf(err error) *Refusal {
	r := &Refusal{Kind: "other", Error: err.Error()}
	var card *cardset.RefusedError
	var inter *cardset.InteractionRefusal
	switch {
	case errors.As(err, &card):
		r.Kind, r.Card, r.Status, r.Reason = "card", card.Card, card.Status.String(), card.Reason
	case errors.As(err, &inter):
		r.Kind, r.Reason, r.Why, r.Cards = "interaction", inter.Reason, inter.Why, inter.Cards
	}
	return r
}

func round(v float64) float64 { return float64(int64(v*1000+0.5)) / 1000 }
