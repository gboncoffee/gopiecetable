package buffer

import (
	"errors"
	"slices"
)

var ErrorBottomOfUndoList = errors.New("reached bottom of undo list")
var ErrorTopOfUndoList = errors.New("reached top of undo list")

type edit interface {
	undoIndex() int
	redoIndex() int
}

type insertion struct {
	idx     int
	piecIdx int
	piec    piece
}

func (i insertion) undoIndex() int {
	return i.idx
}

func (i insertion) redoIndex() int {
	return i.idx + i.piec.length
}

type deletion struct {
	idx     int
	piecIdx int
	length  int
	pieces  []piece
}

func (d deletion) undoIndex() int {
	return d.idx + d.length
}

func (d deletion) redoIndex() int {
	return d.idx
}

func (b *Buffer[Content]) Undo() (int, error) {
	if b.undoTop < 1 {
		return 0, ErrorBottomOfUndoList
	}

	b.undoTop--
	edit := b.edits[b.undoTop]
	b.undo(edit)
	return edit.undoIndex(), nil
}

func (b *Buffer[Content]) Redo() (int, error) {
	if b.undoTop == len(b.edits) {
		return 0, ErrorTopOfUndoList
	}

	b.undoTop++
	edit := b.edits[b.undoTop-1]
	b.redo(edit)
	return edit.redoIndex(), nil
}

func (b *Buffer[Content]) undo(e edit) {
	switch ed := e.(type) {
	case insertion:
		b.undoInsertion(ed)
	case deletion:
		b.undoDeletion(ed)
	}
}

func (b *Buffer[Content]) redo(e edit) {
	switch ed := e.(type) {
	case insertion:
		b.redoInsertion(ed)
	case deletion:
		b.redoDeletion(ed)
	}
}

func (b *Buffer[Content]) undoInsertion(i insertion) {
	piec := b.pieces[i.piecIdx]
	b.pieces = slices.Delete(b.pieces, i.piecIdx, i.piecIdx+1)
	b.size -= piec.length
}

func (b *Buffer[Content]) redoInsertion(i insertion) {
	b.pieces = slices.Insert(b.pieces, i.piecIdx, i.piec)
	b.size += i.piec.length
}

func (b *Buffer[Content]) undoDeletion(d deletion) {
	for i, p := range d.pieces {
		b.pieces = slices.Insert(b.pieces, d.piecIdx+i, p)
	}
	b.size += d.length
}

func (b *Buffer[Content]) redoDeletion(d deletion) {
	// We're guaranteed to have exactly the same pieces at the point of the
	// buffer.
	b.pieces = slices.Delete(b.pieces, d.piecIdx, d.piecIdx+len(d.pieces))
}

func (b *Buffer[Content]) normalizeUndo() {
	b.edits = b.edits[:b.undoTop]
}

func (b *Buffer[Content]) pushUndo(e edit) {
	b.edits = append(b.edits, e)
	b.undoTop = len(b.edits)
}

func (b *Buffer[Content]) pushInsertion(idx int, piecIdx int, piec piece) {
	b.pushUndo(insertion{idx: idx, piecIdx: piecIdx, piec: piec})
}

func (b *Buffer[Content]) extendInsertion() {
	i, _ := b.edits[b.undoTop-1].(insertion)
	i.piec.length++
	b.edits[b.undoTop-1] = i
}

func (b *Buffer[Content]) lastIsInsertion() bool {
	if b.undoTop == 0 {
		return false
	}
	_, is := b.edits[b.undoTop-1].(insertion)
	return is
}

func (b *Buffer[Content]) lastIsDeletion() bool {
	if b.undoTop == 0 {
		return false
	}
	_, is := b.edits[b.undoTop-1].(deletion)
	return is
}

func (b *Buffer[Content]) lastDeletionIdx() int {
	d, _ := b.edits[b.undoTop-1].(deletion)
	return d.idx
}

func (b *Buffer[Content]) undoRedoManageDeletion(
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

func (b *Buffer[Content]) undoRedoAddDeletionPiece(
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
