package testutil

import (
	"bytes"
	"io"
)

func ioBody(v []byte) io.ReadCloser { return io.NopCloser(bytes.NewReader(v)) }
