// Package embed holds the Embedder implementations used by the search
// Service. Only the noop embedder is provided in the default build; real
// local-ONNX or remote-endpoint embedders land behind build tags or in
// follow-up milestones.
package embed

import "context"

// Noop is an Embedder placeholder that returns a single-dim zero vector for
// every input. It lets vector-mode search wiring pass through end-to-end
// without pulling in any real embedding dependency.
type Noop struct{}

func (Noop) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0}
	}
	return out, nil
}

func (Noop) Dim() int      { return 1 }
func (Noop) Model() string { return "noop" }
