// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"

	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/spezifisch/stmps/subsonic"
)

func makeModal(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewGrid().
		SetColumns(0, width, 0).
		SetRows(0, height, 0).
		AddItem(p, 1, 1, 1, 1, 0, 0, true)
}

func formatPlayerStatus(scanning bool, volume int64, position int64, duration int64) string {
	if position < 0 {
		position = 0
	}

	if duration < 0 {
		duration = 0
	}

	positionMin, positionSec := secondsToMinAndSec(position)
	durationMin, durationSec := secondsToMinAndSec(duration)

	st := "( )"
	if scanning {
		st = "[green](S)[-]"
	}

	return fmt.Sprintf("%s[%d%%][::b][%02d:%02d/%02d:%02d]", st, volume, positionMin, positionSec, durationMin, durationSec)
}

func formatSongForStatusBar(currentSong *mpvplayer.QueueItem) (text string) {
	if currentSong == nil {
		return
	}
	if currentSong.Title != "" {
		text += "[::-] [white]" + tview.Escape(currentSong.Title)
	}
	if currentSong.Artist != "" {
		text += " [gray]by [white]" + tview.Escape(currentSong.Artist)
	}
	return
}

func formatSongForPlaylistEntry(entity subsonic.Entity) (text string) {
	if entity.Title != "" {
		text += "[::-] [white]" + tview.Escape(entity.Title)
	}
	if entity.Artist != "" {
		text += " [gray]by [white]" + tview.Escape(entity.Artist)
	}
	return
}
