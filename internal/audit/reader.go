package audit

import (
	"bytes"
	"io"
)

// newLineReader hands the decoder the file as a stream. A truncated final line
// — the process was killed mid-write — ends decoding without discarding
// everything before it.
func newLineReader(b []byte) io.Reader { return bytes.NewReader(b) }
