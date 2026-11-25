package study

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lzeutschler/commander_ai/internal/pod"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// A study plays the arena's games: the same tables and seeds, so a pilot's
// outcomes are the arena's game by game. Its records do not depend on the
// worker count, and a probe asks the alternatives without changing the game.
func TestStudyPlaysTheArenasGames(t *testing.T) {
	root, err := pod.FindRoot()
	if err != nil {
		t.Skip(err)
	}
	if err := pod.UseCorpus(root); err != nil {
		t.Skip(err)
	}
	if _, err := cardset.ByName(pod.PoolSet); err != nil {
		t.Skip("no card tables (run `make setup`): ", err)
	}
	const games, seed = 24, 7
	def := pod.DefaultOptions(pod.Table{})
	random := func() lab.Policy { return lab.Random(1) }
	run := func(workers int, probe *Probe) []Game {
		out := filepath.Join(t.TempDir(), "s.jsonl")
		err := Run(root, Options{Split: "eval", FirstSeed: seed, Games: games, Workers: workers, MaxSteps: def.MaxSteps,
			Mulligans: def.Mulligans, Out: out, Probe: probe,
			Pilots: []Pilot{{"hbot", lab.HBot}, {"random", random}}})
		if err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		var gs []Game
		sc := bufio.NewScanner(f)
		sc.Buffer(nil, 1<<26)
		for sc.Scan() {
			var g Game
			if err := json.Unmarshal(sc.Bytes(), &g); err != nil {
				t.Fatal(err)
			}
			gs = append(gs, g)
		}
		return gs
	}
	one := run(1, nil)
	if len(one) != 2*games {
		t.Fatalf("%d records, want %d", len(one), 2*games)
	}
	a, _ := json.Marshal(one)
	b, _ := json.Marshal(run(4, nil))
	if string(a) != string(b) {
		t.Error("the records differ between 1 and 4 workers")
	}

	rep, _, err := pod.Arena(root, pod.ArenaOptions{Pool: "eval", Candidate: "random", FirstSeed: seed, Games: games,
		Workers: 4, MaxSteps: def.MaxSteps, Mulligans: def.Mulligans})
	if err != nil {
		t.Fatal(err)
	}
	var mine []byte
	for _, g := range one {
		if g.Pilot != "random" {
			continue
		}
		c := byte('L')
		switch {
		case g.Outcome == "won" && g.Winner == g.Seat:
			c = 'W'
		case g.Outcome != "won" && g.Outcome != "drawn":
			c = 'U'
		}
		mine = append(mine, c)
		if len(g.Snaps) == 0 || g.Play.Decisions == 0 {
			t.Errorf("game %d: %d snapshots, %d decisions", g.Game, len(g.Snaps), g.Play.Decisions)
		}
	}
	if string(mine) != rep.Outcomes {
		t.Errorf("study outcomes %s, arena %s", mine, rep.Outcomes)
	}

	probed := run(4, &Probe{Of: "random", Every: 2, Alts: []Pilot{{"self", random}, {"hbot", lab.HBot}}})
	n := 0
	for i, g := range probed {
		if g.Outcome != one[i].Outcome || g.Winner != one[i].Winner || g.Decisions != one[i].Decisions {
			t.Fatalf("game %d (%s): probing changed the game", g.Game, g.Pilot)
		}
		for _, p := range g.Probes {
			n++
			if len(p.Picks) != 3 || len(p.Same) != 2 || p.Picks[0] == "" {
				t.Fatalf("game %d: probe %+v", g.Game, p)
			}
		}
	}
	if n == 0 {
		t.Error("no decision was probed")
	}
}
