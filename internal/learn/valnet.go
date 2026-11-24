package learn

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// ValueNet is a small fully connected network from a feature vector to the
// probability that the viewing seat wins. train/train_value.py writes the
// file: the magic "CAIV0001", a little-endian u64 header length, a JSON
// header and the float32 tensors: the input scale, the first layer's weights
// and bias, then each further layer's weights and bias.
//
// The trainer standardises its inputs, (x - mean) * scale. The mean's share
// is a constant per hidden unit, which the exporter folds into the first
// bias, so the file holds the scale alone and a zero input contributes
// nothing.
//
// The first layer is stored input-major, so a forward pass touches only the
// columns of the inputs that are not zero; most of a feature vector is.
type ValueNet struct {
	Meta   map[string]any
	in     int
	scale  []float32 // 1/std per input
	w0     []float32 // in × h0, input-major
	b0     []float32
	layers []dense // the layers after the first
}

type dense struct {
	in, out int
	w       []float32 // out × in, row-major
	b       []float32
}

const valNetMagic = "CAIV0001"

// LoadValueNet reads a value network and checks it against this build's
// feature width.
func LoadValueNet(path string) (*ValueNet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 16 || string(raw[:8]) != valNetMagic {
		return nil, fmt.Errorf("%s: not a value network file", path)
	}
	hl := int(binary.LittleEndian.Uint64(raw[8:16]))
	if hl <= 0 || 16+hl > len(raw) {
		return nil, fmt.Errorf("%s: bad header length", path)
	}
	var h struct {
		Width  int            `json:"width"`
		Hidden []int          `json:"hidden"`
		Meta   map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(raw[16:16+hl], &h); err != nil {
		return nil, fmt.Errorf("%s: header: %w", path, err)
	}
	if h.Width != Width {
		return nil, fmt.Errorf("%s: trained on %d features, this build encodes %d", path, h.Width, Width)
	}
	if len(h.Hidden) == 0 {
		return nil, fmt.Errorf("%s: no hidden layer", path)
	}
	data := raw[16+hl:]
	take := func(n int) ([]float32, error) {
		if len(data) < 4*n {
			return nil, fmt.Errorf("%s: truncated", path)
		}
		out := make([]float32, n)
		for i := range out {
			out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4*i:]))
		}
		data = data[4*n:]
		return out, nil
	}
	n := &ValueNet{Meta: h.Meta, in: h.Width}
	var e error
	get := func(k int) []float32 {
		v, err := take(k)
		if err != nil && e == nil {
			e = err
		}
		return v
	}
	n.scale = get(h.Width)
	n.w0, n.b0 = get(h.Width*h.Hidden[0]), get(h.Hidden[0])
	sizes := append(append([]int{}, h.Hidden...), 1)
	for i := 1; i < len(sizes); i++ {
		n.layers = append(n.layers, dense{in: sizes[i-1], out: sizes[i], w: get(sizes[i] * sizes[i-1]), b: get(sizes[i])})
	}
	if e != nil {
		return nil, e
	}
	if len(data) != 0 {
		return nil, fmt.Errorf("%s: %d trailing bytes", path, len(data))
	}
	return n, nil
}

// Scratch is the working memory of one forward pass. One per goroutine.
type Scratch struct{ a, b []float32 }

// NewScratch sizes a scratch for n.
func (n *ValueNet) NewScratch() *Scratch {
	w := len(n.b0)
	for _, l := range n.layers {
		w = max(w, l.out)
	}
	return &Scratch{a: make([]float32, w), b: make([]float32, w)}
}

// Value is the network's win probability for the features x. It allocates
// nothing.
func (n *ValueNet) Value(x []float32, sc *Scratch) float32 {
	h0 := len(n.b0)
	a := sc.a[:h0]
	copy(a, n.b0)
	for i, v := range x {
		if v == 0 {
			continue
		}
		z := v * n.scale[i]
		col := n.w0[i*h0 : (i+1)*h0]
		for j, w := range col {
			a[j] += z * w
		}
	}
	for j, v := range a {
		if v < 0 {
			a[j] = 0
		}
	}
	cur, nxt := a, sc.b
	for li, l := range n.layers {
		out := nxt[:l.out]
		for o := 0; o < l.out; o++ {
			row := l.w[o*l.in : (o+1)*l.in]
			s := l.b[o]
			for j, w := range row {
				s += w * cur[j]
			}
			if li < len(n.layers)-1 && s < 0 {
				s = 0
			}
			out[o] = s
		}
		cur, nxt = out, cur[:cap(cur)]
	}
	return float32(1 / (1 + math.Exp(-float64(cur[0]))))
}
