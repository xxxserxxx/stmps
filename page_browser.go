// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"sort"

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
			ui.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
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
					ui.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
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
		ui.addToPlaylistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
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

func (ui *Ui) handleAddEntityToQueue() {
	currentIndex := ui.entityList.GetCurrentItem()
	if currentIndex < 0 {
		return
	}

	if currentIndex+1 < ui.entityList.GetItemCount() {
		ui.entityList.SetCurrentItem(currentIndex + 1)
	}

	// if we have a parent directory subtract 1 to account for the [..]
	// which would be index 0 in that case with index 1 being the first entity
	if ui.currentDirectory.Parent != "" {
		currentIndex--
	}

	if currentIndex == -1 || len(ui.currentDirectory.Entities) <= currentIndex {
		return
	}

	entity := ui.currentDirectory.Entities[currentIndex]

	if entity.IsDirectory {
		ui.addDirectoryToQueue(&entity)
	} else {
		ui.addSongToQueue(&entity)
	}

	ui.updateQueue()
}

func (ui *Ui) handleEntitySelected(directoryId string) {
	response, err := ui.connection.GetMusicDirectory(directoryId)
	if err != nil {
		ui.logger.Printf("handleEntitySelected: GetMusicDirectory %s -- %s", directoryId, err.Error())
	}
	sort.Sort(response.Directory.Entities)

	ui.currentDirectory = &response.Directory
	ui.entityList.Clear()
	if response.Directory.Parent != "" {
		// has parent entity
		ui.entityList.Box.SetTitle(" song ")
		ui.entityList.AddItem(tview.Escape("[..]"), "", 0,
			ui.makeEntityHandler(response.Directory.Parent))
	} else {
		// no parent
		ui.entityList.Box.SetTitle(" album ")
	}

	for _, entity := range response.Directory.Entities {
		var title string
		var id = entity.Id
		var handler func()
		if entity.IsDirectory {
			title = tview.Escape("[" + entity.Title + "]")
			handler = ui.makeEntityHandler(entity.Id)
		} else {
			title = entityListTextFormat(entity, ui.starIdList)
			handler = makeSongHandler(id, ui.connection.GetPlayUrl(&entity),
				title, stringOr(entity.Artist, response.Directory.Name),
				entity.Duration, ui)
		}

		ui.entityList.AddItem(tview.Escape(title), "", 0, handler)
	}
}

func (ui *Ui) makeEntityHandler(directoryId string) func() {
	return func() {
		ui.handleEntitySelected(directoryId)
	}
}

func (ui *Ui) handleToggleEntityStar() {
	currentIndex := ui.entityList.GetCurrentItem()
	if currentIndex < 0 {
		return
	}

	var entity = ui.currentDirectory.Entities[currentIndex-1]

	// If the song is already in the star list, remove it
	_, remove := ui.starIdList[entity.Id]

	if _, err := ui.connection.ToggleStar(entity.Id, ui.starIdList); err != nil {
		ui.logger.PrintError("ToggleStar", err)
		return
	}

	if remove {
		delete(ui.starIdList, entity.Id)
	} else {
		ui.starIdList[entity.Id] = struct{}{}
	}

	var text = entityListTextFormat(entity, ui.starIdList)
	updateEntityListItem(ui.entityList, currentIndex, text)
	ui.updateQueue()
}

func entityListTextFormat(queueItem subsonic.SubsonicEntity, starredItems map[string]struct{}) string {
	var star = ""
	_, hasStar := starredItems[queueItem.Id]
	if hasStar {
		star = " [red]♥"
	}
	return queueItem.Title + star
}

// Just update the text of a specific row
func updateEntityListItem(entityList *tview.List, id int, text string) {
	entityList.SetItemText(id, text, "")
}

func (ui *Ui) addDirectoryToQueue(entity *subsonic.SubsonicEntity) {
	response, err := ui.connection.GetMusicDirectory(entity.Id)
	if err != nil {
		ui.logger.Printf("addDirectoryToQueue: GetMusicDirectory %s -- %s", entity.Id, err.Error())
		return
	}

	sort.Sort(response.Directory.Entities)
	for _, e := range response.Directory.Entities {
		if e.IsDirectory {
			ui.addDirectoryToQueue(&e)
		} else {
			ui.addSongToQueue(&e)
		}
	}
}

func (ui *Ui) search() {
	name, _ := ui.pages.GetFrontPage()
	if name != "browser" {
		return
	}
	ui.searchField.SetText("")
	ui.app.SetFocus(ui.searchField)
}

func (ui *Ui) searchNext() {
	str := ui.searchField.GetText()
	idxs := ui.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}
	curIdx := ui.artistList.GetCurrentItem()
	for _, nidx := range idxs {
		if nidx > curIdx {
			ui.artistList.SetCurrentItem(nidx)
			return
		}
	}
	ui.artistList.SetCurrentItem(idxs[0])
}

func (ui *Ui) searchPrev() {
	str := ui.searchField.GetText()
	idxs := ui.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}
	curIdx := ui.artistList.GetCurrentItem()
	for nidx := len(idxs) - 1; nidx >= 0; nidx-- {
		if idxs[nidx] < curIdx {
			ui.artistList.SetCurrentItem(idxs[nidx])
			return
		}
	}
	ui.artistList.SetCurrentItem(idxs[len(idxs)-1])
}
