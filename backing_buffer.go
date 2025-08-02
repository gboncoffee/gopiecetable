package buffer

type backingBuffer[Content any] struct {
	content []Content
}

func newBackingBuffer[Content any](size int) backingBuffer[Content] {
	return backingBuffer[Content]{
		content: make([]Content, 0, size),
	}
}

func (b *backingBuffer[Content]) append(c Content) {
	b.content = append(b.content, c)
}

func (b *backingBuffer[Content]) full(size int) bool {
	return len(b.content) == size
}

func (b *Buffer[Content]) appendToBack(c Content) {
	buf := &b.buffers[len(b.buffers)-1]
	buf.content = append(buf.content, c)
	if buf.full(b.bufferSize()) {
		b.buffers = append(b.buffers, newBackingBuffer[Content](b.bufferSize()))
	}
}

func (b *backingBuffer[Content]) size() int {
	return len(b.content)
}
