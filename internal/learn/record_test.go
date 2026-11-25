package learn

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/Lzeutschler/commander_ai/internal/pod"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/lab"
)

// Games are dealt to weighted line-ups, only marked pilots' seats are
// recorded, and what is recorded does not depend on the worker count.
func TestRecordLineUps(t *testing.T) {
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
	explorer := func() lab.Policy { return &Exploring{Policy: lab.HBot(), Eps: 0.2} }
	tables := []RecordTable{
		{Pilots: []func() lab.Policy{lab.HBot}, Weight: 1},
		{Pilots: []func() lab.Policy{explorer, lab.HBot, lab.HBot, lab.HBot}, Keep: []bool{true, false, false, false}, Weight: 2},
	}
	def := pod.DefaultOptions(pod.Table{})
	run := func(workers int) (RecordMeta, map[int32]int) {
		out := t.TempDir()
		meta, err := Record(root, RecordOptions{Split: "eval", Tables: tables, PilotName: "test", FirstSeed: 31, Games: 60,
			Every: 20, Workers: workers, MaxSteps: def.MaxSteps, Mulligans: def.Mulligans, Out: out})
		if err != nil {
			t.Fatal(err)
		}
		roles := map[int32]int{}
		for _, sh := range meta.Shards {
			raw, err := os.ReadFile(filepath.Join(out, sh+".m.i32"))
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i+4*metaCols <= len(raw); i += 4 * metaCols {
				roles[int32(binary.LittleEndian.Uint32(raw[i+16:]))]++
			}
		}
		return meta, roles
	}
	one, roles := run(1)
	four, roles4 := run(4)
	if one.Records != four.Records || one.Positive != four.Positive || one.Finished != four.Finished {
		t.Errorf("1 worker: %d records, %d wins, %d games; 4 workers: %d, %d, %d",
			one.Records, one.Positive, one.Finished, four.Records, four.Positive, four.Finished)
	}
	if one.TableGames[0] == 0 || one.TableGames[1] <= one.TableGames[0] || one.TableGames[0]+one.TableGames[1] != one.Finished {
		t.Errorf("games per line-up %v of %d finished: want both, the second about twice as often", one.TableGames, one.Finished)
	}
	// line-up 0 records every seat as pilot 0, line-up 1 its first pilot (4) alone
	if roles[0] == 0 || roles[4] == 0 || len(roles) != 2 || roles[0] != roles4[0] || roles[4] != roles4[4] {
		t.Errorf("records by pilot: %v and %v, want pilots 0 and 4 alone", roles, roles4)
	}
}
