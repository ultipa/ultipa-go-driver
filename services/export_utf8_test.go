package services

// An export chunk must be usable on its own.
//
// Found by running the whole local suite on 8 October: the Python driver's
// per-chunk decode blew up with "byte 0xe6 in position 65535" — a 64 KiB chunk
// boundary that had cut a three-byte character in half. Go does not raise on
// such a chunk: string(chunk.Data) puts U+FFFD where the character was, so the
// same split corrupts text silently here.
//
// The export stream loop now holds back the start of a character a chunk cannot
// finish and puts it at the front of the next chunk. incompleteUTF8Tail is the
// arithmetic it asks for.

import (
	"bytes"
	"testing"
	"unicode/utf8"
)

var (
	twoByte   = []byte("é")
	threeByte = []byte("汉")
	fourByte  = []byte("𐍈")
)

func TestIncompleteUTF8Tail(t *testing.T) {
	if len(twoByte) != 2 || len(threeByte) != 3 || len(fourByte) != 4 {
		t.Fatalf("fixtures are not 2, 3 and 4 bytes: %d %d %d",
			len(twoByte), len(threeByte), len(fourByte))
	}
	with := func(tail []byte) []byte { return append([]byte("abc"), tail...) }
	cases := map[string]struct {
		data []byte
		want int
	}{
		"empty":                    {[]byte{}, 0},
		"ascii":                    {[]byte("abc"), 0},
		"a whole 3-byte character": {with(threeByte), 0},
		"1 of 2 bytes":             {with(twoByte[:1]), 1},
		"1 of 3 bytes":             {with(threeByte[:1]), 1},
		"2 of 3 bytes":             {with(threeByte[:2]), 2},
		"1 of 4 bytes":             {with(fourByte[:1]), 1},
		"2 of 4 bytes":             {with(fourByte[:2]), 2},
		"3 of 4 bytes":             {with(fourByte[:3]), 3},
		"stray continuation bytes": {[]byte{0x80, 0x80, 0x80, 0x80}, 0},
	}
	for name, c := range cases {
		if got := incompleteUTF8Tail(c.data); got != c.want {
			t.Errorf("%s: got %d, want %d", name, got, c.want)
		}
	}
}

// realign is the stream loop's arithmetic, so the test exercises the same
// carry-and-prepend it does without needing a server.
func realign(pieces [][]byte) [][]byte {
	var out [][]byte
	var carry []byte
	for i, piece := range pieces {
		data := piece
		if len(carry) > 0 {
			data = append(append([]byte(nil), carry...), data...)
			carry = nil
		}
		if i != len(pieces)-1 {
			if held := incompleteUTF8Tail(data); held > 0 {
				carry = append([]byte(nil), data[len(data)-held:]...)
				data = data[:len(data)-held]
			}
		}
		out = append(out, data)
	}
	return out
}

// TestRealignmentAtEveryCut puts the split at every byte of a string full of
// multi-byte characters: each chunk must be valid UTF-8 on its own and the
// join must be the original bytes.
func TestRealignmentAtEveryCut(t *testing.T) {
	whole := []byte("before 汉 and é and 𐍈 after")
	for cut := 1; cut < len(whole); cut++ {
		emitted := realign([][]byte{whole[:cut], whole[cut:]})
		var joined []byte
		for _, chunk := range emitted {
			if !utf8.Valid(chunk) {
				t.Fatalf("cut %d: a chunk is not valid UTF-8 on its own: %q", cut, chunk)
			}
			joined = append(joined, chunk...)
		}
		if !bytes.Equal(joined, whole) {
			t.Fatalf("cut %d: the bytes changed: %q", cut, joined)
		}
	}
}

// TestRealignmentOneBytePerChunk is the worst case for the carry.
func TestRealignmentOneBytePerChunk(t *testing.T) {
	whole := []byte("a汉b é c 𐍈")
	pieces := make([][]byte, 0, len(whole))
	for i := range whole {
		pieces = append(pieces, whole[i:i+1])
	}
	var joined []byte
	for _, chunk := range realign(pieces) {
		if !utf8.Valid(chunk) {
			t.Fatalf("a chunk is not valid UTF-8 on its own: %q", chunk)
		}
		joined = append(joined, chunk...)
	}
	if !bytes.Equal(joined, whole) {
		t.Fatalf("the bytes changed: %q", joined)
	}
}

// TestRealignmentKeepsAMalformedTail: a stream that really ends mid-character
// still delivers those bytes on the final chunk rather than swallowing them.
func TestRealignmentKeepsAMalformedTail(t *testing.T) {
	whole := append([]byte("tail "), threeByte[:2]...)
	var joined []byte
	for _, chunk := range realign([][]byte{whole}) {
		joined = append(joined, chunk...)
	}
	if !bytes.Equal(joined, whole) {
		t.Fatalf("bytes were dropped: %q", joined)
	}
	if utf8.Valid(joined) {
		t.Fatal("the fixture was supposed to be malformed")
	}
}

// TestRealignmentLeavesAsciiAlone is the control: with nothing to hold back the
// chunks arrive exactly as the server sent them.
func TestRealignmentLeavesAsciiAlone(t *testing.T) {
	pieces := [][]byte{[]byte(`{"a":1}` + "\n"), []byte(`{"b":2}` + "\n")}
	out := realign(pieces)
	for i := range pieces {
		if !bytes.Equal(out[i], pieces[i]) {
			t.Errorf("chunk %d changed: %q -> %q", i, pieces[i], out[i])
		}
	}
}
