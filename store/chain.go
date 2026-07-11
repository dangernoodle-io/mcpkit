package store

import (
	"context"
	"fmt"
)

// Layer is one link in a Chain: a Source plus whether it is the chain's
// write sink.
type Layer struct {
	Source   Source
	Writable bool
}

// Read wraps s as a read-only Layer.
func Read(s Source) Layer {
	return Layer{Source: s, Writable: false}
}

// Write wraps s as the Chain's writable Layer.
func Write(s Store) Layer {
	return Layer{Source: s, Writable: true}
}

// Chain composes multiple Layers, listed in ascending precedence (later
// layers override earlier ones for reads), into a single Store. Writes
// route to the sole writable layer.
//
// A Chain with zero writable layers is a valid, deliberate read-only
// composition: its Set/Save/Delete return ErrUnsupported rather than
// panicking, since "read-only chain" is a legitimate opt-in shape (unlike
// the ambiguous-write-sink case of 2+ writable layers, which is a
// compose-time programmer error and does panic).
type Chain struct {
	layers   []Layer
	writable Store
}

// NewChain builds a Chain from layers, listed in ascending precedence.
// It panics if more than one layer is writable, or if a layer marked
// Writable does not have a Source implementing Store.
func NewChain(layers ...Layer) *Chain {
	c := &Chain{layers: layers}

	for _, l := range layers {
		if !l.Writable {
			continue
		}

		s, ok := l.Source.(Store)
		if !ok {
			panic(fmt.Sprintf("store: chain layer marked Writable but Source %T does not implement Store", l.Source))
		}

		if c.writable != nil {
			panic("store: chain has more than one writable layer")
		}

		c.writable = s
	}

	return c
}

// Get probes layers from last (highest precedence) to first, returning the
// first hit.
func (c *Chain) Get(ctx context.Context, key string) (string, bool, error) {
	for i := len(c.layers) - 1; i >= 0; i-- {
		v, ok, err := c.layers[i].Source.Get(ctx, key)
		if err != nil {
			return "", false, err
		}

		if ok {
			return v, true, nil
		}
	}

	return "", false, nil
}

// Load merges every layer's Load result, first to last, so later layers
// override earlier ones.
func (c *Chain) Load(ctx context.Context) (map[string]string, error) {
	merged := make(map[string]string)

	for _, l := range c.layers {
		m, err := l.Source.Load(ctx)
		if err != nil {
			return nil, err
		}

		for k, v := range m {
			merged[k] = v
		}
	}

	return merged, nil
}

// Set routes to the chain's writable layer. Returns ErrUnsupported if the
// chain has no writable layer.
func (c *Chain) Set(ctx context.Context, key, value string) error {
	if c.writable == nil {
		return ErrUnsupported
	}

	return c.writable.Set(ctx, key, value)
}

// Save routes to the chain's writable layer. Returns ErrUnsupported if the
// chain has no writable layer.
func (c *Chain) Save(ctx context.Context) error {
	if c.writable == nil {
		return ErrUnsupported
	}

	return c.writable.Save(ctx)
}

// Delete routes to the chain's writable layer. Returns ErrUnsupported if
// the chain has no writable layer.
func (c *Chain) Delete(ctx context.Context, key string) error {
	if c.writable == nil {
		return ErrUnsupported
	}

	return c.writable.Delete(ctx, key)
}
