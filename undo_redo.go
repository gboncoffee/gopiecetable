package gopiecetable

import (
	"errors"
	"slices"
)

// Returned when there's nothing left to undo.
var ErrorBottomOfUndoList = errors.New("reached bottom of undo list")

// Returned when there's nothing left to redo.
var ErrorTopOfUndoList = errors.New("reached top of undo list")

// How undo/redo works:
//
// Every modification of the pieces array goes through replacePieces, which
// records it as a change: "the pieces [piecIdx, piecIdx+len(before)) were
// replaced by after". An edit is a list of changes. Undoing an edit replaces
// each after by its before, in reverse order, and redoing replaces each before
// by its after, in order. As edits are always undone and redone in stack
// order, the pieces array is restored exactly, and so is the content.

// The kind of an edit.
type editKind int

const (
	insertion editKind = iota
	deletion
)

// A change replaces the pieces [piecIdx, piecIdx+len(before)) by after.
type change struct {
	piecIdx int
	before  []piece
	after   []piece
}

// Represents an edit, i.e., a sequence of insertions or deletions that is
// undone and redone as a whole.
type edit struct {
	kind    editKind
	idx     int // The real index where the edit begins.
	length  int // The number of items inserted or deleted.
	changes []change
}

// Where the cursor should be after undoing the edit.
func (e *edit) undoIndex() int {
	if e.kind == insertion {
		return e.idx
	}
	return e.idx + e.length
}

// Where the cursor should be after redoing the edit.
func (e *edit) redoIndex() int {
	if e.kind == insertion {
		return e.idx + e.length
	}
	return e.idx
}

// Undo undoes the last edit. Returns the index where the edit happened.
func (b *PieceTable[Content]) Undo() (int, error) {
	if b.undoTop < 1 {
		return 0, ErrorBottomOfUndoList
	}

	b.undoTop--
	b.editOpen = false
	e := &b.edits[b.undoTop]
	for i := len(e.changes) - 1; i >= 0; i-- {
		c := e.changes[i]
		b.pieces = slices.Replace(
			b.pieces, c.piecIdx, c.piecIdx+len(c.after), c.before...)
	}
	if e.kind == insertion {
		b.size -= e.length
	} else {
		b.size += e.length
	}
	return e.undoIndex(), nil
}

// Redo redoes the last undone edit. Returns the index where the edit happened.
func (b *PieceTable[Content]) Redo() (int, error) {
	if b.undoTop == len(b.edits) {
		return 0, ErrorTopOfUndoList
	}

	e := &b.edits[b.undoTop]
	b.undoTop++
	b.editOpen = false
	for _, c := range e.changes {
		b.pieces = slices.Replace(
			b.pieces, c.piecIdx, c.piecIdx+len(c.before), c.after...)
	}
	if e.kind == insertion {
		b.size += e.length
	} else {
		b.size -= e.length
	}
	return e.redoIndex(), nil
}

// Should be called every action that's not an undo or redo so the list is
// wrapped.
func (b *PieceTable[Content]) normalizeUndo() {
	b.edits = b.edits[:b.undoTop]
}

// Returns the edit that new changes may be grouped into, or nil if there's
// none (there are no edits, or the last action was an undo or redo).
func (b *PieceTable[Content]) openEdit() *edit {
	if !b.editOpen || b.undoTop == 0 || b.undoTop != len(b.edits) {
		return nil
	}
	return &b.edits[b.undoTop-1]
}

func (b *PieceTable[Content]) pushEdit(kind editKind, idx int) {
	b.edits = append(b.edits, edit{kind: kind, idx: idx, length: 1})
	b.undoTop = len(b.edits)
	b.editOpen = true
}

// Must be called before modifying the pieces for an insertion at idx. Groups
// the insertion with the last edit if it's an insertion that ends at idx
// (typing left-to-right), otherwise starts a new edit.
func (b *PieceTable[Content]) startInsertion(idx int) {
	if e := b.openEdit(); e != nil && e.kind == insertion &&
		e.idx+e.length == idx {
		e.length++
		return
	}
	b.pushEdit(insertion, idx)
}

// Must be called before modifying the pieces for a deletion at idx. Groups the
// deletion with the last edit if it's a deletion that begins right after idx
// (deleting right-to-left, like backspace), otherwise starts a new edit.
func (b *PieceTable[Content]) startDeletion(idx int) {
	if e := b.openEdit(); e != nil && e.kind == deletion && e.idx-1 == idx {
		e.idx = idx
		e.length++
		return
	}
	b.pushEdit(deletion, idx)
}

// Replaces the n pieces starting at piecIdx by newPieces, recording the change
// in the current edit. Every modification of the pieces array outside of undo
// and redo must go through here.
func (b *PieceTable[Content]) replacePieces(
	piecIdx int,
	n int,
	newPieces ...piece,
) {
	e := &b.edits[b.undoTop-1]

	// If the replaced range is inside the range written by the last change,
	// fold this change into it instead of recording a new one. This keeps,
	// e.g., typing a long text from recording a change per item.
	if k := len(e.changes); k > 0 {
		last := &e.changes[k-1]
		rel := piecIdx - last.piecIdx
		if rel >= 0 && rel+n <= len(last.after) {
			last.after = slices.Replace(last.after, rel, rel+n, newPieces...)
			b.pieces = slices.Replace(b.pieces, piecIdx, piecIdx+n, newPieces...)
			return
		}
	}

	e.changes = append(e.changes, change{
		piecIdx: piecIdx,
		before:  slices.Clone(b.pieces[piecIdx : piecIdx+n]),
		after:   slices.Clone(newPieces),
	})
	b.pieces = slices.Replace(b.pieces, piecIdx, piecIdx+n, newPieces...)
}
