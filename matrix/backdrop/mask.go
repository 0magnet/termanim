// Package backdrop matrix/backdrop/mask.go
//
// A mask scales the backdrop cell by cell, where Dim scales it all at once.
//
// It is the difference between turning the rain down and cutting a shape out
// of it. Dim answers "how bright is the backdrop"; a mask answers "how bright
// is the backdrop *here*", which is what it takes to make a picture emerge
// from the rain rather than be painted over it. The rain keeps falling through
// the shape — the same glyphs, the same streams — and only its brightness
// changes, so what the reader sees is the rain itself arranging into
// something, which is the effect worth having. Painting a logo on top would be
// a logo on top.
package backdrop

// Mask reports how much to scale the backdrop at one cell, out of 256.
//
// 256 leaves the cell exactly as it was, below dims it, above brightens it.
// Brightening is not the same as adding light: the rain's intensity is a
// position on a ramp and a cell with nothing in it is at the bottom of it, so
// a mask can make the rain that is there brighter but cannot make rain where
// there is none. That is what keeps a masked shape looking like weather rather
// than like a sticker — it is only ever as solid as the rain passing through
// it at that moment.
type Mask interface {
	// ScaleAt returns the scale at x, y, out of 256.
	ScaleAt(x, y int) int
}

// Stencil is a Mask cut from a block of text: a cell is inside the shape when
// the corresponding character is anything but a space.
//
// Rows are indexed from Y downward and X rightward, so the block can be placed
// anywhere on the grid. A row shorter than the widest one is padded with
// outside, and a coordinate past the block is outside, so the caller does not
// have to make the block rectangular or clamp its own lookups.
type Stencil struct {
	// Rows is the shape. Space is outside, anything else is inside.
	Rows []string

	// X and Y are where the block's top-left corner sits on the grid.
	X, Y int

	// Inside and Outside are the scales, out of 256. Zero means 256 for both,
	// so a Stencil with neither set is a no-op rather than a black rectangle
	// — the zero value of a mask should change nothing.
	Inside, Outside int
}

// ScaleAt implements Mask.
func (s *Stencil) ScaleAt(x, y int) int {
	if s.inside(x, y) {
		return scaleOr256(s.Inside)
	}
	return scaleOr256(s.Outside)
}

// inside reports whether the grid cell x, y falls on a non-space character of
// the block.
func (s *Stencil) inside(x, y int) bool {
	row := y - s.Y
	if row < 0 || row >= len(s.Rows) {
		return false
	}
	col := x - s.X
	line := s.Rows[row]
	if col < 0 || col >= len(line) {
		return false
	}
	return line[col] != ' '
}

// Size reports the block's extent in cells.
func (s *Stencil) Size() (cols, rows int) {
	for _, r := range s.Rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	return cols, len(s.Rows)
}

// scaleOr256 reads a scale field, treating the zero value as "no change".
func scaleOr256(n int) int {
	if n == 0 {
		return 256
	}
	if n < 0 {
		return 0
	}
	return n
}

// maskAt is the scale at one cell for a possibly-nil mask.
func maskAt(m Mask, x, y int) int {
	if m == nil {
		return 256
	}
	return scaleOr256(m.ScaleAt(x, y))
}
