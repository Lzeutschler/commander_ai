package learn

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/Lzeutschler/commander_ai/internal/pod"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// RecordOptions says which games to record positions from.
type RecordOptions struct {
	Split     string // the pool split every deck of a table is drawn from
	Pilot     func() lab.Policy
	PilotName string
	FirstSeed uint64
	Games     int
	Every     int // sample every Every-th decision of a game, from a random phase
	Workers   int
	MaxSteps  int
	Mulligans int
	Allow     []string
	Out       string // directory for the shards and meta.json
}

// RecordMeta describes a recorded data set (meta.json).
type RecordMeta struct {
	Width      int            `json:"width"`
	Split      string         `json:"split"`
	Pilot      string         `json:"pilot"`
	FirstSeed  uint64         `json:"first_seed"`
	Games      int            `json:"games"`
	Finished   int            `json:"finished"`
	Every      int            `json:"every"`
	Records    int            `json:"records"`
	Positive   int            `json:"positive"`
	Shards     []string       `json:"shards"`
	Provenance pod.Provenance `json:"provenance"`
	Pool       pod.PoolInfo   `json:"pool"`
	Format     string         `json:"format"`
}

// Record plays opts.Games games at tables drawn from one split of the deck
// pool and writes sampled positions: for each sampled decision one record per
// seat still in the game, the features as that seat sees the position, and
// whether that seat went on to win. Games that do not finish are left out.
//
// Shard k holds <k>.x.f32 (records × Width float32), <k>.y.u8 (1 for a win)
// and <k>.m.i32 (game, seat, turn, decision number per record).
func Record(root string, opts RecordOptions) (RecordMeta, error) {
	var meta RecordMeta
	prov, err := pod.ReadProvenance(root)
	if err != nil {
		return meta, err
	}
	pool, err := pod.LoadPool(root, opts.Allow)
	if err != nil {
		return meta, err
	}
	decks, err := pool.In(opts.Split)
	if err != nil {
		return meta, err
	}
	if len(decks) < 4 {
		return meta, fmt.Errorf("split %q has %d decks", opts.Split, len(decks))
	}
	reg, err := Registry()
	if err != nil {
		return meta, err
	}
	if err := os.MkdirAll(opts.Out, 0o755); err != nil {
		return meta, err
	}
	workers := max(1, min(opts.Workers, opts.Games))
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)

	type tally struct{ finished, records, positive int }
	tallies := make([]tally, workers)
	errs := make([]error, workers)
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		meta.Shards = append(meta.Shards, fmt.Sprintf("%03d", w))
		wg.Add(1)
		go func() {
			defer wg.Done()
			sh, err := newShard(filepath.Join(opts.Out, fmt.Sprintf("%03d", w)))
			if err != nil {
				errs[w] = err
				for range next {
				}
				return
			}
			f := NewFeaturizer(reg)
			rec := &recorder{f: f, every: opts.Every}
			for g := range next {
				rng := rand.New(rand.NewPCG(opts.FirstSeed, uint64(g)))
				var su *lab.Setup
				for attempt := 0; su == nil && attempt < 200; attempt++ {
					table := make([]lab.Deck, 4)
					for i, d := range rng.Perm(len(decks))[:4] {
						table[i] = pool.Decks[decks[d]].Deck
					}
					s, err := lab.Open(lab.Config{Set: pod.PoolSet, Decks: table, Commander: true, Full: true,
						Mulligans: opts.Mulligans, Allow: opts.Allow})
					if err == nil {
						su = s
					}
				}
				if su == nil {
					errs[w] = fmt.Errorf("game %d: no table the kernel deals in 200 draws", g)
					continue
				}
				if err := f.SetTable(su.Table()); err != nil {
					errs[w] = err
					su.Close()
					continue
				}
				rec.reset(g, rng.IntN(opts.Every))
				pols := make([]lab.Policy, 4)
				for i := range pols {
					pols[i] = &recording{Policy: opts.Pilot(), rec: rec}
				}
				res := lab.Play(su.Game(opts.FirstSeed+uint64(g)), pols, opts.MaxSteps)
				su.Close()
				if res.Outcome != lab.Won {
					continue
				}
				tallies[w].finished++
				for i := range rec.seat {
					win := byte(0)
					if int(rec.seat[i]) == res.Winner {
						win = 1
						tallies[w].positive++
					}
					sh.write(rec.x[i*Width:(i+1)*Width], win, [4]int32{int32(g), int32(rec.seat[i]), rec.turn[i], rec.step[i]})
					tallies[w].records++
				}
			}
			if err := sh.close(); err != nil && errs[w] == nil {
				errs[w] = err
			}
		}()
	}
	for g := 0; g < opts.Games; g++ {
		next <- g
	}
	close(next)
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return meta, e
		}
	}
	meta.Width, meta.Split, meta.Pilot, meta.FirstSeed = Width, opts.Split, opts.PilotName, opts.FirstSeed
	meta.Games, meta.Every, meta.Provenance, meta.Pool = opts.Games, opts.Every, prov, pool.Info
	meta.Format = "per shard: .x.f32 records*width float32 LE, .y.u8 win, .m.i32 game seat turn decision"
	for _, t := range tallies {
		meta.Finished += t.finished
		meta.Records += t.records
		meta.Positive += t.positive
	}
	raw, _ := json.MarshalIndent(meta, "", "  ")
	return meta, os.WriteFile(filepath.Join(opts.Out, "meta.json"), append(raw, '\n'), 0o644)
}

// recorder collects one game's sampled positions.
type recorder struct {
	f     *Featurizer
	every int
	game  int
	n     int // decisions seen
	phase int
	x     []float32
	seat  []uint8
	turn  []int32
	step  []int32
}

func (r *recorder) reset(game, phase int) {
	r.game, r.n, r.phase = game, 0, phase
	r.x, r.seat, r.turn, r.step = r.x[:0], r.seat[:0], r.turn[:0], r.step[:0]
}

func (r *recorder) see(s *dirac.State, d *dirac.Decision) {
	r.n++
	if r.n%r.every != r.phase {
		return
	}
	for p := uint8(0); p < s.NumSeats(); p++ {
		if s.FeatLost(p) {
			continue
		}
		r.x = append(r.x, r.f.Encode(s, d, p)...)
		r.seat = append(r.seat, p)
		r.turn = append(r.turn, int32(s.Turn()))
		r.step = append(r.step, int32(r.n))
	}
}

// recording passes a seat's decisions to its pilot and shows each to the
// game's recorder first.
type recording struct {
	lab.Policy
	rec *recorder
}

func (p *recording) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	p.rec.see(s, d)
	return p.Policy.Pick(s, d)
}

func (p *recording) Start(g *lab.Game, seat int) {
	if st, ok := p.Policy.(lab.Starter); ok {
		st.Start(g, seat)
	}
}

type shard struct {
	x, y, m *os.File
	buf     []byte
}

func newShard(base string) (*shard, error) {
	sh := &shard{}
	var err error
	for _, f := range []struct {
		dst **os.File
		ext string
	}{{&sh.x, ".x.f32"}, {&sh.y, ".y.u8"}, {&sh.m, ".m.i32"}} {
		if *f.dst, err = os.Create(base + f.ext); err != nil {
			return nil, err
		}
	}
	return sh, nil
}

func (sh *shard) write(x []float32, win byte, meta [4]int32) {
	sh.buf = sh.buf[:0]
	for _, v := range x {
		sh.buf = binary.LittleEndian.AppendUint32(sh.buf, math.Float32bits(v))
	}
	sh.x.Write(sh.buf)
	sh.y.Write([]byte{win})
	sh.buf = sh.buf[:0]
	for _, v := range meta {
		sh.buf = binary.LittleEndian.AppendUint32(sh.buf, uint32(v))
	}
	sh.m.Write(sh.buf)
}

func (sh *shard) close() error {
	var first error
	for _, f := range []*os.File{sh.x, sh.y, sh.m} {
		if err := f.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
