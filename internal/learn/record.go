package learn

import (
	"encoding/binary"
	"encoding/json"
	"errors"
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
	Split string // the pool split every deck of a table is drawn from
	// Tables are the line-ups games are played with; each game draws one, by
	// weight.
	Tables    []RecordTable
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

// RecordTable is one line-up: one pilot for all four seats, or four that are
// dealt to the seats afresh each game. Keep, when set, says per pilot whether
// the seats it plays are recorded; nil records every seat. Weight (at least
// 1) is the line-up's share among RecordOptions.Tables.
type RecordTable struct {
	Pilots []func() lab.Policy
	Keep   []bool
	Weight int
}

// RecordMeta describes a recorded data set (meta.json).
type RecordMeta struct {
	Width      int            `json:"width"`
	Split      string         `json:"split"`
	Pilot      string         `json:"pilot"`
	Keep       [][]bool       `json:"keep,omitempty"` // per line-up, per pilot
	TableGames []int          `json:"table_games"`    // finished games per line-up
	MetaCols   int            `json:"meta_cols"`
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
	// Complete is false in the meta.json written before the first game: a
	// run the engine brought down leaves that one, and its shards hold the
	// games finished until then. Readers count records from the files.
	Complete bool `json:"complete"`
}

// Record plays opts.Games games at tables drawn from one split of the deck
// pool and writes sampled positions: for each sampled decision one record per
// seat still in the game, the features as that seat sees the position, and
// whether that seat went on to win. Games that do not finish are left out.
//
// Shard k holds <k>.x.f32 (records × Width float32), <k>.y.u8 (1 for a win)
// and <k>.m.i32 (game, seat, turn, decision number, and who played the seat:
// 4 * the line-up's index + the pilot's place in it, per record).
func Record(root string, opts RecordOptions) (RecordMeta, error) {
	var meta RecordMeta
	if len(opts.Tables) == 0 {
		return meta, errors.New("record: no line-up")
	}
	weight := 0
	for _, t := range opts.Tables {
		if n := len(t.Pilots); n != 1 && n != 4 {
			return meta, fmt.Errorf("record: %d pilots in a line-up, want one or four", n)
		}
		if t.Keep != nil && len(t.Keep) != len(t.Pilots) {
			return meta, fmt.Errorf("record: keep names %d pilots of %d", len(t.Keep), len(t.Pilots))
		}
		if t.Weight < 1 {
			return meta, errors.New("record: a line-up's weight is at least 1")
		}
		weight += t.Weight
	}
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
	meta.Width, meta.Split, meta.Pilot, meta.FirstSeed = Width, opts.Split, opts.PilotName, opts.FirstSeed
	meta.Games, meta.Every, meta.Provenance, meta.Pool = opts.Games, opts.Every, prov, pool.Info
	meta.MetaCols, meta.TableGames = metaCols, make([]int, len(opts.Tables))
	meta.Format = "per shard: .x.f32 records*width float32 LE, .y.u8 win, .m.i32 game seat turn decision pilot"
	for _, t := range opts.Tables {
		if t.Keep != nil {
			meta.Keep = make([][]bool, len(opts.Tables))
		}
	}
	for i, t := range opts.Tables {
		if meta.Keep != nil {
			meta.Keep[i] = t.Keep
		}
	}
	for w := 0; w < workers; w++ {
		meta.Shards = append(meta.Shards, fmt.Sprintf("%03d", w))
	}
	writeMeta := func() error {
		raw, _ := json.MarshalIndent(meta, "", "  ")
		return os.WriteFile(filepath.Join(opts.Out, "meta.json"), append(raw, '\n'), 0o644)
	}
	if err := writeMeta(); err != nil {
		return meta, err
	}
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)

	type tally struct {
		finished, records, positive int
		tables                      []int
	}
	tallies := make([]tally, workers)
	errs := make([]error, workers)
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
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
			tallies[w].tables = make([]int, len(opts.Tables))
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
				ti := 0
				if len(opts.Tables) > 1 {
					pick := rng.IntN(weight)
					for pick >= opts.Tables[ti].Weight {
						pick -= opts.Tables[ti].Weight
						ti++
					}
				}
				lineup := &opts.Tables[ti]
				var order []int
				if len(lineup.Pilots) == 4 {
					order = rng.Perm(4)
				}
				for i := range pols {
					role := 0
					if order != nil {
						role = order[i]
					}
					rec.role[i], rec.keep[i] = int32(4*ti+role), lineup.Keep == nil || lineup.Keep[role]
					pols[i] = &recording{Policy: lineup.Pilots[role](), rec: rec}
				}
				res := lab.Play(su.Game(opts.FirstSeed+uint64(g)), pols, opts.MaxSteps)
				su.Close()
				if res.Outcome != lab.Won {
					continue
				}
				tallies[w].finished++
				tallies[w].tables[ti]++
				for i := range rec.seat {
					win := byte(0)
					if int(rec.seat[i]) == res.Winner {
						win = 1
						tallies[w].positive++
					}
					sh.write(rec.x[i*Width:(i+1)*Width], win, [metaCols]int32{int32(g), int32(rec.seat[i]), rec.turn[i], rec.step[i], rec.role[rec.seat[i]]})
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
	for _, t := range tallies {
		meta.Finished += t.finished
		meta.Records += t.records
		meta.Positive += t.positive
		for i, n := range t.tables {
			meta.TableGames[i] += n
		}
	}
	meta.Complete = true
	return meta, writeMeta()
}

// metaCols is the number of int32 a record carries in .m.i32.
const metaCols = 5

// recorder collects one game's sampled positions.
type recorder struct {
	f     *Featurizer
	every int
	game  int
	n     int // decisions seen
	phase int
	role  [4]int32 // per seat: which pilot plays it
	keep  [4]bool  // per seat: record it
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
		if s.FeatLost(p) || !r.keep[p] {
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

func (sh *shard) write(x []float32, win byte, meta [metaCols]int32) {
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
