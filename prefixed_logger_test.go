package main

import (
	"bytes"
	"testing"
)

func TestPrefixedLogger(t *testing.T) {
	var (
		buf = bytes.NewBuffer([]byte{})
		pl  = newPrefixedLogger(buf, "baum")
		n   int
		err error
	)

	n, err = pl.Write([]byte("non-newline terminated string"))
	if n != 29 || err != nil {
		t.Fatalf("Write to prefixedLogger had unexpected results: n=29 != %d, err=nil != %s", n, err)
	}

	if n = len(buf.Bytes()); n != 0 {
		t.Fatalf("Buffer contains %d characters, should contain 0", n)
	}

	pl.Write([]byte("now a newline\nand something"))
	if n = len(buf.Bytes()); n != 50 {
		t.Fatalf("Buffer contains %d characters, should contain 50", n)
	}

	pl.Write([]byte(" more to log\nwith multiple\nnewlines\n"))
	if n = len(buf.Bytes()); n != 120 {
		t.Fatalf("Buffer contains %d characters, should contain 120", n)
	}
}
