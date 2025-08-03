# Piece Table for Go

This is an implementation of the
[piece table](https://en.wikipedia.org/wiki/Piece_table) data structure
supporting infinite undo-redo. This data structure is very efficient for
implementing text editors.

This implementation uses Go 1.18 generics so users can encode the data as they
wish, although helper functions are available to use this to store `rune`,
making the structure very efficient and convenient to use to edit UTF-8 text.

The data structure is rather complex and hard to implement, being written in
~450 lines of Go code. There are lots of corner cases and details that can go
wrong.

The undo-redo functionality is hunk-based, i.e., if you insert (or delete) a
bunch of times in the same place, all your insertions (or deletions) will yield
a single "edit", so undoing it will remove (or reinsert) everything. This is
implemented so inserting left-to-right (the common way) or deleting
right-to-left (the usual way with the backspace key) yields a single edit, but
doing so in the reverse will yield multiple edits.

## Usage

`go get github.com/gboncoffee/gopiecetable@latest`

Example usage:

```go
b := FromString("Hello World")
b.Insert(5, ',') // "Hello, World"
b.Insert(b.Size(), '!') // "Hello, World!"
b.Undo() // "Hello, World"
b.Delete(5) // "Hello World"
b.Insert(5, ',') // "Hello, World"
b.Insert(b.Size(), '!') // "Hello, World!"
b.Undo() // "Hello, World"
b.Undo() // "Hello World"
b.Redo() // "Hello, World"
b.Redo() // "Hello, World!"
```

## Development

Testing this is very hard. There's a bunch of tests, including one that stresses
the piece table by making random pre-selected edits. It takes rather long to run
(almost 10 seconds in my machine). Also, I couldn't make a proper test for the
undo/redo functionality, all testing it has is done manually during the
development of my text editor [ah](https://github.com/gboncoffee/ah).
