// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
 Package gexpect provides a go API to Don Libe's expect C library.
 Expect is a tcl extension, documented and available from:
 http:www.nist.gov/el/msid/expect.cfm


 Installation:

 expect and tcl are not always preinstalled so you may need to
 build+install them yourself. In addition, cgo only work with
 the shared linker (it generates a C file with #pragma's to import
 the symbols and libraries referenced) so you will need expect/tcl to be
 available as shared libraries.

 On Mac OS, I had to build expect specifically to enable shared libraries
 as follows:

 ./configure --enable-shared --enable-symbols --enable-64bit

 The dynamic linker on your system may need to be told where to find 
 the shared library for expect. For example on Mac OS you need to set
 DYLD_LIBRARY_PATH.
*/
package expect

/*
#cgo darwin LDFLAGS: -L/usr/local/lib/expect5.45 -lexpect5.45 -ltcl
#include <stdlib.h>
#include <tcl.h>
#include <expect_tcl.h>
#include <expect.h>
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

// Represents a running process created by Spawn. File
// can be used to write to stdin, and read the unbuffered output of
// stdout and stderr of the process.
type Process struct {
	Fd   int
	Pid  int
	File *os.File
}

// Expectl takes an arbitrary number of patterns as arguments
// and returns either the code specified for a pattern that matches
// or one of EXP_TIMEOUT, EXP_FULLBUFFER or EXP_EOF
type Pattern struct {
	Type    int    // one of Glob, Exact, Regexp or Null
	Pattern string // the pattern itself
	Value   uint   // a non-negative int to be returned on a match
}

const (
	// Pattern types, see the libexpect man page for an explanation.
	_End     int = 0 // exp_end is not needed in the go API
	Glob     int = 1 // exp_glob
	Exact    int = 2 // exp_exact
	RegExp   int = 3 // exp_regexp
	Compiled int = 4 // exp_compiled is currently not supported and will result in an error if used
	Null     int = 5 // exp_null
)

const (
	// The return code from Expectl is either a positive int indicating the
	// pattern that was matched, or one of these negative values.
	ERROR      int = -1
	TIMEOUT    int = -2
	FULLBUFFER int = -5
	EOF        int = -11
)

// If Pattern is non-nil, then it contains a badly specified pattern.
// This will only be set by Expectl and when set, the other fields have no
// meaning.
// Err contains a system error (i.e. Errno) and can be set by either
// Spawn or Expectl.
type ExpectError struct {
	Pattern *Pattern
	Err     error
}

func (e *ExpectError) Error() string {
	if e.Pattern == nil {
		return e.Err.Error()
	}
	return "Bad Pattern: " + fmt.Sprintf("%v", e.Pattern)
}

func SetTimeout(timeout int) {
	C.exp_timeout = C.int(timeout)
}

func SetDebugging(debug bool) {
	if debug {
		C.exp_is_debugging = 1
	} else {
		C.exp_is_debugging = 0
	}
}

func LogToConsole(log bool) {
	if log {
		C.exp_loguser = 1
	} else {
		C.exp_loguser = 0
	}
}

// Spawn a process whose executable is given by file and with the
// command line arguments specified by args.
func Spawn(file string, args ...string) (p *Process, e *ExpectError) {
	cfile := C.CString(file)
	l := len(args) + 2
	cargv := make([]*C.char, l, l)
	cargv[0] = C.CString(file)
	for i, v := range args {
		cargv[i+1] = C.CString(v)
	}
	cargv[l-1] = nil
	fd, errno := C.exp_spawnv(cfile, &cargv[0])
	for i, _ := range cargv {
		C.free(unsafe.Pointer(cargv[i]))
	}
	C.free(unsafe.Pointer(cfile))
	if errno != nil {
		return &Process{Fd: -1}, &ExpectError{Err: errno}
	}
	return &Process{Fd: int(fd),
			Pid:  int(C.exp_pid),
			File: os.NewFile(uintptr(fd), file)},
		nil
}

// Process the output from a previously spawned command using the
// expect engine. Provide one or more patterns as per typical expect
// processing except that there is no need to specify the 'end' pattern
// and that precompiled tcl regexps are not currently supported.
func (p *Process) Expectl(patterns ...Pattern) (
	returncode int, e *ExpectError) {
	// Valide the patterns and create the equivalent exp_case ones.
	for _, v := range patterns {
		switch v.Type {
		case Glob:
		case Exact:
		case RegExp:
		default:
			return ERROR, &ExpectError{Pattern: &v}
		}
	}
	numPatterns := len(patterns) + 1
	ecases := make([]C.struct_exp_case, numPatterns, numPatterns)
	for i, v := range patterns {
		// Carefully avoid using field names since one of them
		// is called 'type' which is a reserved word in go.
		ecases[i] = C.struct_exp_case{
			C.CString(v.Pattern),
			nil,
			uint32(v.Type),
			C.int(v.Value)}
	}
	ecases[numPatterns-1] = C.struct_exp_case{
		nil, nil, uint32(_End), 0}
	value, errno := C.exp_expectv(C.int(p.Fd), &ecases[0])
	for _, v := range ecases {
		C.free(unsafe.Pointer(v.pattern))
	}
	if errno != nil {
		return ERROR, &ExpectError{Err: errno}
	}
	return int(value), nil
}

func (p *Process) KillChild() *ExpectError {
	proc, e := os.FindProcess(p.Pid)
	if e != nil {
		return &ExpectError{Err: e}
	}
	if e = proc.Kill(); e != nil {
		return &ExpectError{Err: e}
	}
	return nil
}

func init() {
	// The expect library is built, by default, to use the tcl stub
	// API. So far as I can tell this means that any libexpect user
	// must initalize the stubs as below.
	C.Expect_Init(C.Tcl_CreateInterp())
}
