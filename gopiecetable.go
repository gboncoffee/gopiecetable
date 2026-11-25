// Package gopiecetable implements an efficient Piece Table generic buffer with
// infinite undo/redo capabilities.
package gopiecetable

import (
	"errors"
	"slices"
)

// Using slices for representing pieces was kinda weird so I didn't.

// Returned when trying to operate out of bounds.
var ErrorOutOfBounds = errors.New("out of bounds")

// PieceTable implements an efficient Piece Table with infinite undo/redo
// capabilities. You should get one from New, FromString or FromSlice.
type PieceTable[Content any] struct {
	// The first buffer never changes and does not respect the buffer size if
	// the piece table is initialized with content.
	buffers []backingBuffer[Content]
	// The pieces.
	pieces []piece
	// The undo/redo list.
	edits []edit
	// Cache. Remember to keep it in sync.
	size int
	// The top of the undo/redo list, i.e., the current edit is undoTop-1.
	undoTop int
	// Whether new insertions or deletions may be grouped into the current
	// edit. False right after an undo or redo.
	editOpen bool
}

// A piece.
type piece struct {
	buffer int
	start  int
	length int
}

// New returns an empty piece table.
func New[Content any]() *PieceTable[Content] {
	buffer := new(PieceTable[Content])
	buffer.buffers = make([]backingBuffer[Content], 1)
	buffer.buffers[0] = newBackingBuffer[Content](buffer.bufferSize())
	return buffer
}

// FromSlice returns a piece table initialized with the contents of the slice.
func FromSlice[Content any](content []Content) *PieceTable[Content] {
	buffer := new(PieceTable[Content])
	buffer.buffers = make([]backingBuffer[Content], 2)

	// Here the memory we alloc is exactly the needed.
	buffer.buffers[0] = newBackingBuffer[Content](len(content))
	buffer.buffers[1] = newBackingBuffer[Content](
		buffer.bufferSize())

	for _, c := range content {
		buffer.buffers[0].append(c)
		buffer.size++
	}
	buffer.pieces = append(buffer.pieces, piece{
		buffer: 0,
		start:  0,
		length: buffer.buffers[0].size(),
	})

	return buffer
}

// Content returns the content of a piece table as a slice.
func Content[Content any](b *PieceTable[Content]) []Content {
	content := make([]Content, 0, b.Size())

	for _, piece := range b.pieces {
		c := b.pieceContent(piece)
		content = append(content, c...)
	}

	return content
}

// Inserts inserts a single item r in the index idx at the piece table. You can
// set idx to the size of the piece table to append onto it.
func (b *PieceTable[Content]) Insert(idx int, r Content) error {
	pidx, disp := 0, 0
	if len(b.pieces) == 0 && idx != 0 {
		return ErrorOutOfBounds
	} else if len(b.pieces) > 0 {
		var err error
		pidx, disp, err = b.findPieceForInsertion(idx)
		if err != nil {
			return err
		}
	}

	b.normalizeUndo()
	b.startInsertion(idx)
	b.size++

	buffer := len(b.buffers) - 1
	newPiece := piece{
		buffer: buffer,
		start:  b.buffers[buffer].size(),
		length: 1,
	}

	b.appendToBack(r)

	// If the piece table is empty.
	if len(b.pieces) == 0 {
		b.replacePieces(0, 0, newPiece)
		return nil
	}

	piec := b.pieces[pidx]

	// If "appending" on the piece.
	if disp == piec.length {
		buf, i := b.indexByPiece(piec, piec.length-1)
		// Minus 2 because we need to exclude the item we just inserted.
		sm1 := b.buffers[buf].size() - 2
		// If the piece is pointing to the end of the buffers, we literally
		// append onto it.
		if buf == buffer && i == sm1 {
			piec.length++
			b.replacePieces(pidx, 1, piec)
			return nil
		}

		// Else we insert the new piece.
		b.replacePieces(pidx+1, 0, newPiece)
		return nil
	}

	// If inserting in the beggining of a piece.
	if disp == 0 {
		b.replacePieces(pidx, 0, newPiece)
		return nil
	}

	// If inserting in the middle of a piece, we split it.
	left := piece{
		buffer: piec.buffer,
		start:  piec.start,
		length: disp,
	}
	rightBuf, rightStart := b.indexByPiece(piec, disp)
	right := piece{
		buffer: rightBuf,
		start:  rightStart,
		length: piec.length - disp,
	}
	b.replacePieces(pidx, 1, left, newPiece, right)

	return nil
}

// Delete removes the item on the index idx.
func (b *PieceTable[Content]) Delete(idx int) error {
	pidx, disp, err := b.findPieceWithIdx(idx)
	if err != nil {
		return err
	}

	b.normalizeUndo()
	b.startDeletion(idx)
	b.size--

	piec := b.pieces[pidx]

	switch {
	// If the piece has a single item, we remove it.
	// TODO: when removing, we may have sequential pieces. We could merge them
	// with tryMergePieces, which is safe to do through replacePieces.
	case piec.length == 1:
		b.replacePieces(pidx, 1)

	// If removing from the top of the piece, we can simply decrease.
	case disp == piec.length-1:
		piec.length--
		b.replacePieces(pidx, 1, piec)

	// If removing from the beggining of the piece, we can simply increase the
	// start.
	case disp == 0:
		piec.start++
		piec.length--
		// If the piece begins at the end of the buffer.
		if piec.start == b.buffers[piec.buffer].size() {
			piec.buffer++
			piec.start = 0
		}
		b.replacePieces(pidx, 1, piec)

	// Else we need to split the piece.
	default:
		left := piece{
			buffer: piec.buffer,
			start:  piec.start,
			length: disp,
		}
		rightBuf, rightStart := b.indexByPiece(piec, disp+1)
		right := piece{
			buffer: rightBuf,
			start:  rightStart,
			length: piec.length - (disp + 1),
		}
		b.replacePieces(pidx, 1, left, right)
	}

	return nil
}

// Get returns the item at the index idx.
func (b *PieceTable[Content]) Get(idx int) (Content, error) {
	var zero Content
	piec, disp, err := b.findPieceWithIdx(idx)
	if err != nil {
		return zero, err
	}

	buf, d := b.indexByPiece(b.pieces[piec], disp)
	return b.buffers[buf].content[d], nil
}

// Size returns the size of the piece table.
func (b *PieceTable[Content]) Size() int {
	return b.size
}

// Given two indexes and a slice of pieces, tries to merge the given pieces.
// p2i must be p1i+1.
func (b *PieceTable[Content]) tryMergePieces(p1i, p2i int, pieces []piece) []piece {
	p1 := pieces[p1i]
	p2 := pieces[p2i]
	p1endbuf, p1enddisp := b.indexByPiece(p1, p1.length-1)

	// If the p1end is at the end of a buffer, we have to check wether the p2
	// begin is at the begin of the next one.
	if p1enddisp == b.buffers[p1endbuf].size()-1 {
		if p2.start == 0 && p2.buffer == p1endbuf+1 {
			return b.mergePieces(p1i, p2i, pieces)
		}
	} else if p1endbuf == p2.buffer && p1enddisp == p2.start-1 {
		return b.mergePieces(p1i, p2i, pieces)
	}
	return pieces
}

// Merges two sequential pieces.
func (b *PieceTable[Content]) mergePieces(p1i, p2i int, pieces []piece) []piece {
	removed := pieces[p2i]
	pieces = slices.Delete(pieces, p2i, p2i+1)
	pieces[p1i].length += removed.length
	return pieces
}

// Finds the piece with a given index.
func (b *PieceTable[Content]) findPieceWithIdx(idx int) (i int, d int, err error) {
	disp := 0
	for i, piece := range b.pieces {
		ndisp := piece.length + disp
		if ndisp > idx {
			return i, idx - disp, nil
		}
		disp = ndisp
	}

	return 0, 0, ErrorOutOfBounds
}

// Finds the piece with a given index, but returns the piece to it's left if
// the index is the first index of the piece (useful to avoiding another corner
// case in the insertion)
func (b *PieceTable[Content]) findPieceForInsertion(
	idx int,
) (i int, d int, err error) {
	disp := 0
	for i, piece := range b.pieces {
		ndisp := piece.length + disp
		if ndisp >= idx {
			return i, idx - disp, nil
		}
		disp = ndisp
	}

	return 0, 0, ErrorOutOfBounds
}

// Returns the content of a piece.
func (b *PieceTable[Content]) pieceContent(p piece) []Content {
	arr := make([]Content, 0, p.length)
	buf := p.buffer
	bdisp := p.start
	for range p.length {
		if bdisp >= b.buffers[buf].size() {
			bdisp = 0
			buf++
		}
		arr = append(arr, b.buffers[buf].content[bdisp])
		bdisp++
	}
	return arr
}

// Returns the buffer and displacement with a given displacement inside a given
// piece.
func (b *PieceTable[Content]) indexByPiece(p piece, d int) (buffer int, bdisp int) {
	// If in the first (piece) buffer.
	if p.start+d < b.buffers[p.buffer].size() {
		return p.buffer, d + p.start
	}

	disp := b.buffers[p.buffer].size() - p.start
	buf := p.buffer + 1
	for {
		newdisp := disp + b.buffers[buf].size()
		if newdisp > d {
			return buf, d - disp
		}
		buf++
		disp = newdisp
	}
}
