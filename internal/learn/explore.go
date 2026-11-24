package learn

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// Exploring plays its pilot's answer, but with probability Eps answers a
// decision with a uniformly random option instead. Games recorded from such
// pilots hold positions the pilot alone never reaches: the ones a search has
// to judge when it tries a move the pilot would not make. The stream is
// seeded from the game and the seat.
type Exploring struct {
	lab.Policy
	Eps float64
	rng *rand.Rand
}

// Start is lab.Starter.
func (p *Exploring) Start(g *lab.Game, seat int) {
	if st, ok := p.Policy.(lab.Starter); ok {
		st.Start(g, seat)
	}
	p.rng = rand.New(rand.NewPCG(g.Seed(), 0x6578706c<<8|uint64(seat)))
}

// Pick is lab.Policy.
func (p *Exploring) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	if d.N > 1 && p.rng.Float64() < p.Eps {
		return d.Options[p.rng.IntN(int(d.N))]
	}
	return p.Policy.Pick(s, d)
}
