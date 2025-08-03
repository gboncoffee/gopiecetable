package gopiecetable

import (
	"errors"
	"slices"
)

// Returned when there's nothing left to undo.
var ErrorBottomOfUndoList = errors.New("reached bottom of undo list")

// Returned when there's nothing left to redo.
var ErrorTopOfUndoList = errors.New("reached top of undo list")

// Represents an edit.
type edit interface {
	undoIndex() int
	redoIndex() int
}

// Represents an insertion, implements edit.
type insertion struct {
	idx     int   // The real index.
	piecIdx int   // The index in the piece array.
	piec    piece // The piece itself.
}

func (i insertion) undoIndex() int {
	return i.idx
}

func (i insertion) redoIndex() int {
	return i.idx + i.piec.length
}

// Represents a deletion, implements edit.
type deletion struct {
	idx     int     // The real index.
	piecIdx int     // The index in the pieces array.
	length  int     // The total length deleted.
	pieces  []piece // The pieces deleted.
}

func (d deletion) undoIndex() int {
	return d.idx + d.length
}

func (d deletion) redoIndex() int {
	return d.idx
}

// Undoes the last edit.
func (b *PieceTable[Content]) Undo() (int, error) {
	if b.undoTop < 1 {
		return 0, ErrorBottomOfUndoList
	}

	b.undoTop--
	edit := b.edits[b.undoTop]
	b.undo(edit)
	return edit.undoIndex(), nil
}

// Redoes the last edit, if the last action was an undo.
func (b *PieceTable[Content]) Redo() (int, error) {
	if b.undoTop == len(b.edits) {
		return 0, ErrorTopOfUndoList
	}

	b.undoTop++
	edit := b.edits[b.undoTop-1]
	b.redo(edit)
	return edit.redoIndex(), nil
}

// Ugly. Should be part of the interface, but methods cannot have type
// parameters.
func (b *PieceTable[Content]) undo(e edit) {
	switch ed := e.(type) {
	case insertion:
		b.undoInsertion(ed)
	case deletion:
		b.undoDeletion(ed)
	}
}

// Ugly. Should be part of the interface, but methods cannot have type
// parameters.
func (b *PieceTable[Content]) redo(e edit) {
	switch ed := e.(type) {
	case insertion:
		b.redoInsertion(ed)
	case deletion:
		b.redoDeletion(ed)
	}
}

func (b *PieceTable[Content]) undoInsertion(i insertion) {
	piec := b.pieces[i.piecIdx]
	b.pieces = slices.Delete(b.pieces, i.piecIdx, i.piecIdx+1)
	b.size -= piec.length
}

func (b *PieceTable[Content]) redoInsertion(i insertion) {
	b.pieces = slices.Insert(b.pieces, i.piecIdx, i.piec)
	b.size += i.piec.length
}

func (b *PieceTable[Content]) undoDeletion(d deletion) {
	for i, p := range d.pieces {
		b.pieces = slices.Insert(b.pieces, d.piecIdx+i, p)
	}
	b.size += d.length
}

func (b *PieceTable[Content]) redoDeletion(d deletion) {
	// We're guaranteed to have exactly the same pieces at the point of the
	// buffer.
	b.pieces = slices.Delete(b.pieces, d.piecIdx, d.piecIdx+len(d.pieces))
}

// Should be called every action that's not an undo or redo so the list is
// wrapped.
func (b *PieceTable[Content]) normalizeUndo() {
	b.edits = b.edits[:b.undoTop]
}

func (b *PieceTable[Content]) pushInsertion(idx int, piecIdx int, piec piece) {
	e := insertion{idx: idx, piecIdx: piecIdx, piec: piec}
	b.edits = append(b.edits, e)
	b.undoTop = len(b.edits)
}

func (b *PieceTable[Content]) extendInsertion() {
	i, _ := b.edits[b.undoTop-1].(insertion)
	i.piec.length++
	b.edits[b.undoTop-1] = i
}

func (b *PieceTable[Content]) lastIsInsertion() bool {
	if b.undoTop == 0 {
		return false
	}
	_, is := b.edits[b.undoTop-1].(insertion)
	return is
}

func (b *PieceTable[Content]) lastIsDeletion() bool {
	if b.undoTop == 0 {
		return false
	}
	_, is := b.edits[b.undoTop-1].(deletion)
	return is
}

func (b *PieceTable[Content]) lastDeletionIdx() int {
	d, _ := b.edits[b.undoTop-1].(deletion)
	return d.idx
}

func (b *PieceTable[Content]) undoRedoManageDeletion(
	idx int,
	piecIdx int,
	p piece,
) {
	if b.lastIsDeletion() {
		ei := b.lastDeletionIdx()
		if ei-1 == idx {
			b.undoRedoAddDeletionPiece(idx, p, piecIdx)
			return
		}
	}
	b.edits = append(b.edits, deletion{
		idx:     idx,
		piecIdx: piecIdx,
		pieces:  []piece{p},
		length:  p.length,
	})
	b.undoTop++
}

func (b *PieceTable[Content]) undoRedoAddDeletionPiece(
	idx int,
	p piece,
	piecIdx int,
) {
	d, _ := b.edits[b.undoTop-1].(deletion)
	d.pieces = slices.Insert(d.pieces, 0, p)
	// Possibly merge the pieces.
	d.pieces = b.tryMergePieces(0, 1, d.pieces)
	d.piecIdx = piecIdx
	d.length += p.length
	d.idx = idx
	b.edits[b.undoTop-1] = d
}
