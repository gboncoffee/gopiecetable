package gopiecetable

import (
	"os"
	"unsafe"
)

// Backing buffer size in bytes.
var bufferSize = os.Getpagesize()

// Small and loose abstraction for the []Content.
type backingBuffer[Content any] struct {
	content []Content
}

// Literally `make([]Content, 0, size)`.
func newBackingBuffer[Content any](size int) backingBuffer[Content] {
	return backingBuffer[Content]{
		content: make([]Content, 0, size),
	}
}

// Literally `append(b.content, c)`.
func (b *backingBuffer[Content]) append(c Content) {
	b.content = append(b.content, c)
}

// Literally `len(b.content) == size`.
func (b *backingBuffer[Content]) full(size int) bool {
	return len(b.content) == size
}

// Appends to the last buffer and, if the operation fills it, allocates a new
// one.
func (b *PieceTable[Content]) appendToBack(c Content) {
	buf := &b.buffers[len(b.buffers)-1]
	buf.content = append(buf.content, c)
	if buf.full(b.bufferSize()) {
		b.buffers = append(b.buffers, newBackingBuffer[Content](b.bufferSize()))
	}
}

// Literally `len(b.content)`.
func (b *backingBuffer[Content]) size() int {
	return len(b.content)
}

// Returns the amount of items the buffers are normally allowed to grow up to.
func (b *PieceTable[Content]) bufferSize() int {
	var zero Content
	return bufferSize / int(unsafe.Sizeof(zero))
}
