package learn

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// The Go reader computes what the trainer computed: testdata/tiny.caiv is a
// network train/train_value.py exported, and tiny.probe.json holds inputs
// with the outputs PyTorch gave for them.
func TestValueNetMatchesTheTrainer(t *testing.T) {
	net, err := LoadValueNet("testdata/tiny.caiv")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/tiny.probe.json")
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		X [][]float32 `json:"x"`
		P []float64   `json:"p"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.X) == 0 || len(probe.X) != len(probe.P) {
		t.Fatalf("probe has %d inputs and %d outputs", len(probe.X), len(probe.P))
	}
	sc := net.NewScratch()
	for i, x := range probe.X {
		if len(x) != Width {
			t.Fatalf("probe input %d has %d features, this build encodes %d", i, len(x), Width)
		}
		got := float64(net.Value(x, sc))
		if math.Abs(got-probe.P[i]) > 1e-4 {
			t.Errorf("input %d: Go %.6f, trainer %.6f", i, got, probe.P[i])
		}
	}
	if n := testing.AllocsPerRun(20, func() { net.Value(probe.X[0], sc) }); n != 0 {
		t.Errorf("a forward pass allocates %v times", n)
	}
}
