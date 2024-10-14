// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import "math"

func secondsToMinAndSec(seconds int64) (int, int) {
	minutes := math.Floor(float64(seconds) / 60)
	remainingSeconds := int(seconds) % 60
	return int(minutes), remainingSeconds
}

func iSecondsToMinAndSec(seconds int) (int, int) {
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	return minutes, remainingSeconds
}
