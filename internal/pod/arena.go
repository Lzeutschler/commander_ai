package pod

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// The arena is the yardstick for a pilot: one seat is played by the candidate,
// the other three by the heuristic bot. Decks rotate around the table and the
// candidate takes every seat, so over 16 games it plays each deck from each
// seat once. Every game is played twice on the same seed: once with the
// heuristic bot in the candidate's seat (the control) and once with the
// candidate. The control's win rate is what the candidate has to beat; it is
// 25% pooled, but far from that for a single deck.

// With ArenaOptions.Pool the tables are drawn instead of fixed: every game
// seats the candidate with a deck of the named split and three other decks of
// the whole pool in a random order. Control and candidate still play the same
// table on the same seed, so deck strength, seat and draw cancel in the gain.

// ArenaOptions says what to measure.
type ArenaOptions struct {
	Table Table
	// Pool, when set, draws a new table for every game: the candidate's deck
	// from this split of the deck pool (train, eval or all), its opponents'
	// from the whole pool. Table is then unused.
	Pool      string
	Candidate string // hbot, random or search:<sims>; a label only when Make is set
	// Make, when set, builds the candidate's policy instead of Candidate: a
	// fresh one per game.
	Make      func() lab.Policy
	FirstSeed uint64
	Games     int // a fixed table rounds it up to a multiple of 16
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
	Schema     int        `json:"schema"`
	Table      string     `json:"table"`
	Provenance Provenance `json:"provenance"`
	Candidate  string     `json:"candidate"`
	Opponents  string     `json:"opponents"`
	Config     RunConfig  `json:"config"`
	Decks      []DeckInfo `json:"decks,omitempty"` // a fixed table's lists
	// Pool and CandidateDecks describe a run on drawn tables.
	Pool           *PoolInfo `json:"pool,omitempty"`
	CandidateDecks string    `json:"candidate_decks,omitempty"`
	// RefusedTables counts drawn tables the kernel would not deal, by reason.
	// Each was replaced by the next draw.
	RefusedTables map[string]int `json:"refused_tables,omitempty"`
	Pooled        DeckArena      `json:"pooled"`
	ByDeck        []DeckArena    `json:"by_deck"`
	// Gain is the candidate's pooled win rate minus the control's, with a 95%
	// interval from the paired games (the same seed, seat and decks).
	Gain     float64 `json:"gain"`
	GainLow  float64 `json:"gain_low"`
	GainHigh float64 `json:"gain_high"`
	// Unfinished counts candidate games that stalled, ran out of steps or
	// failed, ControlUnfinished the control's. They count as losses.
	Unfinished        int `json:"unfinished"`
	ControlUnfinished int `json:"control_unfinished"`
	// Outcomes is the candidate's game by game, in seed order: W won, L lost,
	// U unfinished. Two runs over the same games are compared pair by pair
	// from it (CompareArena).
	Outcomes string `json:"outcomes,omitempty"`
}

// arenaGame is one seed's pair of games, from the candidate's seat.
type arenaGame struct {
	deck                          int // the deck in the candidate's seat
	control, candidate            bool
	unfinished, controlUnfinished bool
}

// playPair plays seed on su twice, the heuristic bot and then the candidate
// in seat c, and reports the time the candidate's game took.
func playPair(su *lab.Setup, seats, c int, seed uint64, mk func() lab.Policy, maxSteps int) (arenaGame, time.Duration) {
	play := func(cand lab.Policy) lab.Result {
		pols := pilots(seats)
		pols[c] = cand
		return lab.Play(su.Game(seed), pols, maxSteps)
	}
	done := func(r lab.Result) bool { return r.Outcome == lab.Won || r.Outcome == lab.Drawn }
	control := play(lab.HBot())
	t0 := time.Now()
	cand := play(mk())
	return arenaGame{
		control:           control.Outcome == lab.Won && control.Winner == c,
		candidate:         cand.Outcome == lab.Won && cand.Winner == c,
		unfinished:        !done(cand),
		controlUnfinished: !done(control),
	}, time.Since(t0)
}

// tally fills rep's win rates and gain from res. names[d] is deck d's name;
// a deck that never sat in the candidate's seat is left out.
func (rep *ArenaReport) tally(res []arenaGame, names []string) {
	by := make([]DeckArena, len(names))
	for d := range by {
		by[d].Deck = names[d]
	}
	rep.Pooled = DeckArena{Deck: "all"}
	// b: the candidate won where the control lost; c: the reverse
	var b, c int
	outcomes := make([]byte, len(res))
	for g, r := range res {
		outcomes[g] = 'L'
		if r.candidate {
			outcomes[g] = 'W'
		} else if r.unfinished {
			outcomes[g] = 'U'
		}
		for _, da := range []*DeckArena{&by[r.deck], &rep.Pooled} {
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
		if r.controlUnfinished {
			rep.ControlUnfinished++
		}
	}
	finish := func(t *Tally) {
		w := lab.Wilson(t.Wins, t.Games)
		t.Rate, t.Low, t.High = round(w.P), round(w.Lo), round(w.Hi)
	}
	rep.ByDeck = rep.ByDeck[:0]
	for d := range by {
		if by[d].Control.Games == 0 {
			continue
		}
		finish(&by[d].Control)
		finish(&by[d].Candidate)
		rep.ByDeck = append(rep.ByDeck, by[d])
	}
	finish(&rep.Pooled.Control)
	finish(&rep.Pooled.Candidate)
	rep.Outcomes = string(outcomes)
	rep.Gain, rep.GainLow, rep.GainHigh = pairedGain(b, c, len(res))
}

// pairedGain is the difference of two win rates over the same n games, with
// its 95% interval: b games only the first won, c games only the second.
func pairedGain(b, c, n int) (gain, low, high float64) {
	diff := float64(b-c) / float64(n)
	se := math.Sqrt(float64(b+c)-float64((b-c)*(b-c))/float64(n)) / float64(n)
	return round(diff), round(diff - 1.96*se), round(diff + 1.96*se)
}

// ArenaCompare is two candidates over the same games.
type ArenaCompare struct {
	A, B            string
	Games           int
	RateA, RateB    float64
	OnlyA, OnlyB    int     // games only the one won
	Gain, Low, High float64 // A's win rate minus B's, with its 95% interval
}

// CompareArena sets two arena runs over the same games side by side: the same
// tables, seeds, engine and settings, the candidate alone differing.
func CompareArena(a, b ArenaReport) (ArenaCompare, error) {
	cmp := ArenaCompare{A: a.Candidate, B: b.Candidate, Games: len(a.Outcomes)}
	same := func(x, y RunConfig) bool {
		return x.CardSet == y.CardSet && x.Surface == y.Surface && x.Mulligans == y.Mulligans && x.MaxSteps == y.MaxSteps &&
			x.FirstSeed == y.FirstSeed && x.Games == y.Games && strings.Join(x.Allow, "|") == strings.Join(y.Allow, "|")
	}
	digest := func(r ArenaReport) string {
		if r.Pool == nil {
			return ""
		}
		return r.Pool.Digest
	}
	switch {
	case a.Outcomes == "" || b.Outcomes == "":
		return cmp, errors.New("a report without the outcomes game by game (written before they were kept); run it again")
	case a.Table != b.Table || a.CandidateDecks != b.CandidateDecks || digest(a) != digest(b) ||
		!same(a.Config, b.Config) || len(a.Outcomes) != len(b.Outcomes):
		return cmp, errors.New("the two runs did not play the same games")
	case a.Provenance != b.Provenance:
		return cmp, errors.New("the two runs are from different engine or card revisions")
	}
	var wa, wb int
	for g := range a.Outcomes {
		x, y := a.Outcomes[g] == 'W', b.Outcomes[g] == 'W'
		if x {
			wa++
		}
		if y {
			wb++
		}
		if x && !y {
			cmp.OnlyA++
		}
		if y && !x {
			cmp.OnlyB++
		}
	}
	cmp.RateA, cmp.RateB = round(float64(wa)/float64(cmp.Games)), round(float64(wb)/float64(cmp.Games))
	cmp.Gain, cmp.Low, cmp.High = pairedGain(cmp.OnlyA, cmp.OnlyB, cmp.Games)
	return cmp, nil
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
	mk := opts.Make
	if mk == nil {
		var err error
		if mk, err = policyOf(opts.Candidate); err != nil {
			return rep, tim, err
		}
	}
	prov, err := ReadProvenance(root)
	if err != nil {
		return rep, tim, err
	}
	if opts.Pool != "" {
		return arenaPool(root, opts, prov, mk)
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

	res := make([]arenaGame, games)
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
				var d time.Duration
				res[g], d = playPair(setups[r], seats, c, opts.FirstSeed+uint64(g), mk, opts.MaxSteps)
				res[g].deck = (c + r) % seats
				spent += d
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
	}
	names := make([]string, seats)
	for d := range names {
		names[d] = decks[d].Name
	}
	rep.tally(res, names)

	tim = ArenaTiming{Host: ThisHost(), Workers: workers,
		CandidateSeconds:     round(time.Duration(candNanos).Seconds()),
		CandidateGamesPerSec: round(float64(games) / wall.Seconds()),
		MeasuredAt:           time.Now().UTC().Format(time.RFC3339)}
	return rep, tim, nil
}

// DrawTable draws game g's table from the pool: a deck of cands for the
// candidate, three other decks of the whole pool, in a random seat order.
// It is a function of the first seed, g and the attempt alone.
func DrawTable(rng *rand.Rand, pool *Pool, cands []int) (table []int, seat int) {
	cand := cands[rng.IntN(len(cands))]
	table = []int{cand}
	for _, i := range rng.Perm(len(pool.Decks)) {
		if i != cand && len(table) < 4 {
			table = append(table, i)
		}
	}
	rng.Shuffle(len(table), func(i, j int) { table[i], table[j] = table[j], table[i] })
	for i, d := range table {
		if d == cand {
			seat = i
		}
	}
	return table, seat
}

// arenaPool is Arena on drawn tables.
func arenaPool(root string, opts ArenaOptions, prov Provenance, mk func() lab.Policy) (ArenaReport, ArenaTiming, error) {
	var rep ArenaReport
	var tim ArenaTiming
	pool, err := LoadPool(root, opts.Allow)
	if err != nil {
		return rep, tim, err
	}
	cands, err := pool.In(opts.Pool)
	if err != nil {
		return rep, tim, err
	}
	games := opts.Games
	res := make([]arenaGame, games)
	workers := max(1, min(opts.Workers, games))
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var candNanos int64
	refused := map[string]int{}
	var failed error
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var spent time.Duration
			mine := map[string]int{}
			var myErr error
			for g := range next {
				rng := rand.New(rand.NewPCG(opts.FirstSeed, uint64(g)))
				var su *lab.Setup
				var table []int
				var seat int
				for attempt := 0; su == nil && attempt < 200; attempt++ {
					table, seat = DrawTable(rng, pool, cands)
					decks := make([]lab.Deck, len(table))
					for i, d := range table {
						decks[i] = pool.Decks[d].Deck
					}
					s, err := lab.Open(lab.Config{Set: PoolSet, Decks: decks, Commander: true, Full: true,
						Mulligans: opts.Mulligans, Allow: opts.Allow})
					if err != nil {
						mine[refusalKind(err)]++
						continue
					}
					su = s
				}
				if su == nil {
					myErr = fmt.Errorf("arena: game %d found no table the kernel deals in 200 draws", g)
					continue
				}
				var d time.Duration
				res[g], d = playPair(su, len(table), seat, opts.FirstSeed+uint64(g), mk, opts.MaxSteps)
				res[g].deck = table[seat]
				su.Close()
				spent += d
			}
			mu.Lock()
			candNanos += int64(spent)
			for k, n := range mine {
				refused[k] += n
			}
			if myErr != nil {
				failed = myErr
			}
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
	if failed != nil {
		return rep, tim, failed
	}

	rep = ArenaReport{
		Schema: Schema, Table: "pool", Provenance: prov, Candidate: opts.Candidate, Opponents: "hbot",
		Pool: &pool.Info, CandidateDecks: opts.Pool, RefusedTables: refused,
		Config: RunConfig{CardSet: PoolSet, Surface: "full", Pilot: opts.Candidate, Mulligans: opts.Mulligans,
			MaxSteps: opts.MaxSteps, FirstSeed: opts.FirstSeed, Games: games, Allow: sorted(opts.Allow)},
	}
	names := make([]string, len(pool.Decks))
	for d := range names {
		names[d] = pool.Decks[d].Deck.Name
	}
	rep.tally(res, names)
	tim = ArenaTiming{Host: ThisHost(), Workers: workers,
		CandidateSeconds:     round(time.Duration(candNanos).Seconds()),
		CandidateGamesPerSec: round(float64(games) / wall.Seconds()),
		MeasuredAt:           time.Now().UTC().Format(time.RFC3339)}
	return rep, tim, nil
}

// refusalKind names why the kernel would not deal a drawn table.
func refusalKind(err error) string {
	var inter *cardset.InteractionRefusal
	var card *cardset.RefusedError
	switch {
	case errors.As(err, &inter):
		return "interaction " + inter.Reason
	case errors.As(err, &card):
		return "card " + card.Card
	}
	return "other"
}
