// Package learn connects the engine to a learned model: it turns positions
// into fixed-width feature vectors, records them from games with their
// outcomes, and plays a trained value network inside the engine's tree search.
package learn

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The feature vector is built from the engine's own encoding of a position as
// one seat sees it (dirac/feat, the four-seat pod set): the opponents' hands
// are counts, libraries are counts, nothing hidden leaks in.
//
//	dense    the encoder's state scalars: turn, step, mana, a block per seat
//	pooled   per card group (each seat's battlefield, the viewer's hand, the
//	         stack, ...): the number of cards, and the scaled sum and the
//	         maximum of the cards' per-card records
//
// It carries no card identities, only what each card is and does in general
// terms, so nothing in it is tied to a particular deck list.
const (
	DenseW  = feat.DenseWidthPod
	Groups  = feat.EntityGroupsPod
	RawW    = feat.EntityRawWidthPod
	groupW  = 1 + 2*RawW
	Width   = DenseW + Groups*groupW
	sumGain = 0.25
)

// Registry opens the card corpus the encoder reads card facts from. It is
// opened once per process.
func Registry() (*cards.Registry, error) {
	dir, err := cardset.CorpusDir()
	if err != nil {
		return nil, err
	}
	return cards.SharedCorpus(dir)
}

// Featurizer encodes positions of one table. One per goroutine.
type Featurizer struct {
	reg  *cards.Registry
	tbl  *kop.Table
	enc  *feat.Encoder
	pool *feat.Pool
	out  [Width]float32
}

// NewFeaturizer returns a featurizer reading card facts from reg.
func NewFeaturizer(reg *cards.Registry) *Featurizer { return &Featurizer{reg: reg} }

// SetTable prepares the featurizer for games on tbl. It is a no-op for the
// table it already holds.
func (f *Featurizer) SetTable(tbl *kop.Table) error {
	if f.tbl == tbl {
		return nil
	}
	pool, err := oracle.FeatPool(f.reg, tbl, nil)
	if err != nil {
		return err
	}
	f.tbl, f.pool = tbl, pool
	if f.enc == nil {
		f.enc = feat.NewEncoder(pool)
		f.enc.SetPod(true)
	} else {
		f.enc.SetPool(pool)
	}
	return nil
}

// Encode writes the features of s at its pending decision d as seat sees it.
// The slice is the featurizer's own and valid until the next call.
func (f *Featurizer) Encode(s *dirac.State, d *dirac.Decision, seat uint8) []float32 {
	st := f.enc.EncodeStateFor(s, d, seat)
	out := f.out[:]
	clear(out)
	copy(out, st.Dense[:])
	for i := range st.Cards {
		c := &st.Cards[i]
		g := int(c.Group)
		if g >= Groups {
			continue
		}
		b := out[DenseW+g*groupW : DenseW+(g+1)*groupW]
		b[0] += 0.125
		sum, mx := b[1:1+RawW], b[1+RawW:]
		for j, v := range c.Raw {
			sum[j] += sumGain * v
			if v > mx[j] {
				mx[j] = v
			}
		}
	}
	return out
}
