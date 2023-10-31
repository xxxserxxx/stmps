// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-only

package logger

import "fmt"

type Logger struct {
	Prints chan string
}

func Init() *Logger {
	return &Logger{make(chan string, 100)}
}

func (l *Logger) Print(s string) {
	l.Prints <- s
}

func (l *Logger) Printf(s string, as ...interface{}) {
	l.Prints <- fmt.Sprintf(s, as...)
}

func (l *Logger) PrintError(source string, err error) {
	l.Printf("Error(%s) -> %s", source, err.Error())
}
