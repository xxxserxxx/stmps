// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/subsonic"
)

type PlaylistPage struct {
	Root                *tview.Flex
	NewPlaylistModal    tview.Primitive
	DeletePlaylistModal tview.Primitive

	playlistList     *tview.List
	newPlaylistInput *tview.InputField
	selectedPlaylist *tview.List
	playlists        []subsonic.Playlist

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createPlaylistPage() *PlaylistPage {
	playlistPage := PlaylistPage{
		ui:        ui,
		logger:    ui.logger,
		playlists: make([]subsonic.Playlist, 0),
	}

	// left half: playlists
	playlistPage.playlistList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	playlistPage.playlistList.Box.
		SetTitle(" playlist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	playlistPage.UpdatePlaylists()

	// right half: songs of selected playlist
	playlistPage.selectedPlaylist = tview.NewList().
		ShowSecondaryText(false)
	playlistPage.selectedPlaylist.Box.
		SetTitle(" songs ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// flex wrapper
	playlistColFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(playlistPage.playlistList, 0, 1, true).
		AddItem(playlistPage.selectedPlaylist, 0, 1, false)

	// root view
	playlistPage.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(playlistColFlex, 0, 1, true)

	// "new playlist" modal
	playlistPage.newPlaylistInput = tview.NewInputField().
		SetLabel("Name: ").
		SetFieldWidth(50)
	playlistPage.newPlaylistInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			playlistPage.newPlaylist(playlistPage.newPlaylistInput.GetText())
			ui.pages.HidePage(PageNewPlaylist)
			ui.pages.SwitchToPage(PagePlaylists)
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage(PageNewPlaylist)
			ui.pages.SwitchToPage(PagePlaylists)
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		return event
	})

	newPlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(playlistPage.newPlaylistInput, 0, 1, true)

	newPlaylistFlex.SetTitle("Create new playlist").
		SetBorder(true)

	playlistPage.NewPlaylistModal = makeModal(newPlaylistFlex, 58, 3)

	// main list input handler
	playlistPage.playlistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(playlistPage.selectedPlaylist)
			return nil
		}
		switch event.Rune() {
		case 'a':
			playlistPage.handleAddPlaylistToQueue()
			return nil
		case 'n':
			ui.pages.ShowPage(PageNewPlaylist)
			ui.app.SetFocus(ui.playlistPage.newPlaylistInput)
			return nil
		case 'd':
			ui.pages.ShowPage(PageDeletePlaylist)
			return nil
		case 'R':
			playlistPage.UpdatePlaylists()
			return nil
		}

		return event
	})

	// TODO (C) Add filter/search to playlist coluumn
	playlistPage.selectedPlaylist.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(playlistPage.playlistList)
			return nil
		}
		if event.Rune() == 'a' {
			playlistPage.handleAddPlaylistSongToQueue()
			return nil
		}
		return event
	})

	// delete playlist modal
	deletePlaylistList := tview.NewList().
		ShowSecondaryText(false)

	deletePlaylistList.SetBorder(true).
		SetTitle("Confirm deletion")

	deletePlaylistList.AddItem("Confirm", "", 0, nil)

	deletePlaylistFlex := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(deletePlaylistList, 0, 1, true)

	deletePlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			playlistPage.deletePlaylist(playlistPage.playlistList.GetCurrentItem())
			ui.app.SetFocus(playlistPage.playlistList)
			ui.pages.HidePage(PageDeletePlaylist)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			ui.app.SetFocus(playlistPage.playlistList)
			ui.pages.HidePage(PageDeletePlaylist)
			return nil
		}
		return event
	})

	playlistPage.DeletePlaylistModal = makeModal(deletePlaylistFlex, 20, 3)

	playlistPage.playlistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		if index < 0 || index >= len(playlistPage.playlists) {
			return
		}
		playlistPage.handlePlaylistSelected(playlistPage.playlists[index])
	})

	// open first playlist by default so we don't get stuck when there's only one playlist
	if len(playlistPage.playlists) > 0 {
		playlistPage.handlePlaylistSelected(playlistPage.playlists[0])
	}

	return &playlistPage
}

func (p *PlaylistPage) IsNewPlaylistInputFocused(focused tview.Primitive) bool {
	return focused == p.newPlaylistInput
}

func (p *PlaylistPage) GetCount() int {
	return p.playlistList.GetItemCount()
}

func (p *PlaylistPage) UpdatePlaylists() {
	playlists, err := p.ui.connection.GetPlaylists()
	if err != nil {
		p.logger.PrintError("GetPlaylists", err)
		return
	}
	p.playlistList.Clear()
	p.playlists = playlists.Playlists
	for _, pl := range playlists.Playlists {
		p.playlistList.AddItem(tview.Escape(pl.Name), "", 0, nil)
	}
}

func (p *PlaylistPage) addPlaylist(playlist subsonic.Playlist) {
	// Rather than getting the current selected, let's use the features of closures.
}

func (p *PlaylistPage) handleAddPlaylistSongToQueue() {
	playlistIndex := p.playlistList.GetCurrentItem()
	entityIndex := p.selectedPlaylist.GetCurrentItem()
	if playlistIndex < 0 || playlistIndex >= p.playlistList.GetItemCount() {
		return
	}
	if entityIndex < 0 || entityIndex >= p.selectedPlaylist.GetItemCount() {
		return
	}
	if playlistIndex >= len(p.playlists) || entityIndex >= len(p.playlists[playlistIndex].Entries) {
		return
	}

	// select next entry
	if entityIndex+1 < p.selectedPlaylist.GetItemCount() {
		p.selectedPlaylist.SetCurrentItem(entityIndex + 1)
	}

	entity := p.playlists[playlistIndex].Entries[entityIndex]
	p.ui.addSongToQueue(entity)

	p.ui.queuePage.UpdateQueue()
}

func (p *PlaylistPage) handleAddPlaylistToQueue() {
	currentIndex := p.playlistList.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= p.playlistList.GetItemCount() || currentIndex >= len(p.playlists) {
		return
	}

	// focus next entry
	if currentIndex+1 < p.playlistList.GetItemCount() {
		p.playlistList.SetCurrentItem(currentIndex + 1)
	}

	playlist := p.playlists[currentIndex]
	for _, entity := range playlist.Entries {
		p.ui.addSongToQueue(entity)
	}

	p.ui.queuePage.UpdateQueue()
}

func (p *PlaylistPage) handlePlaylistSelected(playlist subsonic.Playlist) {
	var err error
	playlist, err = p.ui.connection.GetPlaylist(string(playlist.Id))
	if err != nil {
		p.logger.PrintError("handlePlaylistSelected", err)
		return
	}

	p.selectedPlaylist.Clear()
	p.selectedPlaylist.SetSelectedFocusOnly(true)

	for _, entity := range playlist.Entries {
		handler := p.ui.makeSongHandler(entity)
		line := formatSongForPlaylistEntry(entity)
		p.selectedPlaylist.AddItem(line, "", 0, handler)
	}
}

func (p *PlaylistPage) newPlaylist(name string) {
	playlist, err := p.ui.connection.CreatePlaylist("", name, nil)
	if err != nil {
		p.logger.Printf("newPlaylist: CreatePlaylist %s -- %s", name, err.Error())
		return
	}

	p.playlists = append(p.playlists, playlist)

	p.playlistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
	p.ui.addToPlaylistList.AddItem(tview.Escape(playlist.Name), "", 0, nil)
}

func (p *PlaylistPage) deletePlaylist(index int) {
	if index < 0 || index >= len(p.playlists) {
		return
	}

	playlist := p.playlists[index]

	if index == 0 {
		p.playlistList.SetCurrentItem(1)
	}

	// Removes item with specified index
	p.playlists = append(p.playlists[:index], p.playlists[index+1:]...)

	p.playlistList.RemoveItem(index)
	p.ui.addToPlaylistList.RemoveItem(index)
	if err := p.ui.connection.DeletePlaylist(string(playlist.Id)); err != nil {
		p.logger.PrintError("deletePlaylist", err)
	}
}
