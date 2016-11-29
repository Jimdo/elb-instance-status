package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type prefixedLogger struct {
	channel       string
	wrappedWriter io.Writer

	buffer     []byte
	bufferLock sync.Mutex
}

func newPrefixedLogger(wrappedWriter io.Writer, channel string) *prefixedLogger {
	return &prefixedLogger{
		channel:       channel,
		wrappedWriter: wrappedWriter,
		buffer:        []byte{},
	}
}

func (p *prefixedLogger) dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

func (p *prefixedLogger) Write(in []byte) (n int, err error) {
	p.bufferLock.Lock()
	defer p.bufferLock.Unlock()

	n = len(in)
	p.buffer = append(p.buffer, in...)

	for {
		if i := bytes.IndexByte(p.buffer, '\n'); i >= 0 {
			// We have a full newline-terminated line.
			fmt.Fprintf(p.wrappedWriter, "[%s] %s\n", p.channel, string(p.dropCR(p.buffer[0:i])))
			p.buffer = p.buffer[i+1 : len(p.buffer)]
		} else {
			break
		}
	}

	return
}
