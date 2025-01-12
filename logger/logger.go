// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package logger

import (
	"fmt"
	"io"
	"os"
)

// TODO (B) Support log levels github.com/d2g/logfilter
type Logger struct {
	Prints chan string
	fout   io.WriteCloser
}

func Init(file string) *Logger {
	l := Logger{
		Prints: make(chan string, 100),
	}
	if file != "" {
		var err error
		if l.fout, err = os.Create(file); err != nil {
			fmt.Printf("error opening requested log file %s\n", file)
		}
	}
	return &l
}

func (l *Logger) Print(s string) {
	if l.fout != nil {
		fmt.Fprintf(l.fout, "%s\n", s)
	}
	l.Prints <- s
}

func (l *Logger) Printf(s string, as ...interface{}) {
	if l.fout != nil {
		fmt.Fprintf(l.fout, s, as...)
		fmt.Fprintf(l.fout, "\n")
	}
	l.Prints <- fmt.Sprintf(s, as...)
}

func (l *Logger) PrintError(source string, err error) {
	if l.fout != nil {
		fmt.Fprintf(l.fout, "Error(%s) -> %s\n", source, err.Error())
	}
	l.Printf("Error(%s) -> %s", source, err.Error())
}

func (l *Logger) Close() error {
	if l.fout != nil {
		return l.fout.Close()
	}
	return nil
}
