// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import "math"

// if the first argument isn't empty, return it, otherwise return the second
//
//nolint:golint,unused
func stringOr(firstChoice string, secondChoice string) string {
	// TODO stringOr is not used anymore, so commented out until removal.
	if firstChoice != "" {
		return firstChoice
	}
	return secondChoice
}

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
