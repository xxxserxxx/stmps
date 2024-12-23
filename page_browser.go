// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/subsonic"
)

type BrowserPage struct {
	Root               *tview.Flex
	AddToPlaylistModal tview.Primitive

	artistFlex *tview.Flex

	artistList  *tview.List
	entityList  *tview.List
	searchField *tview.InputField

	currentArtist subsonic.Artist
	currentAlbum  subsonic.Album

	artistObjectList []subsonic.Artist

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createBrowserPage(artists []subsonic.Artist) *BrowserPage {
	browserPage := BrowserPage{
		ui:     ui,
		logger: ui.logger,

		currentArtist: subsonic.Artist{},
		// artistObjectList is initially sparse artist info: name & id (and artist image & album count)
		artistObjectList: artists,
	}

	// artist list
	browserPage.artistList = tview.NewList().
		ShowSecondaryText(false)
	browserPage.artistList.Box.
		SetTitle(" artist ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	for _, artist := range artists {
		browserPage.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
	}
	browserPage.artistObjectList = artists

	// album list
	browserPage.entityList = tview.NewList().
		ShowSecondaryText(false).
		SetSelectedFocusOnly(true)
	browserPage.entityList.Box.
		SetTitle(" album ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	browserPage.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetChangedFunc(func(s string) {
			idxs := browserPage.artistList.FindItems(s, "", false, true)
			if len(idxs) == 0 {
				return
			}
			browserPage.artistList.SetCurrentItem(idxs[0])
		}).
		SetDoneFunc(func(key tcell.Key) {
			ui.app.SetFocus(browserPage.artistList)
		})

	browserPage.artistFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(browserPage.artistList, 0, 1, true).
		AddItem(browserPage.entityList, 0, 1, false)

	browserPage.Root = tview.NewFlex().SetDirection(tview.FlexRow)
	browserPage.showSearchField(false) // add artist/search items

	// going right from the artist list should focus the album/song list
	browserPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRight {
			ui.app.SetFocus(browserPage.entityList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			browserPage.showSearchField(false)
			ui.app.SetFocus(browserPage.artistList)
			return nil
		}

		switch event.Rune() {
		case 'a':
			browserPage.handleAddArtistToQueue()
			return nil
		case '/':
			browserPage.showSearchField(true)
			browserPage.search()
			return nil
		case 'n':
			browserPage.showSearchField(true)
			browserPage.searchNext()
			return nil
		case 'N':
			browserPage.showSearchField(true)
			browserPage.searchPrev()
			return nil
		case 'S':
			browserPage.handleAddRandomSongs("similar")
			return nil
		case 'R':
			goBackTo := browserPage.artistList.GetCurrentItem()

			// REFRESH artists
			artistsIndex, err := ui.connection.GetArtists()
			if err != nil {
				ui.logger.Printf("Error fetching artists from server: %s\n", err)
				return event
			}
			artists := make([]subsonic.Artist, 0)
			for _, ind := range artistsIndex.Index {
				artists = append(artists, ind.Artists...)
			}
			sort.Slice(artists, func(i, j int) bool {
				return artists[i].Name < artists[j].Name
			})

			browserPage.artistList.Clear()
			ui.connection.ClearCache()

			for _, artist := range artists {
				browserPage.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
			}
			browserPage.artistObjectList = artists
			ui.logger.Printf("added %d items to artistList and artistObjectList", len(artists))

			// Try to put the user to about where they were
			if goBackTo < browserPage.artistList.GetItemCount() {
				browserPage.artistList.SetCurrentItem(goBackTo)
			}
			return nil
		}
		return event
	})

	browserPage.artistList.SetChangedFunc(func(index int, _ string, _ string, _ rune) {
		ui.logger.Printf("artistList changed, index %d, artistList %d, artistObjectList %d", index, browserPage.artistList.GetItemCount(), len(browserPage.artistObjectList))
		if index < len(browserPage.artistObjectList) {
			browserPage.handleArtistSelected(browserPage.artistObjectList[index])
		}
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

	browserPage.AddToPlaylistModal = makeModal(addToPlaylistFlex, 60, 20)

	ui.addToPlaylistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.pages.HidePage(PageAddToPlaylist)
			ui.pages.SwitchToPage(PageBrowser)
			ui.app.SetFocus(browserPage.entityList)
			return nil
		} else if event.Key() == tcell.KeyEnter {
			playlist := ui.playlists[ui.addToPlaylistList.GetCurrentItem()]
			browserPage.handleAddEntityToPlaylist(&playlist)

			ui.pages.HidePage(PageAddToPlaylist)
			ui.pages.SwitchToPage(PageBrowser)
			ui.app.SetFocus(browserPage.entityList)
			return nil
		}

		return event
	})

	browserPage.entityList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyLeft {
			ui.app.SetFocus(browserPage.artistList)
			return nil
		}
		if event.Rune() == 'a' {
			browserPage.handleAddEntityToQueue()
			return nil
		}
		if event.Rune() == 'y' {
			browserPage.handleToggleEntityStar()
			return nil
		}
		if event.Rune() == 'A' {
			// only makes sense to add to a playlist if there are playlists
			if ui.playlistPage.GetCount() > 0 {
				ui.pages.ShowPage(PageAddToPlaylist)
				ui.app.SetFocus(ui.addToPlaylistList)
			} else {
				ui.showMessageBox("No playlists available. Create one first.")
			}
			return nil
		}
		// REFRESH only the artist
		if event.Rune() == 'R' {
			artistIdx := browserPage.artistList.GetCurrentItem()
			entity := browserPage.artistObjectList[artistIdx]
			ui.connection.RemoveArtistCacheEntry(entity.Id)
			browserPage.handleArtistSelected(browserPage.artistObjectList[artistIdx])
			return nil
		}
		if event.Rune() == 'S' {
			browserPage.handleAddRandomSongs("similar")
		}
		return event
	})

	// open first artist by default so we don't get stuck when there's only one artist
	if len(browserPage.artistObjectList) > 0 {
		browserPage.handleArtistSelected(browserPage.artistObjectList[0])
	}

	return &browserPage
}

func (b *BrowserPage) showSearchField(visible bool) {
	b.Root.Clear()
	b.Root.AddItem(b.artistFlex, 0, 1, true)

	if visible {
		b.Root.AddItem(b.searchField, 1, 0, false)
	}
}

func (b *BrowserPage) IsSearchFocused(focused tview.Primitive) bool {
	return focused == b.searchField
}

func (b *BrowserPage) UpdateStars() {
	// reload album/song list if one is open
	if b.currentArtist.Id != "" {
		if b.currentAlbum.Id != "" {
			b.handleEntitySelected(b.currentAlbum.Id)
		} else {
			b.handleArtistSelected(b.currentArtist)
		}
	}
}

func (b *BrowserPage) handleAddArtistToQueue() {
	currentIndex := b.artistList.GetCurrentItem()
	if b.artistList.GetCurrentItem() < 0 {
		return
	}

	for _, album := range b.currentArtist.Albums {
		b.addAlbumToQueue(album)
	}

	if currentIndex+1 < b.artistList.GetItemCount() {
		b.artistList.SetCurrentItem(currentIndex + 1)
	}

	b.ui.queuePage.UpdateQueue()
}

func (b *BrowserPage) handleAddRandomSongs(randomType string) {
	defer b.ui.queuePage.UpdateQueue()
	if randomType == "random" || b.currentAlbum.Id == "" {
		b.ui.addRandomSongsToQueue("")
		return
	}

	currentIndex := b.entityList.GetCurrentItem()
	// account for [..] entry that we show, see handleEntitySelected()
	currentIndex--
	if currentIndex < 0 {
		return
	}
	if currentIndex > len(b.currentAlbum.Songs) {
		b.logger.Printf("error: handleAddRandomSongs invalid state, index %d > %d number of songs", currentIndex, len(b.currentAlbum.Songs))
		return
	}

	b.ui.addRandomSongsToQueue(b.currentAlbum.Songs[currentIndex].Id)
}

// handleArtistSelected takes an artist ID and sets up the contents of the
// entityList, including setting up selection handlers for each item in the
// list. It also refreshes the artist from the server if it has a sparse copy.
func (b *BrowserPage) handleArtistSelected(artist subsonic.Artist) {
	// Refresh the artist and update the object list
	artist, err := b.ui.connection.GetArtist(artist.Id)
	if err != nil {
		b.logger.PrintError("handleArtistSelected", err)
	}
	idx := b.artistList.GetCurrentItem()
	if idx >= len(b.artistObjectList) {
		b.logger.Printf("error: handleArtistSelected index %d > %d size of artist object list", idx, len(b.artistObjectList))
		return
	}
	b.artistObjectList[idx] = artist

	b.currentArtist = artist

	b.entityList.Clear()
	b.currentAlbum = subsonic.Album{}

	b.entityList.Box.SetTitle(" album ")

	for _, album := range artist.Albums {
		title := entityListTextFormat(album.Id, album.Name, true, b.ui.starIdList)
		b.entityList.AddItem(title, "", 0, func() { b.handleEntitySelected(album.Id) })
	}
}

// hasArtist tests whether artist is the artist, or is in either
// the artist or album artist lists
func hasArtist(song subsonic.Entity, artist subsonic.Artist) bool {
	if song.ArtistId == artist.Id {
		return true
	}
	for _, art := range song.Artists {
		if art.Id == artist.Id {
			return true
		}
	}
	for _, art := range song.AlbumArtists {
		if art.Id == artist.Id {
			return true
		}
	}
	return false
}

// handleEntitySelected takes an album or song ID and sets up the
// contents of the entityList, including setting up selection handlers for each
// item in the list.
//
// If an album is selected, it displays only songs that have the selected artist
// as their artist.
func (b *BrowserPage) handleEntitySelected(id string) {
	if id == "" {
		return
	}

	if b.currentAlbum.Id == "" {
		// entityList contains albums, so set the album
		album, err := b.ui.connection.GetAlbum(id)
		if err != nil {
			b.logger.Printf("handleEntitySelected: GetAlbum %s -- %v", id, err)
			return
		}
		b.currentAlbum = album
		// Browsing an album
		b.entityList.Clear()
		b.entityList.Box.SetTitle(" song ")
		b.entityList.AddItem(
			tview.Escape("[..]"), "", 0,
			func() { b.handleArtistSelected(b.currentArtist) })
		for _, song := range album.Songs {
			if hasArtist(song, b.currentArtist) {
				b.entityList.AddItem(song.Title, "", 0, b.ui.makeSongHandler(song))
			}
		}
		return
	}

	// entityList contains songs, so activate the song

	// Derive the song from the selection list
	currentIndex := b.entityList.GetCurrentItem()
	// We don't handle [..] here
	if currentIndex < 1 {
		return
	}
	b.entityList.Clear()
	song := b.currentAlbum.Songs[currentIndex]

	uri := b.ui.connection.GetPlayUrl(song)
	coverArtId := song.CoverArtId
	if err := b.ui.player.PlayUri(uri, coverArtId, song); err != nil {
		b.ui.logger.PrintError("SongHandler Play", err)
		return
	}
	b.ui.queuePage.UpdateQueue()
}

func (b *BrowserPage) handleToggleEntityStar() {
	currentIndex := b.entityList.GetCurrentItem()
	// Keep the original so we can update the label later
	originalIndex := currentIndex
	var idToStar, title string
	var isAlbum bool
	if b.currentAlbum.Id != "" {
		// We're in an album; remove 1 for the [..]
		currentIndex--
		if currentIndex < 0 {
			return
		}
		if currentIndex >= len(b.currentAlbum.Songs) {
			b.logger.Printf("error: handleToggleEntityStar bad state; index %d > %d number of songs", currentIndex, len(b.currentAlbum.Songs))
			return
		}
		song := b.currentAlbum.Songs[currentIndex-1]
		idToStar = song.Id
		title = song.Title
	} else {
		// We're in a list of albums
		if currentIndex >= len(b.currentArtist.Albums) {
			b.logger.Printf("error: handleToggleEntityStar bad state; index %d > %d number of albums", currentIndex, len(b.currentArtist.Albums))
			return
		}
		album := b.currentArtist.Albums[currentIndex]
		idToStar = album.Id
		title = album.Name
		isAlbum = true
	}

	// If the song is already in the star list, remove it
	_, remove := b.ui.starIdList[idToStar]

	if _, err := b.ui.connection.ToggleStar(idToStar, b.ui.starIdList); err != nil {
		b.logger.PrintError("ToggleStar", err)
		return
	}

	if remove {
		delete(b.ui.starIdList, idToStar)
	} else {
		b.ui.starIdList[idToStar] = struct{}{}
	}

	// update entity list entry
	text := entityListTextFormat(idToStar, title, isAlbum, b.ui.starIdList)
	b.entityList.SetItemText(originalIndex, text, "")

	b.ui.queuePage.UpdateQueue()
}

func entityListTextFormat(id, title string, dir bool, starredItems map[string]struct{}) string {
	if dir {
		title = "[" + title + "]"
	}

	star := ""
	if _, hasStar := starredItems[id]; hasStar {
		star = " [red]♥"
	}
	return tview.Escape(title) + star
}

//nolint:golint,unused
func (b *BrowserPage) addArtistToQueue(artist subsonic.Artist) {
	var err error
	// If the artist is sparse, populate it
	if len(artist.Albums) == 0 {
		artist, err = b.ui.connection.GetArtist(artist.Id)
		if err != nil {
			b.logger.Printf("addArtistToQueue: GetArtist %s -- %s", artist.Id, err.Error())
			return
		}
		// If it's _still_ sparse, return, as there's nothing to do
		if len(artist.Albums) == 0 {
			b.logger.Printf("addArtistToQueue: artist %q (%q) has no albums", artist.Name, artist.Id)
			return
		}
	}

	for _, album := range artist.Albums {
		if len(album.Songs) == 0 {
			album, err = b.ui.connection.GetAlbum(album.Id)
			if err != nil {
				b.logger.Printf("addArtistToQueue: GetAlbum %s -- %s", album.Id, err.Error())
				// Hope the error is transient
				continue
			}
		}
		for _, s := range album.Songs {
			b.ui.addSongToQueue(s)
		}
	}
}

func (b *BrowserPage) addAlbumToQueue(album subsonic.Album) {
	var err error
	if len(album.Songs) == 0 {
		album, err = b.ui.connection.GetAlbum(album.Id)
	}
	if err != nil {
		b.logger.Printf("addAlbumToQueue: GetAlbum %s -- %s", album.Id, err.Error())
		return
	}

	for _, s := range album.Songs {
		// TODO maybe BrowserPage gets its own version of this function that uses dirname as artist name as fallback
		b.ui.addSongToQueue(s)
	}
}

//nolint:golint,unused
func (b *BrowserPage) addDirectoryToQueue(entity *subsonic.Entity) {
	directory, err := b.ui.connection.GetMusicDirectory(entity.Id)
	if err != nil {
		b.logger.Printf("addDirectoryToQueue: GetMusicDirectory %s -- %s", entity.Id, err.Error())
		return
	}

	for _, e := range directory.Entities {
		if e.IsDirectory {
			b.addDirectoryToQueue(&e)
		} else {
			// TODO maybe BrowserPage gets its own version of this function that uses dirname as artist name as fallback
			b.ui.addSongToQueue(e)
		}
	}
}

func (b *BrowserPage) search() {
	name, _ := b.ui.pages.GetFrontPage()
	if name != "browser" {
		return
	}
	b.searchField.SetText("")
	b.ui.app.SetFocus(b.searchField)
}

func (b *BrowserPage) searchNext() {
	str := b.searchField.GetText()
	idxs := b.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}

	curIdx := b.artistList.GetCurrentItem()
	for _, nidx := range idxs {
		if nidx > curIdx {
			b.artistList.SetCurrentItem(nidx)
			return
		}
	}
	b.artistList.SetCurrentItem(idxs[0])
}

func (b *BrowserPage) searchPrev() {
	str := b.searchField.GetText()
	idxs := b.artistList.FindItems(str, "", false, true)
	if len(idxs) == 0 {
		return
	}

	curIdx := b.artistList.GetCurrentItem()
	for nidx := len(idxs) - 1; nidx >= 0; nidx-- {
		if idxs[nidx] < curIdx {
			b.artistList.SetCurrentItem(idxs[nidx])
			return
		}
	}
	b.artistList.SetCurrentItem(idxs[len(idxs)-1])
}

func (b *BrowserPage) handleAddEntityToX(add func(song subsonic.Entity), update func()) {
	currentIndex := b.entityList.GetCurrentItem()
	if currentIndex < 0 {
		return
	}

	if b.currentAlbum.Id != "" {
		// account for [..] entry that we show, see handleEntitySelected()
		currentIndex--
		if currentIndex < 0 {
			return
		}
		if currentIndex >= b.currentAlbum.Songs.Len() {
			b.logger.Printf("error: handleAddEntityToX invalid state, index %d > %d number of songs", currentIndex, len(b.currentAlbum.Songs))
			return
		}
		add(b.currentAlbum.Songs[currentIndex])
	} else {
		if currentIndex >= len(b.currentArtist.Albums) {
			b.logger.Printf("error: handleAddEntityToX invalid state, index %d > %d number of albums", currentIndex, len(b.currentArtist.Albums))
			return
		}
		for _, song := range b.currentArtist.Albums[currentIndex].Songs {
			if hasArtist(song, b.currentArtist) {
				add(song)
			}
		}
	}

	update()

	if currentIndex+1 < b.entityList.GetItemCount() {
		b.entityList.SetCurrentItem(currentIndex + 1)
	}
}

func (b *BrowserPage) handleAddEntityToQueue() {
	b.handleAddEntityToX(b.ui.addSongToQueue, b.ui.queuePage.UpdateQueue)
}

func (b *BrowserPage) handleAddEntityToPlaylist(playlist *subsonic.Playlist) {
	b.handleAddEntityToX(func(song subsonic.Entity) {
		if err := b.ui.connection.AddSongToPlaylist(string(playlist.Id), song.Id); err != nil {
			b.logger.PrintError("AddSongToPlaylist", err)
			return
		}
	}, b.ui.playlistPage.UpdatePlaylists)
}
