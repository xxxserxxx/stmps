// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/wildeyedskies/stmp/mpvplayer"
	"github.com/wildeyedskies/stmp/subsonic"
)

func (ui *Ui) handlePageInput(event *tcell.EventKey) *tcell.EventKey {
	// we don't want any of these firing if we're trying to add a new playlist
	focused := ui.app.GetFocus()
	if focused == ui.newPlaylistInput || focused == ui.searchField {
		return event
	}

	switch event.Rune() {
	case '1':
		ui.pages.SwitchToPage("browser")
		ui.currentPage.SetText("Browser")

	case '2':
		ui.pages.SwitchToPage("queue")
		ui.currentPage.SetText("Queue")

	case '3':
		ui.pages.SwitchToPage("playlists")
		ui.currentPage.SetText("Playlists")

	case '4':
		ui.pages.SwitchToPage("log")
		ui.currentPage.SetText("Log")

	case 'Q':
		ui.player.Quit()
		ui.app.Stop()

	case 'r':
		// add random songs to queue
		ui.handleAddRandomSongs()

	case 'D':
		// clear queue and stop playing
		ui.player.ClearQueue()
		ui.updateQueue()

	case 'p':
		// toggle playing/pause
		err := ui.player.Pause()
		if err != nil {
			ui.logger.PrintError("handlePageInput: Pause", err)
		}
		return nil

	case 'P':
		// stop playing without changes to queue
		ui.logger.Print("key stop")
		err := ui.player.Stop()
		if err != nil {
			ui.logger.PrintError("handlePageInput: Stop", err)
		}
		return nil

	case 'X':
		// debug stuff
		ui.logger.Print("test")
		//ui.player.Test()
		ui.showMessageBox("foo bar")
		return nil

	case '-':
		// volume-
		if err := ui.player.AdjustVolume(-5); err != nil {
			ui.logger.PrintError("handlePageInput: AdjustVolume-", err)
		}
		return nil

	case '=':
		// volume+
		if err := ui.player.AdjustVolume(5); err != nil {
			ui.logger.PrintError("handlePageInput: AdjustVolume+", err)
		}
		return nil

	case '.':
		// <<
		if err := ui.player.Seek(10); err != nil {
			ui.logger.PrintError("handlePageInput: Seek+", err)
		}
		return nil

	case ',':
		// >>
		if err := ui.player.Seek(-10); err != nil {
			ui.logger.PrintError("handlePageInput: Seek-", err)
		}
		return nil

	case '>':
		// skip to next track
		if err := ui.player.PlayNextTrack(); err != nil {
			ui.logger.PrintError("handlePageInput: Next", err)
		}
		ui.updateQueue()
	}

	return event
}

func (ui *Ui) handleAddRandomSongs() {
	ui.addRandomSongsToQueue()
	ui.updateQueue()
}

func (ui *Ui) addRandomSongsToQueue() {
	response, err := ui.connection.GetRandomSongs()
	if err != nil {
		ui.logger.Printf("addRandomSongsToQueue %s", err.Error())
	}
	for _, e := range response.RandomSongs.Song {
		ui.addSongToQueue(&e)
	}
}

// make sure to call ui.QueuePage.UpdateQueue() after this
func (ui *Ui) addSongToQueue(entity *subsonic.SubsonicEntity) {
	uri := ui.connection.GetPlayUrl(entity)

	var artist string
	if ui.currentDirectory == nil {
		artist = entity.Artist
	} else {
		artist = stringOr(entity.Artist, ui.currentDirectory.Name)
	}

	var id = entity.Id

	queueItem := &mpvplayer.QueueItem{
		Id:       id,
		Uri:      uri,
		Title:    entity.GetSongTitle(),
		Artist:   artist,
		Duration: entity.Duration,
	}
	ui.player.AddToQueue(queueItem)
}

func makeSongHandler(id string, uri string, title string, artist string, duration int,
	ui *Ui) func() {
	return func() {
		if err := ui.player.Play(id, uri, title, artist, duration); err != nil {
			ui.logger.PrintError("SongHandler Play", err)
			return
		}
		ui.updateQueue()
	}
}
