// Package study watches pilots play: the same drawn tables and seeds as the
// arena, with several pilots taking turns in the candidate's seat, and a
// record of how each played and how each game went. With a probe it also
// asks other pilots what they would have done at the candidate's decisions.
package study

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"slices"
	"sync"

	"github.com/Lzeutschler/commander_ai/internal/learn"
	"github.com/Lzeutschler/commander_ai/internal/pod"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/lab"
	"github.com/adams-shaun/gorge/dirac/search"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Pilot is a named policy maker: a fresh policy per game.
type Pilot struct {
	Name string
	Make func() lab.Policy
}

// Probe asks other pilots for their answer at some of one pilot's decisions.
type Probe struct {
	Of    string  // the pilot whose games are probed
	Alts  []Pilot // asked at its decisions, from a copy of the game
	Every int     // probe every Every-th decision that offers more than one option
	// Net, when set, values the probed positions and the turn snapshots of
	// every pilot's games for the candidate's seat, next to the engine's
	// hand-made leaf.
	Net *learn.ValueNet
	Reg *cards.Registry
}

// Options says what to watch.
type Options struct {
	Split     string // the pool split the candidate's deck is drawn from
	FirstSeed uint64
	Games     int
	Pilots    []Pilot
	Probe     *Probe
	Workers   int
	MaxSteps  int
	Mulligans int
	Allow     []string
	Out       string // JSON lines, one per game and pilot
	Only      []int  // when set, these games alone
}

// Game is one pilot's game at one table, from the candidate's seat.
type Game struct {
	Game      int      `json:"game"`
	Pilot     string   `json:"pilot"`
	Seat      int      `json:"seat"`
	Decks     []string `json:"decks"`
	Outcome   string   `json:"outcome"` // won, drawn, stalled, truncated, failed
	Winner    int      `json:"winner"`
	Turns     int      `json:"turns"`
	Decisions int      `json:"decisions"`
	Stall     string   `json:"stall,omitempty"`
	Err       string   `json:"err,omitempty"` // a failed game's panic or bad answer
	Exits     [4]Exit  `json:"exits"`
	Play      Play     `json:"play"`
	Snaps     []Snap   `json:"snaps"`
	Probes    []Probed `json:"probes,omitempty"`
}

// Exit is when and how a seat left the game. Turn 0: it did not.
type Exit struct {
	Turn   int    `json:"turn"`
	Cause  string `json:"cause,omitempty"` // life, commander, deck, other
	Active int    `json:"active"`          // whose turn it was
}

// Play counts what the candidate's seat did.
type Play struct {
	Mulligans  int `json:"mulligans"`
	Decisions  int `json:"decisions"`
	Choices    int `json:"choices"` // decisions with more than one option
	Lands      int `json:"lands"`
	Casts      int `json:"casts"`
	Creatures  int `json:"creatures"`   // creature spells cast
	Permanents int `json:"permanents"`  // other permanent spells cast
	Spells     int `json:"spells"`      // instants and sorceries cast
	Commander  int `json:"commander"`   // casts from the command zone
	Activates  int `json:"activations"` // abilities activated
	Passes     int `json:"passes"`      // priority passed
	PassCast   int `json:"pass_cast"`   // priority passed while a cast was offered
	// Attacks: attackers declared, the turns with at least one, and per
	// attacker the target's rank among the opponents still in by life and by
	// creature power (0: the highest), or a permanent.
	Attackers  int    `json:"attackers"`
	AttackTurn int    `json:"attack_turns"`
	ByLife     [3]int `json:"by_life"`
	ByPower    [3]int `json:"by_power"`
	AtPerm     int    `json:"at_permanent"`
	Offered    int    `json:"attack_offered"` // attackers it could have declared, summed over its attack steps
	Blocks     int    `json:"blocks"`
	BlockSteps int    `json:"block_steps"` // blocker steps it was asked in
}

// Snap is the table at the first decision of a turn.
type Snap struct {
	Turn      int     `json:"turn"`
	Active    int     `json:"active"`
	Life      [4]int  `json:"life"`
	Lands     [4]int  `json:"lands"` // on the battlefield
	Hand      [4]int  `json:"hand"`
	Library   [4]int  `json:"library"`
	Creatures [4]int  `json:"creatures"`
	Power     [4]int  `json:"power"`
	CmdDmg    [4]int  `json:"cmd_dmg"` // the most a seat was dealt by one commander
	Lost      [4]bool `json:"lost"`
	// NetV and LeafV value the position for the candidate's seat.
	NetV  float32 `json:"net_v,omitempty"`
	LeafV float32 `json:"leaf_v,omitempty"`
}

// Probed is one probed decision: what the candidate played and what each
// alternative pilot would have played.
type Probed struct {
	Turn  int      `json:"turn"`
	Kind  string   `json:"kind"`
	N     int      `json:"n"`
	Picks []string `json:"picks"` // the candidate's, then each Alt's, as kinds
	Same  []bool   `json:"same"`  // per Alt: the very same action as the candidate
	Desc  []string `json:"desc"`  // readable, same order as Picks
	NetV  float32  `json:"net_v,omitempty"`
	LeafV float32  `json:"leaf_v,omitempty"`
}

// Run plays every game with every pilot and writes the records.
func Run(root string, opts Options) error {
	pool, err := pod.LoadPool(root, opts.Allow)
	if err != nil {
		return err
	}
	cands, err := pool.In(opts.Split)
	if err != nil {
		return err
	}
	workers := max(1, min(opts.Workers, opts.Games))
	prev := runtime.GOMAXPROCS(workers)
	defer runtime.GOMAXPROCS(prev)
	out := make([][]Game, opts.Games)
	errs := make([]error, workers)
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var ev *learn.Evaluator
			if p := opts.Probe; p != nil && p.Net != nil {
				ev = learn.NewEvaluator(p.Net, p.Reg)
			}
			for g := range next {
				games, err := playTable(pool, cands, opts, g, ev)
				if err != nil {
					errs[w] = err
					continue
				}
				out[g] = games
			}
		}()
	}
	for g := 0; g < opts.Games; g++ {
		if opts.Only == nil || slices.Contains(opts.Only, g) {
			next <- g
		}
	}
	close(next)
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	f, err := os.Create(opts.Out)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, games := range out { // a game left out by Only has none
		for i := range games {
			if err := enc.Encode(&games[i]); err != nil {
				f.Close()
				return err
			}
		}
	}
	return f.Close()
}

// playTable draws game g's table as the arena does and plays it once per pilot.
func playTable(pool *pod.Pool, cands []int, opts Options, g int, ev *learn.Evaluator) ([]Game, error) {
	rng := rand.New(rand.NewPCG(opts.FirstSeed, uint64(g)))
	var su *lab.Setup
	var table []int
	var seat int
	for attempt := 0; su == nil && attempt < 200; attempt++ {
		table, seat = pod.DrawTable(rng, pool, cands)
		decks := make([]lab.Deck, len(table))
		for i, d := range table {
			decks[i] = pool.Decks[d].Deck
		}
		s, err := lab.Open(lab.Config{Set: pod.PoolSet, Decks: decks, Commander: true, Full: true,
			Mulligans: opts.Mulligans, Allow: opts.Allow})
		if err == nil {
			su = s
		}
	}
	if su == nil {
		return nil, fmt.Errorf("study: game %d found no table the kernel deals in 200 draws", g)
	}
	defer su.Close()
	if ev != nil {
		if err := ev.SetTable(su.Table()); err != nil {
			return nil, err
		}
	}
	var games []Game
	for _, p := range opts.Pilots {
		w := &watch{tbl: su.Table(), seat: seat, ev: ev}
		w.rec = Game{Game: g, Pilot: p.Name, Seat: seat, Decks: su.DeckNames(), Winner: -1}
		if pr := opts.Probe; pr != nil && pr.Of == p.Name {
			w.probe = pr
			for _, a := range pr.Alts {
				w.alts = append(w.alts, a.Make())
			}
		}
		pols := make([]lab.Policy, len(table))
		for i := range pols {
			if i == seat {
				pols[i] = &watched{Policy: p.Make(), w: w, cand: true}
			} else {
				pols[i] = &watched{Policy: lab.HBot(), w: w}
			}
		}
		game := su.Game(opts.FirstSeed + uint64(g))
		res := lab.Play(game, pols, opts.MaxSteps)
		w.finish(game.State(), res)
		games = append(games, w.rec)
	}
	return games, nil
}

// watch follows one game.
type watch struct {
	tbl   *kop.Table
	seat  int
	ev    *learn.Evaluator
	game  *lab.Game
	rec   Game
	turn  int
	proj  dirac.Projection
	lost  [4]bool
	steps int // the candidate's decisions with more than one option
	probe *Probe
	alts  []lab.Policy
	// attacked: the candidate declared an attacker this turn
	attacked int
	blocked  int
}

type watched struct {
	lab.Policy
	w    *watch
	cand bool
}

func (p *watched) Start(g *lab.Game, seat int) {
	p.w.game = g
	if st, ok := p.Policy.(lab.Starter); ok {
		st.Start(g, seat)
	}
}

func (p *watched) Pick(s *dirac.State, d *dirac.Decision) dirac.Action {
	p.w.see(s, d)
	if !p.cand {
		return p.Policy.Pick(s, d)
	}
	var pr *Probed
	if p.w.probe != nil && d.N > 1 {
		p.w.steps++
		if p.w.steps%p.w.probe.Every == 0 {
			pr = p.w.ask(s, d)
		}
	}
	a := p.Policy.Pick(s, d)
	if pr != nil {
		pr.Picks[0], pr.Desc[0] = kindName(a.Kind()), p.w.game.Describe(a)
		for i := range p.w.alts {
			pr.Same[i] = pr.Desc[i+1] == pr.Desc[0] && pr.Picks[i+1] == pr.Picks[0]
		}
		p.w.rec.Probes = append(p.w.rec.Probes, *pr)
	}
	p.w.picked(s, d, a)
	return a
}

// see notes the table at every decision: a new turn's snapshot and seats
// that left the game since the last decision.
func (w *watch) see(s *dirac.State, d *dirac.Decision) {
	if t := s.Turn(); t != w.turn {
		w.turn = t
		w.rec.Snaps = append(w.rec.Snaps, w.snap(s, d))
	}
	w.exits(s)
}

func (w *watch) exits(s *dirac.State) {
	for p := uint8(0); p < s.NumSeats() && p < 4; p++ {
		if w.lost[p] || !s.FeatLost(p) {
			continue
		}
		w.lost[p] = true
		s.Project(&w.proj)
		cause := "other"
		cmd := 0
		for q := 0; q < 4; q++ {
			cmd = max(cmd, int(w.proj.CmdDmg[p][q]))
		}
		switch {
		case cmd >= 21:
			cause = "commander"
		case s.Life(int(p)) <= 0:
			cause = "life"
		case w.proj.LibN[p] == 0:
			cause = "deck"
		}
		w.rec.Exits[p] = Exit{Turn: s.Turn(), Cause: cause, Active: int(s.ActivePlayer())}
	}
}

func (w *watch) snap(s *dirac.State, d *dirac.Decision) Snap {
	s.Project(&w.proj)
	pj := &w.proj
	sn := Snap{Turn: s.Turn(), Active: int(s.ActivePlayer())}
	for p := 0; p < 4 && p < int(s.NumSeats()); p++ {
		sn.Life[p], sn.Hand[p], sn.Library[p] = int(pj.Life[p]), int(pj.HandN[p]), int(pj.LibN[p])
		sn.Lost[p] = s.FeatLost(uint8(p))
		for q := 0; q < 4; q++ {
			sn.CmdDmg[p] = max(sn.CmdDmg[p], int(pj.CmdDmg[p][q]))
		}
	}
	for i := 0; i < int(pj.NObj); i++ {
		o := &pj.Objs[i]
		if o.Zone != kop.ZoneBattlefield || o.Ctrl >= 4 {
			continue
		}
		ty := w.tbl.Cards[o.Card].Types
		if ty&kop.TLand != 0 {
			sn.Lands[o.Ctrl]++
		}
		if ty&kop.TCreature == 0 {
			continue
		}
		sn.Creatures[o.Ctrl]++
		sn.Power[o.Ctrl] += max(0, int(o.Power))
	}
	if w.ev != nil && !s.FeatLost(uint8(w.seat)) {
		sn.NetV = w.ev.Value(s, d, uint8(w.seat))
		sn.LeafV = search.HeuristicLeaf{}.Value(s, d, uint8(w.seat))
	}
	return sn
}

// picked counts the candidate's answer a at d.
func (w *watch) picked(s *dirac.State, d *dirac.Decision, a dirac.Action) {
	pl := &w.rec.Play
	pl.Decisions++
	if d.N > 1 {
		pl.Choices++
	}
	switch d.Kind {
	case dirac.DkAttackers:
		if a.Kind() != dirac.AkAttack {
			return
		}
		if w.attacked != s.Turn() {
			w.attacked = s.Turn()
			pl.AttackTurn++
			pl.Offered += attackers(d)
		}
		pl.Attackers++
		t := a.Target()
		if t < dirac.PlayerTarget0 || t >= dirac.PlayerTarget0+dirac.MaxSeats {
			pl.AtPerm++
			return
		}
		def := int(t - dirac.PlayerTarget0)
		s.Project(&w.proj)
		power := w.power()
		var byLife, byPower int
		for q := 0; q < 4 && q < int(s.NumSeats()); q++ {
			if q == w.seat || q == def || s.FeatLost(uint8(q)) {
				continue
			}
			if w.proj.Life[q] > w.proj.Life[def] {
				byLife++
			}
			if power[q] > power[def] {
				byPower++
			}
		}
		pl.ByLife[min(byLife, 2)]++
		pl.ByPower[min(byPower, 2)]++
		return
	case dirac.DkBlockers:
		if w.blocked != s.Turn() {
			w.blocked = s.Turn()
			pl.BlockSteps++
		}
		if a.Kind() == dirac.AkBlock {
			pl.Blocks++
		}
		return
	case dirac.DkMulligan:
		if a.Kind() == dirac.AkMulligan {
			pl.Mulligans++
		}
		return
	case dirac.DkPriority:
	default:
		return
	}
	switch a.Kind() {
	case dirac.AkPass:
		pl.Passes++
		for _, o := range d.Options[:d.N] {
			if isCast(o.Kind()) {
				pl.PassCast++
				break
			}
		}
	case dirac.AkPlayLand:
		pl.Lands++
	case dirac.AkActivate:
		pl.Activates++
	default:
		if !isCast(a.Kind()) {
			return
		}
		pl.Casts++
		o := dirac.Obj(a.Source())
		if int(o) < s.NumObjs() {
			s.Project(&w.proj)
			if w.proj.Objs[o].Zone == kop.ZoneCommand {
				pl.Commander++
			}
			ty := w.tbl.Cards[s.ObjCard(o)].Types
			switch {
			case ty&kop.TCreature != 0:
				pl.Creatures++
			case ty&(kop.TInstant|kop.TSorcery) != 0:
				pl.Spells++
			default:
				pl.Permanents++
			}
		}
	}
}

// attackers counts the distinct creatures an attackers decision offers.
func attackers(d *dirac.Decision) int {
	seen := map[dirac.TCode]bool{}
	for _, o := range d.Options[:d.N] {
		if o.Kind() == dirac.AkAttack {
			seen[o.Source()] = true
		}
	}
	return len(seen)
}

// power sums each seat's creature power in w.proj.
func (w *watch) power() [4]int {
	var p [4]int
	for i := 0; i < int(w.proj.NObj); i++ {
		o := &w.proj.Objs[i]
		if o.Zone == kop.ZoneBattlefield && o.Ctrl < 4 && w.tbl.Cards[o.Card].Types&kop.TCreature != 0 {
			p[o.Ctrl] += max(0, int(o.Power))
		}
	}
	return p
}

// ask puts the decision to each alternative pilot on a copy of the game.
func (w *watch) ask(s *dirac.State, d *dirac.Decision) *Probed {
	pr := &Probed{Turn: s.Turn(), Kind: decisionName(d.Kind), N: int(d.N),
		Picks: make([]string, 1+len(w.alts)), Desc: make([]string, 1+len(w.alts)), Same: make([]bool, len(w.alts))}
	for i, alt := range w.alts {
		fork := w.game.Fork()
		if st, ok := alt.(lab.Starter); ok {
			st.Start(fork, w.seat)
		}
		a := alt.Pick(fork.State(), fork.Decision())
		pr.Picks[i+1], pr.Desc[i+1] = kindName(a.Kind()), fork.Describe(a)
		fork.Close()
	}
	if w.ev != nil {
		pr.NetV = w.ev.Value(s, d, uint8(w.seat))
		pr.LeafV = search.HeuristicLeaf{}.Value(s, d, uint8(w.seat))
	}
	return pr
}

// finish notes the result and the seats that left with the last action.
func (w *watch) finish(s *dirac.State, res lab.Result) {
	w.exits(s)
	w.rec.Outcome = outcomeName(res.Outcome)
	w.rec.Winner, w.rec.Turns, w.rec.Decisions, w.rec.Stall, w.rec.Err = res.Winner, res.Turns, res.Decisions, res.Stall, res.Err
}

func isCast(k dirac.ActionKind) bool {
	switch k {
	case dirac.AkCast, dirac.AkCastCasualty, dirac.AkSneak, dirac.AkBestow, dirac.AkWebSling, dirac.AkMayFlash, dirac.AkCastConspire:
		return true
	}
	return false
}

func kindName(k dirac.ActionKind) string {
	switch {
	case k == dirac.AkPass:
		return "pass"
	case k == dirac.AkPlayLand:
		return "land"
	case isCast(k):
		return "cast"
	case k == dirac.AkActivate:
		return "activate"
	case k == dirac.AkAttack:
		return "attack"
	case k == dirac.AkBlock:
		return "block"
	case k == dirac.AkDone:
		return "done"
	case k == dirac.AkTarget:
		return "target"
	case k == dirac.AkMode:
		return "mode"
	case k == dirac.AkChoose:
		return "choose"
	case k == dirac.AkKeep:
		return "keep"
	case k == dirac.AkMulligan:
		return "mulligan"
	case k == dirac.AkOrder:
		return "order"
	}
	return "other"
}

func decisionName(k dirac.DecisionKind) string {
	return [...]string{"none", "priority", "attackers", "blockers", "target", "modes", "choose", "trigger_order", "mulligan"}[min(int(k), 8)]
}

func outcomeName(o lab.Outcome) string {
	return [...]string{"running", "won", "drawn", "stalled", "truncated", "failed"}[min(int(o), 5)]
}
