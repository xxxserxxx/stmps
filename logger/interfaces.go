// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package logger

type LoggerInterface interface {
	Print(s string)
	Printf(s string, as ...interface{})
	PrintError(source string, err error)
}
