// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 Unit tests for gexpect, the intent is not to test expect itself,
 but rather just the go binding to it.
*/
package expect

import (
	"fmt"
	"testing"
)

func TestBadSpawn(t *testing.T) {
	_, e := Spawn("/bin/echox", "hello", "world")
	if e == nil {
		t.Error("No error detected")
	}
	if e.Error() != "no such file or directory" {
		t.Error("Wrong error detected: ", e)
	}
}

func TestBadPattern(t *testing.T) {
	p, _ := Spawn("/bin/echo", "hello", "world")
	_, e := p.Expectl(Pattern{44, "bad pattern", 2})
	if e == nil {
		t.Error("Failed: to detect bad pattern")
	}
	if e.Error() != "Bad Pattern: &{44 bad pattern 2}" {
		t.Error("Wrong error detected: ", e)
	}
}

func testHelloWorld(t *testing.T, output string) int {
	p, e := Spawn("/bin/echo", "hello", "world")
	if e != nil {
		t.Error("Failed: ", e)
	}
	v, e := p.Expectl(Pattern{Exact, output, 1})
	if e != nil {
		t.Error("Failed: ", e)
	}
	return v

}

func TestHelloWorld(t *testing.T) {
	if testHelloWorld(t, "hello world\r\n") != 1 {
		t.Error("Failed to match input")
	}
	if testHelloWorld(t, "xhello world") == 1 {
		t.Error("Incorrectly matched input")
	}
}

func spawnSed(t *testing.T) *Process {
	p, e := Spawn("sed")
	if e != nil {
		t.Error("Failed: ", e)
	}
	return p
}

func expectSed(t *testing.T, p *Process, patterns ...Pattern) int {
	v, e := p.Expectl(patterns...)
	if e != nil {
		t.Error("Failed: ", e)
	}
	return v
}

func TestSingleInteraction(t *testing.T) {
	p := spawnSed(t)
	fmt.Fprintf(p.File, "here's some input for you to match on\n")
	v := expectSed(t, p, Pattern{Glob, "input", 1})
	if v != 1 {
		t.Error("Failed to match input: ", v)
	}
}

func TestMultipleInteractions(t *testing.T) {
	p := spawnSed(t)
	fmt.Fprintf(p.File, "here's some input for you to match on\n")
	fmt.Fprintf(p.File, "oh and more\n")
	v := expectSed(t, p, Pattern{Glob, "more", 1})
	if v != 1 {
		t.Error("Failed to match input: ", v)
	}
	fmt.Fprintf(p.File, "Mary had a little lamb\n")
	fmt.Fprintf(p.File, "Really?\n")
	fmt.Fprintf(p.File, "Or maybe it was a goat?\n")
	v = expectSed(t, p, Pattern{Glob, "goat", 2})
	if v != 2 {
		t.Error("Failed to match input: ", v)
	}
	fmt.Fprintf(p.File, "At last\n")
	v = expectSed(t, p, Pattern{Exact, "At last", 3})
	if v != 3 {
		t.Error("Failed to match input: ", v)
	}
}

func TestMultiplePatternsAndInteractions(t *testing.T) {
	p := spawnSed(t)
	patterns := []Pattern{
		Pattern{Exact, "a response", 1},
		Pattern{Glob, "wombats", 2},
		Pattern{RegExp, "ABO.T", 3},
	}
	fmt.Fprintf(p.File, ">> a response\n")
	if expectSed(t, p, Pattern{Exact, ">>", 0}) != 0 {
		t.Error("Failed to match prompt")
	}
	v := expectSed(t, p, patterns...)
	if v != 1 {
		t.Error("Failed to match input: ", v)
	}
	fmt.Fprintf(p.File, ">> a response with wombats therein\n")
	if expectSed(t, p, Pattern{Exact, ">>", 0}) != 0 {
		t.Error("Failed to match prompt")
	}
	v = expectSed(t, p, patterns...)
	if v != 1 {
		t.Error("Failed to match input: ", v)
	}
	fmt.Fprintf(p.File, ">> another response with wombats therein\n")
	if expectSed(t, p, Pattern{Exact, ">>", 0}) != 0 {
		t.Error("Failed to match prompt")
	}
	v = expectSed(t, p, patterns...)
	if v != 2 {
		t.Error("Failed to match input: ", v)
	}
	fmt.Fprintf(p.File, ">> ABORT\n")
	if expectSed(t, p, Pattern{Exact, ">>", 0}) != 0 {
		t.Error("Failed to match prompt")
	}
	v = expectSed(t, p, patterns...)
	if v != 3 {
		t.Error("Failed to match input: ", v)
	}
}

func TestTimeout(t *testing.T) {
	p := spawnSed(t)
	SetTimeout(1) // let's only wait 1 seconds
	v := expectSed(t, p, Pattern{Glob, "input", 1})
	if v != TIMEOUT {
		t.Error("Failed to timeout")
	}
}

func TestKill(t *testing.T) {
	p := spawnSed(t)
	SetTimeout(1) // let's only wait 1 seconds
	p.KillChild()
	v := expectSed(t, p, Pattern{Glob, "input", 1})
	if v != EOF {
		t.Error("Failed to see EOF from a killed child")
	}
}

func TestClose(t *testing.T) {
	p := spawnSed(t)
	SetTimeout(1) // let's only wait 1 seconds
	p.File.Close()
	_, e := p.Expectl(Pattern{Glob, "input", 1})
	if e == nil || e.Error() != "bad file descriptor" {
		t.Error("Failed to see a bad file descripter on a closed stream")
	}
}

func init() {
	LogToConsole(false)
}
