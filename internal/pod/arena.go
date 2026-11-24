package pod

import (
	"fmt"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/dirac/lab"
)

// The arena is the yardstick for a pilot: one seat is played by the candidate,
// the other three by the heuristic bot. Decks rotate around the table and the
// candidate takes every seat, so over 16 games it plays each deck from each
// seat once. Every game is played twice on the same seed: once with the
// heuristic bot in the candidate's seat (the control) and once with the
// candidate. The control's win rate is what the candidate has to beat; it is
// 25% pooled, but far from that for a single deck.

// ArenaOptions says what to measure.
type ArenaOptions struct {
	Table     Table
	Candidate string // hbot, random or search:<sims>
	FirstSeed uint64
	Games     int // rounded up to a multiple of 16
	Workers   int
	MaxSteps  int
	Mulligans int
	Allow     []string
}

// Tally counts one seat's wins over a set of games.
type Tally struct {
	Games int     `json:"games"`
	Wins  int     `json:"wins"`
	Rate  float64 `json:"rate"`
	Low   float64 `json:"low"`  // 95% Wilson interval
	High  float64 `json:"high"` //
}

// DeckArena is the candidate against the control while piloting one deck.
type DeckArena struct {
	Deck      string `json:"deck"`
	Control   Tally  `json:"control"`
	Candidate Tally  `json:"candidate"`
}

// ArenaReport is the reproducible record of an arena run.
type ArenaReport struct {
	Schema     int         `json:"schema"`
	Table      string      `json:"table"`
	Provenance Provenance  `json:"provenance"`
	Candidate  string      `json:"candidate"`
	Opponents  string      `json:"opponents"`
	Config     RunConfig   `json:"config"`
	Decks      []DeckInfo  `json:"decks"`
	Pooled     DeckArena   `json:"pooled"`
	ByDeck     []DeckArena `json:"by_deck"`
	// Gain is the candidate's pooled win rate minus the control's, with a 95%
	// interval from the paired games (the same seed, seat and decks).
	Gain     float64 `json:"gain"`
	GainLow  float64 `json:"gain_low"`
	GainHigh float64 `json:"gain_high"`
	// Unfinished counts candidate games that stalled, ran out of steps or
	// failed. They count as losses.
	Unfinished int `json:"unfinished"`
}

// ArenaTiming is the hardware-dependent half.
type ArenaTiming struct {
	Host                 Host    `json:"host"`
	Workers              int     `json:"workers"`
	CandidateSeconds     float64 `json:"candidate_seconds"`
	CandidateGamesPerSec float64 `json:"candidate_games_per_second"`
	MeasuredAt           string  `json:"measured_at"`
}

// policyOf builds the candidate's policy. Each call returns a fresh one: a
// policy carries per-game state.
func policyOf(spec string) (func() lab.Policy, error) {
	switch {
	case spec == "hbot":
		return lab.HBot, nil
	case spec == "random":
		return func() lab.Policy { return lab.Random(1) }, nil
	case strings.HasPrefix(spec, "search:"):
		sims, err := strconv.Atoi(strings.TrimPrefix(spec, "search:"))
		if err != nil || sims < 1 {
			return nil, fmt.Errorf("candidate %q: want search:<simulations>", spec)
		}
		return func() lab.Policy { return lab.Search(lab.SearchOptions(sims)) }, nil
	}
	return nil, fmt.Errorf("unknown candidate %q (hbot, random, search:<simulations>)", spec)
}

// Arena plays opts.
func Arena(root string, opts ArenaOptions) (ArenaReport, ArenaTiming, error) {
	var rep ArenaReport
	var tim ArenaTiming
	mk, err := policyOf(opts.Candidate)
	if err != nil {
		return rep, tim, err
	}
	prov, err := ReadProvenance(root)
	if err != nil {
		return rep, tim, err
	}
	decks, infos, _, err := PlayedTable(root, opts.Table, opts.Allow)
	if err != nil {
		return rep, tim, err
	}
	seats := len(decks)
	configs := seats * seats
	games := (opts.Games + configs - 1) / configs * configs

	// one matchup per rotation: seat i holds deck (i+r) mod seats
	setups := make([]*lab.Setup, seats)
	for r := range setups {
		rot := make([]lab.Deck, seats)
		for i := range rot {
			rot[i] = decks[(i+r)%seats]
		}
		su, err := lab.Open(labConfig(rot, opts.Mulligans, opts.Allow))
		if err != nil {
			return rep, tim, err
		}
		defer su.Close()
		setups[r] = su
	}

	type result struct {
		deck               int // the deck in the candidate's seat
		control, candidate bool
		unfinished         bool
	}
	res := make([]result, games)
	workers := max(1, min(opts.Workers, games))
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)
	next := make(chan int)
	var wg sync.WaitGroup
	var candNanos int64
	var mu sync.Mutex
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var spent time.Duration
			for g := range next {
				r, c := g%seats, (g/seats)%seats
				seed := opts.FirstSeed + uint64(g)
				play := func(cand lab.Policy) lab.Result {
					pols := pilots(seats)
					pols[c] = cand
					return lab.Play(setups[r].Game(seed), pols, opts.MaxSteps)
				}
				control := play(lab.HBot())
				t0 := time.Now()
				cand := play(mk())
				spent += time.Since(t0)
				res[g] = result{
					deck:       (c + r) % seats,
					control:    control.Outcome == lab.Won && control.Winner == c,
					candidate:  cand.Outcome == lab.Won && cand.Winner == c,
					unfinished: cand.Outcome != lab.Won && cand.Outcome != lab.Drawn,
				}
			}
			mu.Lock()
			candNanos += int64(spent)
			mu.Unlock()
		}()
	}
	t0 := time.Now()
	for g := 0; g < games; g++ {
		next <- g
	}
	close(next)
	wg.Wait()
	wall := time.Since(t0)

	rep = ArenaReport{
		Schema: Schema, Table: opts.Table.Name, Provenance: prov, Candidate: opts.Candidate, Opponents: "hbot", Decks: infos,
		Config: RunConfig{CardSet: CardSet, Surface: "full", Pilot: opts.Candidate, Mulligans: opts.Mulligans,
			MaxSteps: opts.MaxSteps, FirstSeed: opts.FirstSeed, Games: games, Allow: sorted(opts.Allow)},
		ByDeck: make([]DeckArena, seats),
	}
	// b: the candidate won where the control lost; c: the reverse
	var b, c int
	for d := range rep.ByDeck {
		rep.ByDeck[d].Deck = decks[d].Name
	}
	rep.Pooled.Deck = "all"
	for _, r := range res {
		for _, da := range []*DeckArena{&rep.ByDeck[r.deck], &rep.Pooled} {
			da.Control.Games++
			da.Candidate.Games++
			if r.control {
				da.Control.Wins++
			}
			if r.candidate {
				da.Candidate.Wins++
			}
		}
		if r.candidate && !r.control {
			b++
		}
		if r.control && !r.candidate {
			c++
		}
		if r.unfinished {
			rep.Unfinished++
		}
	}
	finish := func(t *Tally) {
		w := lab.Wilson(t.Wins, t.Games)
		t.Rate, t.Low, t.High = round(w.P), round(w.Lo), round(w.Hi)
	}
	for d := range rep.ByDeck {
		finish(&rep.ByDeck[d].Control)
		finish(&rep.ByDeck[d].Candidate)
	}
	finish(&rep.Pooled.Control)
	finish(&rep.Pooled.Candidate)
	n := float64(games)
	diff := float64(b-c) / n
	se := math.Sqrt(float64(b+c)-float64((b-c)*(b-c))/n) / n
	rep.Gain, rep.GainLow, rep.GainHigh = round(diff), round(diff-1.96*se), round(diff+1.96*se)

	tim = ArenaTiming{Host: ThisHost(), Workers: workers,
		CandidateSeconds:     round(time.Duration(candNanos).Seconds()),
		CandidateGamesPerSec: round(float64(games) / wall.Seconds()),
		MeasuredAt:           time.Now().UTC().Format(time.RFC3339)}
	return rep, tim, nil
}
