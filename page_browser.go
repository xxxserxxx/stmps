// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/wildeyedskies/stmp/subsonic"
)

func (ui *Ui) createBrowserPage(indexes *[]subsonic.SubsonicIndex) (*tview.Flex, tview.Primitive) {
	// artist list
	ui.artistList = tview.NewList().
		ShowSecondaryText(false)
	ui.artistList.Box.
		SetTitle(" artist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	for _, index := range *indexes {
		for _, artist := range index.Artists {
			ui.artistList.AddItem(artist.Name, "", 0, nil)
			ui.artistIdList = append(ui.artistIdList, artist.Id)
		}
	}

	// album list
	ui.entityList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	ui.entityList.Box.
		SetTitle(" album ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	ui.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetChangedFunc(func(s string) {
			idxs := ui.artistList.FindItems(s, "", false, true)
			if len(idxs) == 0 {
				return
			}
			ui.artistList.SetCurrentItem(idxs[0])
		}).
		SetDoneFunc(func(key tcell.Key) {
			ui.app.SetFocus(ui.artistList)
		})

	artistFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.artistList, 0, 1, true).
		AddItem(ui.entityList, 0, 1, false)

	browserFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(artistFlex, 0, 1, true).
		AddItem(ui.searchField, 1, 0, false)

	// going right from the artist list should focus the album/song list
	ui.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(ui.entityList)
			return nil
		}
		switch event.Rune() {
		case '/':
			ui.search()
			return nil
		case 'n':
			ui.searchNext()
			return nil
		case 'N':
			ui.searchPrev()
			return nil
		case 'R':
			goBackTo := ui.artistList.GetCurrentItem()
			// REFRESH artists
			indexResponse, err := ui.connection.GetIndexes()
			if err != nil {
				ui.logger.Printf("Error fetching indexes from server: %s\n", err)
				return event
			}
			ui.artistList.Clear()
			ui.connection.ClearCache()
			for _, index := range indexResponse.Indexes.Index {
				for _, artist := range index.Artists {
					ui.artistList.AddItem(artist.Name, "", 0, nil)
					ui.artistIdList = append(ui.artistIdList, artist.Id)
				}
			}
			// Try to put the user to about where they were
			if goBackTo < ui.artistList.GetItemCount() {
				ui.artistList.SetCurrentItem(goBackTo)
			}
		}
		return event
	})

	ui.artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		ui.handleEntitySelected(ui.artistIdList[index])
	})

	// "add to playlist" modal
	for _, playlist := range ui.playlists {
		ui.addToPlaylistList.AddItem(playlist.Name, "", 0, nil)
	}
	ui.addToPlaylistList.SetBorder(true).
		SetTitle("Add to Playlist")

	addToPlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(ui.addToPlaylistList, 0, 1, true)

	addToPlaylistModal := makeModal(addToPlaylistFlex, 60, 20)

	ui.addToPlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(ui.entityList)
		} else if event.Key() == tcell.KeyEnter {
			playlist := ui.playlists[ui.addToPlaylistList.GetCurrentItem()]
			ui.handleAddSongToPlaylist(&playlist)

			ui.pages.HidePage("addToPlaylist")
			ui.pages.SwitchToPage("browser")
			ui.app.SetFocus(ui.entityList)
		}
		return event
	})

	ui.entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(ui.artistList)
			return nil
		}
		if event.Rune() == 'a' {
			ui.handleAddEntityToQueue()
			return nil
		}
		if event.Rune() == 'y' {
			ui.handleToggleEntityStar()
			return nil
		}
		// only makes sense to add to a playlist if there are playlists
		if event.Rune() == 'A' {
			if ui.playlistList.GetItemCount() > 0 {
				ui.pages.ShowPage("addToPlaylist")
				ui.app.SetFocus(ui.addToPlaylistList)
			} else {
				ui.showMessageBox("No playlists available. Create one first.")
			}
			return nil
		}
		// REFRESH only the artist
		if event.Rune() == 'r' {
			artistIdx := ui.artistList.GetCurrentItem()
			entity := ui.artistIdList[artistIdx]
			//ui.logger.Printf("refreshing artist idx %d, entity %s (%s)", artistIdx, entity, ui.connection.directoryCache[entity].Directory.Name)
			ui.connection.RemoveCacheEntry(entity)
			ui.handleEntitySelected(ui.artistIdList[artistIdx])
			return nil
		}
		return event
	})

	return browserFlex, addToPlaylistModal
}
