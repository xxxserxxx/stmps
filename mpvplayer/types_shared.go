// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package mpvplayer

// StatusData is a player progress report for the UI
type StatusData struct {
	Volume   int64
	Position int64
	Duration int64
}
