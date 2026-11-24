package learn

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/lab"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Evaluator values the leaves of the engine's tree search with a ValueNet. It
// supplies no prior: like the engine's heuristic leaf, the search then treats
// every candidate alike, so the two differ in the leaf value alone.
//
// Spread, when above 1, stretches the network's output around an even chance
// before the search reads it: v' = sigmoid(Spread * (logit(v) - logit(1/n)))
// for n seats still in the game. It keeps the order of any two positions and
// widens the gap between them. A win probability moves little with one
// action, and the search's exploration term is sized for values that use the
// whole of [0, 1], as the hand-made leaf's do.
type Evaluator struct {
	net    *ValueNet
	f      *Featurizer
	sc     *Scratch
	Spread float64
}

// Value is search.Evaluator: the win probability of seat at s.
func (e *Evaluator) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if over, winner := s.Terminal(); over {
		switch {
		case winner < 0:
			return 0.5
		case uint8(winner) == seat:
			return 1
		}
		return 0
	}
	if s.FeatLost(seat) {
		return 0
	}
	v := e.net.Value(e.f.Encode(s, d, seat), e.sc)
	if e.Spread <= 1 {
		return v
	}
	alive := 0
	for p := uint8(0); p < s.NumSeats(); p++ {
		if !s.FeatLost(p) {
			alive++
		}
	}
	even := 1 / float64(max(alive, 2))
	x := min(max(float64(v), 1e-6), 1-1e-6)
	z := e.Spread * (math.Log(x/(1-x)) - math.Log(even/(1-even)))
	return float32(1 / (1 + math.Exp(-z)))
}

// Priors is search.Evaluator. The evaluator is no search.PriorSource, so the
// search never reads what it writes.
func (e *Evaluator) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	clear(out[:d.N])
}

// SearchPolicy answers a seat's decisions with the engine's tree search over
// a leaf evaluator: the engine's heuristic when Net is nil, else the network.
// It follows dirac/lab's own search policy step for step, so with a nil Net it
// plays the games lab.Search plays.
type SearchPolicy struct {
	Opts search.Options
	Net  *ValueNet
	Reg  *cards.Registry // needed with a Net
	// Spread is the Evaluator's (1 or less: the network's output as it is).
	Spread float64

	tbl   *kop.Table
	sr    *search.Searcher
	ev    *Evaluator
	st    search.SeatState
	seed  uint64
	queue []dirac.Action
}

// NewSearchPolicy checks the options. net may be nil.
func NewSearchPolicy(sims int, net *ValueNet, reg *cards.Registry) (*SearchPolicy, error) {
	o := lab.SearchOptions(sims)
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if net != nil && reg == nil {
		return nil, fmt.Errorf("a network search needs the card registry")
	}
	return &SearchPolicy{Opts: o, Net: net, Reg: reg}, nil
}

func splitmix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// Start is lab.Starter.
func (p *SearchPolicy) Start(g *lab.Game, seat int) {
	if tbl := g.State().Table(); p.tbl != tbl {
		bot := hbot.New(hbot.NewFacts(tbl))
		var eval search.Evaluator = search.HeuristicLeaf{}
		if p.Net != nil {
			if p.ev == nil {
				p.ev = &Evaluator{net: p.Net, f: NewFeaturizer(p.Reg), sc: p.Net.NewScratch(), Spread: p.Spread}
			}
			if err := p.ev.f.SetTable(tbl); err != nil {
				panic(fmt.Sprintf("learn: features for the table: %v", err))
			}
			eval = p.ev
		}
		p.tbl, p.sr = tbl, search.NewSearcher(p.Opts, eval, bot)
	}
	p.st, p.queue = search.SeatState{}, p.queue[:0]
	p.seed = splitmix(0 ^ splitmix(g.Seed()^uint64(seat+1)*0x9e3779b97f4a7c15))
}

// Pick is lab.Policy.
func (p *SearchPolicy) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	if a, ok := p.next(d); ok {
		return a
	}
	res := p.sr.SeatSearch(s, d, p.seed, &p.st)
	p.queue = append(p.queue[:0], res.Acts[:res.NActs]...)
	if a, ok := p.next(d); ok {
		return a
	}
	return d.Options[0]
}

func (p *SearchPolicy) next(d *dirac.Decision) (dirac.Action, bool) {
	for len(p.queue) > 0 {
		a := p.queue[0]
		p.queue = p.queue[1:]
		for _, o := range d.Options[:d.N] {
			if o == a {
				return a, true
			}
		}
	}
	return 0, false
}
