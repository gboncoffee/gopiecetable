package buffer

import "strings"

func String(b *Buffer[rune]) string {
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

func FromString(content string) *Buffer[rune] {
	buffer := new(Buffer[rune])
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

	// Fix the displacement.

	buffer.pieces = append(buffer.pieces, piece{
		buffer: 0,
		start:  0,
		length: buffer.buffers[0].size(),
	})

	return buffer
}
