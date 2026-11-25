package gopiecetable

// Tests for the undo/redo system.
//
// Besides the content, the tests check that undo and redo restore the pieces
// exactly: edits are recorded as replacements of ranges of the pieces array,
// so any piece left different after an undo or redo would make older edits
// replace the wrong pieces, corrupting the content (e.g., reordering it or
// bringing back deleted text).

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func helperTestSize(t *testing.T, b *PieceTable[rune], expected int) {
	t.Helper()
	if b.Size() != expected {
		t.Fatalf("expected size %d, got %d", expected, b.Size())
	}
}

// Checks content, size and indexing all at once.
func helperTestState(t *testing.T, b *PieceTable[rune], expected string) {
	t.Helper()
	helperTestContent(t, b, expected)
	helperTestSize(t, b, len([]rune(expected)))
	helperTestIndexing(t, b, expected)
}

func helperTestPieces(t *testing.T, b *PieceTable[rune], expected []piece) {
	t.Helper()
	if !slices.Equal(b.pieces, expected) {
		t.Fatalf("pieces are %v, expected %v", b.pieces, expected)
	}
}

func helperUndo(t *testing.T, b *PieceTable[rune], expectedIdx int) {
	t.Helper()
	idx, err := b.Undo()
	if err != nil {
		t.Fatalf("undo failed: %v", err)
	}
	if idx != expectedIdx {
		t.Fatalf("undo returned index %d, expected %d", idx, expectedIdx)
	}
}

func helperRedo(t *testing.T, b *PieceTable[rune], expectedIdx int) {
	t.Helper()
	idx, err := b.Redo()
	if err != nil {
		t.Fatalf("redo failed: %v", err)
	}
	if idx != expectedIdx {
		t.Fatalf("redo returned index %d, expected %d", idx, expectedIdx)
	}
}

func helperUndoFails(t *testing.T, b *PieceTable[rune]) {
	t.Helper()
	if _, err := b.Undo(); !errors.Is(err, ErrorBottomOfUndoList) {
		t.Fatalf("expected ErrorBottomOfUndoList, got %v", err)
	}
}

func helperRedoFails(t *testing.T, b *PieceTable[rune]) {
	t.Helper()
	if _, err := b.Redo(); !errors.Is(err, ErrorTopOfUndoList) {
		t.Fatalf("expected ErrorTopOfUndoList, got %v", err)
	}
}

func helperType(b *PieceTable[rune], idx int, s string) {
	for _, c := range s {
		b.Insert(idx, c)
		idx++
	}
}

func helperBackspace(b *PieceTable[rune], idx int, n int) {
	for range n {
		idx--
		b.Delete(idx)
	}
}

func TestUndoRedoEmptyHistory(t *testing.T) {
	b := FromString("Hello")
	helperUndoFails(t, b)
	helperRedoFails(t, b)
	helperTestState(t, b, "Hello")

	e := New[rune]()
	helperUndoFails(t, e)
	helperRedoFails(t, e)
	helperTestState(t, e, "")
}

func TestUndoRedoInsertBeginning(t *testing.T) {
	b := FromString("World")
	b.Insert(0, '!')
	helperTestState(t, b, "!World")
	helperUndo(t, b, 0)
	helperTestState(t, b, "World")
	helperUndoFails(t, b)
	helperRedo(t, b, 1)
	helperTestState(t, b, "!World")
	helperRedoFails(t, b)
}

func TestUndoRedoInsertMiddle(t *testing.T) {
	b := FromString("Hello World")
	b.Insert(5, ',')
	helperTestState(t, b, "Hello, World")
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello World")
	helperUndoFails(t, b)
	helperRedo(t, b, 6)
	helperTestState(t, b, "Hello, World")
	helperRedoFails(t, b)
}

func TestUndoRedoInsertEnd(t *testing.T) {
	b := FromString("Hello")
	b.Insert(5, '!')
	helperTestState(t, b, "Hello!")
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello")
	helperUndoFails(t, b)
	helperRedo(t, b, 6)
	helperTestState(t, b, "Hello!")
	helperRedoFails(t, b)
}

func TestUndoRedoDeleteBeginning(t *testing.T) {
	b := FromString("Hello")
	b.Delete(0)
	helperTestState(t, b, "ello")
	helperUndo(t, b, 1)
	helperTestState(t, b, "Hello")
	helperRedo(t, b, 0)
	helperTestState(t, b, "ello")
}

func TestUndoRedoDeleteMiddle(t *testing.T) {
	b := FromString("Hello")
	b.Delete(2)
	helperTestState(t, b, "Helo")
	helperUndo(t, b, 3)
	helperTestState(t, b, "Hello")
	helperRedo(t, b, 2)
	helperTestState(t, b, "Helo")
}

func TestUndoRedoDeleteEnd(t *testing.T) {
	b := FromString("Hello")
	b.Delete(4)
	helperTestState(t, b, "Hell")
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello")
	helperRedo(t, b, 4)
	helperTestState(t, b, "Hell")
}

// Undoing and redoing the same edit over and over must be stable.
func TestUndoRedoRepeated(t *testing.T) {
	b := FromString("Hello World")
	b.Delete(5)
	for range 10 {
		helperUndo(t, b, 6)
		helperTestState(t, b, "Hello World")
		helperRedo(t, b, 5)
		helperTestState(t, b, "HelloWorld")
	}
}

// Typing sequentially should be undone as a single edit.
func TestUndoRedoTypingIsOneEdit(t *testing.T) {
	for _, tc := range []struct {
		name, initial, typed, expected string
		idx                            int
	}{
		{"beginning", "World", "Hello ", "Hello World", 0},
		{"middle", "Hello World", ", dear", "Hello, dear World", 5},
		{"end", "Hello", " World", "Hello World", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := FromString(tc.initial)
			helperType(b, tc.idx, tc.typed)
			helperTestState(t, b, tc.expected)
			helperUndo(t, b, tc.idx)
			helperTestState(t, b, tc.initial)
			helperUndoFails(t, b)
			helperRedo(t, b, tc.idx+len(tc.typed))
			helperTestState(t, b, tc.expected)
			helperRedoFails(t, b)
		})
	}
}

// Deleting backwards sequentially (backspace) should be undone as a single
// edit.
func TestUndoRedoBackspaceIsOneEdit(t *testing.T) {
	for _, tc := range []struct {
		name, expected string
		idx, n         int
	}{
		{"from end", "Hello", 11, 6},
		{"from middle", "Hellorld", 7, 3},
		{"to beginning", " World", 5, 5},
		{"everything", "", 11, 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := FromString("Hello World")
			helperBackspace(b, tc.idx, tc.n)
			helperTestState(t, b, tc.expected)
			helperUndo(t, b, tc.idx)
			helperTestState(t, b, "Hello World")
			helperUndoFails(t, b)
			helperRedo(t, b, tc.idx-tc.n)
			helperTestState(t, b, tc.expected)
			helperRedoFails(t, b)
		})
	}
}

// Edits that only add or remove whole pieces are undone exactly, so we can
// undo all of them.
func TestUndoRedoWholePieces(t *testing.T) {
	b := FromString("ac")
	b.Insert(1, 'b')
	b.Insert(3, 'd') // "abcd", four pieces: a, b, c, d.
	helperTestState(t, b, "abcd")
	helperBackspace(b, 4, 4)
	helperTestState(t, b, "")
	helperUndo(t, b, 4)
	helperTestState(t, b, "abcd")
	helperRedo(t, b, 0)
	helperTestState(t, b, "")
	helperUndo(t, b, 4)
	helperUndo(t, b, 3)
	helperTestState(t, b, "abc")
	helperUndo(t, b, 1)
	helperTestState(t, b, "ac")
	helperUndoFails(t, b)
	helperRedo(t, b, 2)
	helperRedo(t, b, 4)
	helperRedo(t, b, 0)
	helperTestState(t, b, "")
	helperRedoFails(t, b)
}

// Several edits in a row, each at a piece boundary, are fully undoable.
func TestUndoRedoWholeHistory(t *testing.T) {
	b := FromString("Hello World")
	b.Insert(5, ',')          // "Hello, World"
	helperType(b, 0, ">> ")   // ">> Hello, World"
	helperType(b, 15, "!!")   // ">> Hello, World!!"
	helperBackspace(b, 17, 2) // ">> Hello, World"
	helperBackspace(b, 3, 3)  // "Hello, World"
	states := []string{
		"Hello World",
		"Hello, World",
		">> Hello, World",
		">> Hello, World!!",
		">> Hello, World",
		"Hello, World",
	}
	for i := len(states) - 2; i >= 0; i-- {
		if _, err := b.Undo(); err != nil {
			t.Fatalf("undo failed: %v", err)
		}
		helperTestState(t, b, states[i])
	}
	helperUndoFails(t, b)
	for i := 1; i < len(states); i++ {
		if _, err := b.Redo(); err != nil {
			t.Fatalf("redo failed: %v", err)
		}
		helperTestState(t, b, states[i])
	}
	helperRedoFails(t, b)
}

func TestUndoRedoNewEditDiscardsRedo(t *testing.T) {
	b := FromString("Hello")
	b.Insert(5, '!')
	helperUndo(t, b, 5)
	b.Insert(5, '?')
	helperTestState(t, b, "Hello?")
	helperRedoFails(t, b)
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello")
	helperUndoFails(t, b)
	helperRedo(t, b, 6)
	helperTestState(t, b, "Hello?")

	b.Delete(0)
	helperUndo(t, b, 1)
	b.Delete(5)
	helperTestState(t, b, "Hello")
	helperRedoFails(t, b)
}

// A failed edit must not touch the history.
func TestUndoRedoFailedEditKeepsRedo(t *testing.T) {
	b := FromString("Hello")
	b.Insert(5, '!')
	helperUndo(t, b, 5)
	if err := b.Delete(10); err == nil {
		t.Fatalf("expected delete out of bounds to fail")
	}
	if err := b.Insert(10, 'x'); err == nil {
		t.Fatalf("expected insert out of bounds to fail")
	}
	helperRedo(t, b, 6)
	helperTestState(t, b, "Hello!")

	e := New[rune]()
	if err := e.Insert(1, 'x'); err == nil {
		t.Fatalf("expected insert out of bounds in empty table to fail")
	}
	helperUndoFails(t, e)
	helperTestState(t, e, "")
}

func TestUndoRedoNewBuffer(t *testing.T) {
	b := New[rune]()
	helperType(b, 0, "Hello")
	helperTestState(t, b, "Hello")
	helperUndo(t, b, 0)
	helperTestState(t, b, "")
	helperUndoFails(t, b)
	helperRedo(t, b, 5)
	helperTestState(t, b, "Hello")

	// Delete everything and type again in the now empty buffer.
	helperBackspace(b, 5, 5)
	helperTestState(t, b, "")
	helperType(b, 0, "Bye")
	helperTestState(t, b, "Bye")
	helperUndo(t, b, 0)
	helperTestState(t, b, "")
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello")
	helperRedo(t, b, 0)
	helperRedo(t, b, 3)
	helperTestState(t, b, "Bye")
}

// Appending to a piece restored by undoing a deletion must not extend the
// previous insertion edit, which refers to another piece.
func TestUndoRedoAppendAfterUndoDeletion(t *testing.T) {
	b := FromString("xy")
	helperType(b, 2, "ab") // "xyab"
	b.Delete(3)            // "xya"
	helperUndo(t, b, 4)    // "xyab"
	helperTestState(t, b, "xyab")
	b.Insert(4, 'c') // "xyabc"
	helperTestState(t, b, "xyabc")
	helperUndo(t, b, 4)
	helperTestState(t, b, "xyab")
	helperRedo(t, b, 5)
	helperTestState(t, b, "xyabc")
	helperUndo(t, b, 4)
	helperTestState(t, b, "xyab")
	helperUndo(t, b, 2)
	helperTestState(t, b, "xy")
	helperUndoFails(t, b)
	helperRedo(t, b, 4)
	helperRedo(t, b, 5)
	helperTestState(t, b, "xyabc")
	helperRedoFails(t, b)
}

func TestUndoRedoDeletionSize(t *testing.T) {
	b := FromString("Hello")
	b.Delete(4)
	helperUndo(t, b, 5)
	helperTestSize(t, b, 5)
	helperRedo(t, b, 4)
	helperTestState(t, b, "Hell")
}

// Typing more than a backing buffer holds forces new backing buffers to be
// allocated.
func TestUndoRedoTypingAcrossBackingBuffers(t *testing.T) {
	b := New[rune]()
	n := b.bufferSize()*2 + b.bufferSize()/2
	text := strings.Repeat("abcdefghij", n/10+1)[:n]

	helperType(b, 0, text)
	helperTestState(t, b, text)

	// Typing is a single edit even when it spans several backing buffers.
	helperUndo(t, b, 0)
	helperTestState(t, b, "")
	helperUndoFails(t, b)
	helperRedo(t, b, n)
	helperTestState(t, b, text)
	helperRedoFails(t, b)
}

// Deletions crossing backing buffers produce deleted pieces that span more
// than one backing buffer.
func TestUndoRedoDeletingAcrossBackingBuffers(t *testing.T) {
	b := New[rune]()
	n := b.bufferSize()*2 + b.bufferSize()/2
	text := strings.Repeat("abcdefghij", n/10+1)[:n]
	helperType(b, 0, text)

	// Backspace a range in the middle, crossing a buffer boundary.
	mid := b.bufferSize() + 5
	helperBackspace(b, mid, 10)
	afterDelete := text[:mid-10] + text[mid:]
	helperTestState(t, b, afterDelete)

	// Delete from the beginning of a piece that starts at a buffer boundary.
	b.Delete(mid - 10)
	afterDelete2 := afterDelete[:mid-10] + afterDelete[mid-9:]
	helperTestState(t, b, afterDelete2)

	helperUndo(t, b, mid-9)
	helperTestState(t, b, afterDelete)
	helperUndo(t, b, mid)
	helperTestState(t, b, text)
	helperRedo(t, b, mid-10)
	helperTestState(t, b, afterDelete)
	helperRedo(t, b, mid-10)
	helperTestState(t, b, afterDelete2)
	helperRedoFails(t, b)
}

// The README example.
func TestUndoRedoReadme(t *testing.T) {
	b := FromString("Hello World")
	b.Insert(5, ',')
	b.Insert(b.Size(), '!')
	helperUndo(t, b, 12)
	helperTestState(t, b, "Hello, World")
	helperUndo(t, b, 5)
	helperTestState(t, b, "Hello World")
	helperRedo(t, b, 6)
	helperRedo(t, b, 13)
	helperTestState(t, b, "Hello, World!")
}

// Undo must restore the pieces exactly, including edits that split or shrunk
// a piece.
func TestUndoRestoresPieces(t *testing.T) {
	original := []piece{{buffer: 0, start: 0, length: 5}}
	for _, tc := range []struct {
		name string
		edit func(b *PieceTable[rune])
	}{
		{"insert beginning", func(b *PieceTable[rune]) { b.Insert(0, 'x') }},
		{"insert middle", func(b *PieceTable[rune]) { b.Insert(2, 'x') }},
		{"insert end", func(b *PieceTable[rune]) { b.Insert(5, 'x') }},
		{"type middle", func(b *PieceTable[rune]) { helperType(b, 2, "xyz") }},
		{"delete beginning", func(b *PieceTable[rune]) { b.Delete(0) }},
		{"delete middle", func(b *PieceTable[rune]) { b.Delete(2) }},
		{"delete end", func(b *PieceTable[rune]) { b.Delete(4) }},
		{"backspace middle", func(b *PieceTable[rune]) { helperBackspace(b, 4, 2) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := FromString("Hello")
			tc.edit(b)
			edited := slices.Clone(b.pieces)
			b.Undo()
			helperTestState(t, b, "Hello")
			helperTestPieces(t, b, original)
			b.Redo()
			helperTestPieces(t, b, edited)
		})
	}
}

// Sequences that used to corrupt the content when undoing past an edit that
// split or shrunk a piece.
func TestUndoRedoPastSplitPieces(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edits  func(b *PieceTable[rune])
		states []string
	}{
		{
			"two deletions",
			func(b *PieceTable[rune]) { b.Delete(4); b.Delete(0) },
			[]string{"Hello", "Hell", "ell"},
		},
		{
			"deletion then middle insertion",
			func(b *PieceTable[rune]) { b.Delete(4); b.Insert(2, 'x') },
			[]string{"Hello", "Hell", "Hexll"},
		},
		{
			"typing then deletion",
			func(b *PieceTable[rune]) { helperType(b, 5, "ab"); b.Delete(6) },
			[]string{"Hello", "Helloab", "Helloa"},
		},
		{
			"middle insertions",
			func(b *PieceTable[rune]) {
				b.Insert(2, 'x')
				b.Insert(1, 'y')
				b.Insert(5, 'z')
			},
			[]string{"Hello", "Hexllo", "Hyexllo", "Hyexlzlo"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := FromString("Hello")
			tc.edits(b)
			last := len(tc.states) - 1
			helperTestState(t, b, tc.states[last])
			for i := last - 1; i >= 0; i-- {
				if _, err := b.Undo(); err != nil {
					t.Fatalf("undo failed: %v", err)
				}
				helperTestState(t, b, tc.states[i])
			}
			helperUndoFails(t, b)
			for i := 1; i <= last; i++ {
				if _, err := b.Redo(); err != nil {
					t.Fatalf("redo failed: %v", err)
				}
				helperTestState(t, b, tc.states[i])
			}
			helperRedoFails(t, b)
		})
	}
}

// Edits are not grouped across an undo or redo.
func TestUndoRedoNoGroupingAcrossUndo(t *testing.T) {
	b := FromString("xy")
	helperType(b, 2, "ab")
	helperUndo(t, b, 2)
	helperRedo(t, b, 4)
	b.Insert(4, 'c')
	helperUndo(t, b, 4)
	helperTestState(t, b, "xyab")

	b = FromString("Hello")
	helperBackspace(b, 5, 2)
	helperUndo(t, b, 5)
	helperRedo(t, b, 3)
	b.Delete(2)
	helperUndo(t, b, 3)
	helperTestState(t, b, "Hel")
}

// Model-based test: applies random edits, undos and redos, and checks the
// piece table against a snapshot of the expected content for every point of
// the history.
//
// The history granularity (which edits are grouped together) is an
// implementation detail, so the test learns it from undoTop: if an edit grows
// undoTop it creates a new history entry, otherwise it was merged into the
// current one.
//
// The test also keeps the pieces of every point of the history, and checks
// that undo and redo restore them exactly.
func TestUndoRedoRandom(t *testing.T) {
	// The big string is slow to compare on every step, so it gets one seed.
	initials := []struct {
		name, content string
		seeds         uint64
	}{
		{"empty", "", 5},
		{"small", testString, 5},
		{"big", bigString, 1},
	}
	for _, initial := range initials {
		for seed := range initial.seeds {
			t.Run(fmt.Sprintf("%s/%d", initial.name, seed), func(t *testing.T) {
				helperRandomUndoRedo(t, initial.content, seed, 2000)
			})
		}
	}
}

func helperRandomUndoRedo(t *testing.T, initial string, seed uint64, steps int) {
	var b *PieceTable[rune]
	if initial == "" {
		b = New[rune]()
	} else {
		b = FromString(initial)
	}

	reference := []rune(initial)
	// history[i] is the expected content when undoTop == i, and pieces[i]
	// the pieces the table had at that point.
	history := []string{initial}
	pieces := [][]piece{slices.Clone(b.pieces)}
	top := 0
	undos := 0

	rng := rand.New(rand.NewPCG(seed, 0xdecaf))
	position := 0
	log := make([]string, 0, steps)
	lastLog := func() []string {
		return log[max(0, len(log)-30):]
	}

	check := func(step int) {
		t.Helper()
		expected := history[top]
		if b.undoTop != top {
			t.Fatalf("step %d: undoTop is %d, expected %d\nlast ops: %v",
				step, b.undoTop, top, lastLog())
		}
		if got := String(b); got != expected {
			t.Fatalf("step %d: content mismatch\ngot:      %q\nexpected: %q\nlast ops: %v",
				step, got, expected, lastLog())
		}
		if b.Size() != len([]rune(expected)) {
			t.Fatalf("step %d: size is %d, expected %d\nlast ops: %v",
				step, b.Size(), len([]rune(expected)), lastLog())
		}
		// Checking every index of the big string on every step is too slow.
		if len(expected) < 5000 {
			for i, r := range []rune(expected) {
				c, err := b.Get(i)
				if err != nil || c != r {
					t.Fatalf("step %d: Get(%d) = %q, %v, expected %q\nlast ops: %v",
						step, i, c, err, r, lastLog())
				}
			}
		}
	}

	// Records the result of an edit in the history.
	record := func() {
		content := string(reference)
		if b.undoTop == top+1 {
			history = append(history[:top+1], content)
			pieces = append(pieces[:top+1], slices.Clone(b.pieces))
			top++
		} else if b.undoTop == top && top > 0 {
			history = history[:top+1]
			pieces = pieces[:top+1]
			history[top] = content
			pieces[top] = slices.Clone(b.pieces)
		} else {
			t.Fatalf("unexpected undoTop %d after edit (was %d)\nlast ops: %v",
				b.undoTop, top, lastLog())
		}
	}

	// After an undo or redo, checks that the pieces are the recorded ones.
	checkPieces := func(step int) {
		t.Helper()
		if !slices.Equal(b.pieces, pieces[top]) {
			t.Fatalf("step %d: pieces are %v, expected %v\nlast ops: %v",
				step, b.pieces, pieces[top], lastLog())
		}
	}

	for step := range steps {
		// Keep the cursor in bounds and sometimes jump around.
		if rng.IntN(10) == 0 || position > len(reference) {
			position = rng.IntN(len(reference) + 1)
		}

		switch op := rng.IntN(100); {
		case op < 40: // Type.
			r := rune('a' + rng.IntN(26))
			log = append(log, fmt.Sprintf("ins(%d,%c)", position, r))
			if err := b.Insert(position, r); err != nil {
				t.Fatalf("step %d: insert failed: %v\nlast ops: %v",
					step, err, lastLog())
			}
			reference = slices.Insert(reference, position, r)
			position++
			record()

		case op < 55: // Backspace.
			if position == 0 {
				continue
			}
			position--
			log = append(log, fmt.Sprintf("del(%d)", position))
			if err := b.Delete(position); err != nil {
				t.Fatalf("step %d: delete failed: %v\nlast ops: %v",
					step, err, lastLog())
			}
			reference = slices.Delete(reference, position, position+1)
			record()

		case op < 65: // Forward delete.
			if position >= len(reference) {
				continue
			}
			log = append(log, fmt.Sprintf("del(%d)", position))
			if err := b.Delete(position); err != nil {
				t.Fatalf("step %d: delete failed: %v\nlast ops: %v",
					step, err, lastLog())
			}
			reference = slices.Delete(reference, position, position+1)
			record()

		case op < 85: // Undo, sometimes several in a row.
			for range 1 + rng.IntN(4) {
				log = append(log, "undo")
				idx, err := b.Undo()
				if top == 0 {
					if !errors.Is(err, ErrorBottomOfUndoList) {
						t.Fatalf("step %d: expected bottom of undo list, got %v",
							step, err)
					}
					break
				}
				if err != nil {
					t.Fatalf("step %d: undo failed: %v\nlast ops: %v",
						step, err, lastLog())
				}
				undos++
				top--
				reference = []rune(history[top])
				if idx < 0 || idx > len(reference) {
					t.Fatalf("step %d: undo returned index %d out of [0, %d]",
						step, idx, len(reference))
				}
				position = idx
				checkPieces(step)
				check(step)
			}

		default: // Redo.
			log = append(log, "redo")
			idx, err := b.Redo()
			if top == len(history)-1 {
				if !errors.Is(err, ErrorTopOfUndoList) {
					t.Fatalf("step %d: expected top of undo list, got %v",
						step, err)
				}
				break
			}
			if err != nil {
				t.Fatalf("step %d: redo failed: %v\nlast ops: %v",
					step, err, lastLog())
			}
			top++
			reference = []rune(history[top])
			if idx < 0 || idx > len(reference) {
				t.Fatalf("step %d: redo returned index %d out of [0, %d]",
					step, idx, len(reference))
			}
			position = idx
			checkPieces(step)
		}

		check(step)
	}

	// Make sure the test is actually exercising undo.
	if undos < steps/20 {
		t.Fatalf("only %d undos in %d steps", undos, steps)
	}

	// Undo everything and redo everything.
	for top > 0 {
		if _, err := b.Undo(); err != nil {
			t.Fatalf("undo failed: %v", err)
		}
		top--
		check(steps)
		checkPieces(steps)
	}
	helperUndoFails(t, b)
	for top < len(history)-1 {
		if _, err := b.Redo(); err != nil {
			t.Fatalf("redo failed: %v", err)
		}
		top++
		check(steps)
		checkPieces(steps)
	}
	helperRedoFails(t, b)
}
