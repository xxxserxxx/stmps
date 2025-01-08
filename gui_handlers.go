// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/spezifisch/stmps/subsonic"
)

func (ui *Ui) handlePageInput(event *tcell.EventKey) *tcell.EventKey {
	// we don't want any of these firing if we're trying to add a new playlist
	focused := ui.app.GetFocus()
	if ui.playlistPage.IsNewPlaylistInputFocused(focused) || ui.browserPage.IsSearchFocused(focused) || focused == ui.searchPage.searchField || ui.selectPlaylistWidget.visible {
		return event
	}

	switch event.Rune() {
	case '1':
		ui.ShowPage(PageBrowser)

	case '2':
		ui.ShowPage(PageQueue)

	case '3':
		ui.ShowPage(PagePlaylists)

	case '4':
		ui.ShowPage(PageSearch)

	case '5':
		ui.ShowPage(PageLog)

	case '?':
		ui.ShowHelp()

	case 'Q':
		ui.Quit()

	case 'r':
		// add random songs to queue
		ui.handleAddRandomSongs("")

	case 'D':
		// clear queue and stop playing
		ui.player.ClearQueue()
		ui.queuePage.UpdateQueue()

	case 'p':
		// toggle playing/pause
		err := ui.player.Pause()
		if err != nil {
			ui.logger.PrintError("handlePageInput: Pause", err)
		}

	case 'P':
		// stop playing without changes to queue
		ui.logger.Print("key stop")
		err := ui.player.Stop()
		if err != nil {
			ui.logger.PrintError("handlePageInput: Stop", err)
		}

	case 'X':
		// debug stuff
		ui.logger.Print("test")
		//ui.player.Test()
		ui.showMessageBox("foo bar")

	case '-':
		// volume-
		if err := ui.player.AdjustVolume(-5); err != nil {
			ui.logger.PrintError("handlePageInput: AdjustVolume-", err)
		}

	case '+', '=':
		// volume+
		if err := ui.player.AdjustVolume(5); err != nil {
			ui.logger.PrintError("handlePageInput: AdjustVolume+", err)
		}

	case '.':
		// <<
		if err := ui.player.Seek(10); err != nil {
			ui.logger.PrintError("handlePageInput: Seek+", err)
		}

	case ',':
		// >>
		if err := ui.player.Seek(-10); err != nil {
			ui.logger.PrintError("handlePageInput: Seek-", err)
		}

	case '>':
		// skip to next track
		if err := ui.player.PlayNextTrack(); err != nil {
			ui.logger.PrintError("handlePageInput: Next", err)
		}
		ui.queuePage.UpdateQueue()

	case 'c':
		ui.logger.Printf("info: starting server scan")
		ui.scanning = true
		if err := ui.connection.StartScan(); err != nil {
			ui.logger.PrintError("startScan:", err)
		}

	default:
		return event
	}

	return nil
}

func (ui *Ui) ShowPage(name string) {
	ui.pages.SwitchToPage(name)
	ui.menuWidget.SetActivePage(name)
	_, prim := ui.pages.GetFrontPage()
	ui.app.SetFocus(prim)
}

func (ui *Ui) Quit() {
	if len(ui.queuePage.queueData.playerQueue) > 0 {
		ids := make([]string, len(ui.queuePage.queueData.playerQueue))
		for i, it := range ui.queuePage.queueData.playerQueue {
			ids[i] = it.Id
		}
		// stmps always only ever plays the first song in the queue
		pos := ui.player.GetTimePos()
		if err := ui.connection.SavePlayQueue(ids, ids[0], int(pos)); err != nil {
			log.Printf("error stashing play queue: %s", err)
		}
	} else {
		// The only way to purge a saved play queue is to force an error by providing
		// bad data. Therefore, we ignore errors.
		_ = ui.connection.SavePlayQueue([]string{"XXX"}, "XXX", 0)
	}
	ui.player.Quit()
	ui.app.Stop()
}

func (ui *Ui) handleAddRandomSongs(id string) {
	ui.addRandomSongsToQueue(id)
	ui.queuePage.UpdateQueue()
}

func (ui *Ui) addRandomSongsToQueue(id string) {
	entities, err := ui.connection.GetRandomSongs(id)
	if err != nil {
		ui.logger.Printf("addRandomSongsToQueue %s", err.Error())
	}
	for _, e := range entities {
		ui.addSongToQueue(e)
	}
}

// make sure to call ui.QueuePage.UpdateQueue() after this
func (ui *Ui) addSongToQueue(entity subsonic.Entity) {
	ui.logger.Printf("debug addSongToQueue %s", entity)
	uri := ui.connection.GetPlayUrl(entity)

	album, err := ui.connection.GetAlbum(entity.Parent)
	albumName := ""
	if err != nil {
		ui.logger.PrintError("addSongToQueue", err)
	} else {
		switch {
		case album.Name != "":
			albumName = album.Name
		case album.Title != "":
			albumName = album.Title
		case album.Album != "":
			albumName = album.Album
		}
	}

	// Populate the genre, by hook or crook
	genre := entity.Genre
	if genre == "" {
		genre = album.Genre
	}
	if genre == "" && len(album.Genres) > 0 {
		genre = album.Genres[0].Name
	}

	queueItem := &mpvplayer.QueueItem{
		Id:          entity.Id,
		Uri:         uri,
		Title:       entity.GetSongTitle(),
		Artist:      entity.Artist,
		Duration:    entity.Duration,
		Album:       albumName,
		TrackNumber: entity.Track,
		CoverArtId:  entity.CoverArtId,
		DiscNumber:  entity.DiscNumber,
		Year:        entity.Year,
		Genre:       genre,
	}
	ui.player.AddToQueue(queueItem)
}

func (ui *Ui) makeSongHandler(entity subsonic.Entity) func() {
	return func() {
		uri := ui.connection.GetPlayUrl(entity)
		if err := ui.player.PlayUri(uri, entity.CoverArtId, entity); err != nil {
			ui.logger.PrintError("SongHandler Play", err)
			return
		}
		ui.queuePage.UpdateQueue()
	}
}
