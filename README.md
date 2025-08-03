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

```
