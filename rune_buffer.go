package gopiecetable

import "strings"

// Returns the content of a PieceTable[rune] as a string.
func String(b *PieceTable[rune]) string {
	var builder strings.Builder

	// Trying to be intelligent about the memory.
	builder.Grow(
		((len(b.buffers)-1)*b.bufferSize() +
			b.buffers[0].size()) * 4)

	for _, piece := range b.pieces {
		content := b.pieceContent(piece)
		for _, r := range content {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// Same as FromSlice, but returns a PieceTable[rune] from the contents of a
// string. This function may allocate up to four times more memory than the
// required for the first buffer of the piece table if the text has lots of big
// UTF-8 runes. Note that the first buffer is immutable, so the additional
// memory is essentialy useless.
//
// For this reason, if you know that your text has lots of those runes, you may
// want to manually convert the string to a []rune and use FromSlice.
//
// For instance, the CJK characters (from Chinese, Japanese and Korean) are
// encoded as 2 or 3 byte runes. If using FromString with text consisting of
// only these languages, you're guaranteed to have at least a two times overhead
// for the first buffer.
func FromString(content string) *PieceTable[rune] {
	buffer := new(PieceTable[rune])
	buffer.buffers = make([]backingBuffer[rune], 2)

	// We make Go alloc a sane amount of memory (may be up to 4x more than we
	// actually need due to how UTF-8 works, but hey, we're doing only one
	// allocation, and who cares about virtual memory anyways?).
	buffer.buffers[0] = newBackingBuffer[rune](len(content))
	// This buffer does not have a displacement of zero, we're going to fix it
	// after. We'll only actual discover it's proper displacement after
	// iterating the string due to UTF-8.
	buffer.buffers[1] =
		newBackingBuffer[rune](buffer.bufferSize())

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
