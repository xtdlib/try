// Copyright 2022, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// Package try emulates aspects of the ill-fated "try" proposal using generics.
// See https://golang.org/issue/32437 for inspiration.
//
// Example usage:
//
//	func Fizz(...) (..., err error) {
//		defer try.HandleF(&err, func() {
//			if err == io.EOF {
//				err = io.ErrUnexpectedEOF
//			}
//		})
//		... := try.E2(Buzz(...))
//		return ..., nil
//	}
//
// This package is a sharp tool and should be used with care.
// Quick and easy error handling can occlude critical error handling logic.
// Panic handling generally should not cross package boundaries or be an explicit part of an API.
//
// Package try is a good fit for short Go programs and unit tests where
// development speed is a greater priority than reliability.
// Since the E functions panic if an error is encountered, recovering in such programs is optional.
//
// Code before try:
//
//	func (a *MixedArray) UnmarshalNext(uo json.UnmarshalOptions, d *json.Decoder) error {
//		switch t, err := d.ReadToken(); {
//		case err != nil:
//			return err
//		case t.Kind() != '[':
//			return fmt.Errorf("got %v, expecting array start", t.Kind())
//		}
//
//		if err := uo.UnmarshalNext(d, &a.Scalar); err != nil {
//			return err
//		}
//		if err := uo.UnmarshalNext(d, &a.Slice); err != nil {
//			return err
//		}
//		if err := uo.UnmarshalNext(d, &a.Map); err != nil {
//			return err
//		}
//
//		switch t, err := d.ReadToken(); {
//		case err != nil:
//			return err
//		case t.Kind() != ']':
//			return fmt.Errorf("got %v, expecting array end", t.Kind())
//		}
//		return nil
//	}
//
// Code after try:
//
//	func (a *MixedArray) UnmarshalNext(uo json.UnmarshalOptions, d *json.Decoder) (err error) {
//		defer try.Handle(&err)
//		if t := try.E1(d.ReadToken()); t.Kind() != '[' {
//			return fmt.Errorf("found %v, expecting array start", t.Kind())
//		}
//		try.E(uo.UnmarshalNext(d, &a.Scalar))
//		try.E(uo.UnmarshalNext(d, &a.Slice))
//		try.E(uo.UnmarshalNext(d, &a.Map))
//		if t := try.E1(d.ReadToken()); t.Kind() != ']' {
//			return fmt.Errorf("found %v, expecting array end", t.Kind())
//		}
//		return nil
//	}
//
// # Quick tour of the API
//
// The E family of functions all remove a final error return, panicking if non-nil.
//
// Handle recovers from that panic and allows assignment of the error to a return
// error value. Other panics are not recovered.
//
//	func f() (err error) {
//		defer try.Handle(&err)
//		...
//	}
//
// HandleF is like Handle, but it calls a function after any such assignment.
//
//	func f() (err error) {
//		defer try.HandleF(&err, func() {
//			if err == io.EOF {
//				err = io.ErrUnexpectedEOF
//			}
//		})
//		...
//	}
//
//	func foo(i int) (err error) {
//		defer try.HandleF(&err, func() {
//			err = fmt.Errorf("unable to foo %d: %w", i, err)
//		})
//		...
//	}
//
// F wraps an error with file and line information and calls a function on error.
// It inter-operates well with testing.TB and log.Fatal.
//
//	func TestFoo(t *testing.T) {
//		defer try.F(t.Fatal)
//		...
//	}
//
//	func main() {
//		defer try.F(log.Fatal)
//		...
//	}
//
// Recover is like F, but it supports more complicated error handling
// by passing the error and runtime frame directly to a function.
//
//	func f() {
//		defer try.Recover(func(err error, frame runtime.Frame) {
//			// do something useful with err and frame
//		})
//		...
//	}
package try

import (
	"context"
	"fmt"
	"runtime"
	"strconv"

	"log/slog"
)

// wrapError wraps an error to ensure that we only recover from errors
// panicked by this package.
type wrapError struct {
	error
	pc [1]uintptr
}

func (e wrapError) Catch() error {
	return e.error
}

func (e wrapError) Error() string {
	// Retrieve the last path segment of the filename.
	// We avoid using strings.LastIndexByte to keep dependencies small.
	frames := runtime.CallersFrames(e.pc[:])
	frame, _ := frames.Next()
	file := frame.File
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			file = file[i+len("/"):]
			break
		}
	}
	return file + ":" + strconv.Itoa(frame.Line) + ": " + e.error.Error()
}

// Unwrap primarily exists for testing purposes.
func (e wrapError) Unwrap() error {
	return e.error
}

func r(recovered any, fn func(wrapError)) {
	switch ex := recovered.(type) {
	case nil:
	case wrapError:
		slog.Default().Debug("try: caught: " + ex.Error())
		fn(ex)
	default:
		panic(ex)
	}
}

// Recover recovers an error previously panicked with an E function.
// If it recovers an error, it calls fn with the error and the runtime frame in which it occurred.
// func Recover(fn func(err error, frame runtime.Frame)) {
// 	r(recover(), func(w WrapError) {
// 		frames := runtime.CallersFrames(w.pc[:])
// 		frame, _ := frames.Next()
// 		fn(w.error, frame)
// 	})
// }

// Catch recovers an error previously panicked with an E function and stores it into errptr.
func Catch[T error](errptr *T) {
	// 	r(recover(), func(w WrapError) { *errptr = w.error })
	r(recover(), func(w wrapError) {
		if e, ok := w.error.(T); ok {
			*errptr = e
		} else {
			panic("try: caught error of wrong type: " + w.error.Error())
		}
	})
}

// CatchF recovers an error previously panicked with an E function and stores it into errptr.
// If it recovers an error, it calls fn.
func CatchF[T error](errptr *T, fn func()) {
	// r(recover(), func(w WrapError) {
	// 	*errptr = w.error
	// 	if w.error != nil {
	// 		fn()
	// 	}
	// })

	r(recover(), func(w wrapError) {
		if _, ok := w.error.(T); ok {
			*errptr = w.error.(T)
			if w.error != nil {
				fn()
			}
		} else {
			panic("try: caught error of wrong type: " + w.error.Error())
		}
	})

}

// F recovers an error previously panicked with an E function, wraps it, and passes it to fn.
// The wrapping includes the file and line of the runtime frame in which it occurred.
// F pairs well with testing.TB.Fatal and log.Fatal.
func F(fn func(...any)) {
	r(recover(), func(w wrapError) { f(fn, w) })
}

func e(err error) {
	we := wrapError{error: err}
	// 3: runtime.Callers, e, E
	runtime.Callers(3, we.pc[:])
	panic(we)
}

// E panics if err is non-nil.
func E(err error) {
	if err != nil {
		e(err)
	}
}

// E1 returns a as is.
// It panics if err is non-nil.
func E1[A any](a A, err error) A {
	if err != nil {
		e(err)
	}
	return a
}

// E2 returns a and b as is.
// It panics if err is non-nil.
func E2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		e(err)
	}
	return a, b
}

// E3 returns a, b, and c as is.
// It panics if err is non-nil.
func E3[A, B, C any](a A, b B, c C, err error) (A, B, C) {
	if err != nil {
		e(err)
	}
	return a, b, c
}

// E4 returns a, b, c, and d as is.
// It panics if err is non-nil.
func E4[A, B, C, D any](a A, b B, c C, d D, err error) (A, B, C, D) {
	if err != nil {
		e(err)
	}
	return a, b, c, d
}

// f simply calls fn with w.
//
// This uses the special "line" pragma to set the file and line number to be
// something consistent. It must be declared last in the file to prevent "line"
// from affecting the line numbers of anything else in this file.
func f(fn func(...any), w wrapError) {
//line try.go:1
	fn(w)
}

// L logs error if err is non-nil.
func L(err error) {
	if err != nil {
		slog.Default().Log(context.Background(), slog.LevelError, err.Error())
	}
}

func L1[A any](a A, err error) A {
	if err != nil {
		slog.Default().Log(context.Background(), slog.LevelError, err.Error())
	}
	return a
}

func L2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		slog.Default().Log(context.Background(), slog.LevelError, err.Error())
	}
	return a, b
}

func L3[A, B, C any](a A, b B, c C, err error) (A, B, C) {
	if err != nil {
		slog.Default().Log(context.Background(), slog.LevelError, err.Error())
	}
	return a, b, c
}

func L4[A, B, C, D any](a A, b B, c C, d D, err error) (A, B, C, D) {
	if err != nil {
		slog.Default().Log(context.Background(), slog.LevelError, err.Error())
	}
	return a, b, c, d
}

// M panics if err is non-nil.
func M(err error) {
	if err != nil {
		panic(err)
	}
}

func M1[A any](a A, err error) A {
	if err != nil {
		panic(err)
	}
	return a
}

func M2[A, B any](a A, b B, err error) (A, B) {
	if err != nil {
		panic(err)
	}
	return a, b
}

func M3[A, B, C any](a A, b B, c C, err error) (A, B, C) {
	if err != nil {
		panic(err)
	}
	return a, b, c
}

func M4[A, B, C, D any](a A, b B, c C, d D, err error) (A, B, C, D) {
	if err != nil {
		panic(err)
	}
	return a, b, c, d
}

func Equal[T comparable](a, b T) {
	if a != b {
		e(fmt.Errorf("try: not equal: %+v != %+v", a, b))
	}
}

func NotEqual[T comparable](a, b T) {
	if a == b {
		e(fmt.Errorf("try: equal: %+v == %+v", a, b))
	}
}

func Zero[T comparable](a T) {
	var b T
	if a != b {
		e(fmt.Errorf("try: zero: %+v == %+v", a, b))
	}
}

func NotZero[T comparable](a T) {
	var b T
	if a == b {
		e(fmt.Errorf("try: zero: %+v == %+v", a, b))
	}
}
