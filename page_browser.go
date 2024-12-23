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
	// TODO Subsonic can provide artist images. Find a place to display them in the browser
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
	// TODO add filter/search to entity list (albums/songs)
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

	// TODO Add a toggle to switch the browser to a directory browser

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
		// TODO Enter on an artist should... what? Add & play? Switch to the Entity list?

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
		it, _ := browserPage.artistList.GetItemText(index)
		ui.logger.Printf("artistList changed, index %d (%d, %d): %q, %q",
			index,
			browserPage.artistList.GetItemCount(),
			len(browserPage.artistObjectList),
			it,
			browserPage.artistObjectList[index].Name)
		if index < len(browserPage.artistObjectList) {
			browserPage.handleArtistSelected(index, browserPage.artistObjectList[index])
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
			browserPage.handleArtistSelected(artistIdx, entity)
			return nil
		}
		if event.Rune() == 'S' {
			browserPage.handleAddRandomSongs("similar")
		}
		return event
	})

	// open first artist by default so we don't get stuck when there's only one artist
	if len(browserPage.artistObjectList) > 0 {
		browserPage.handleArtistSelected(0, browserPage.artistObjectList[0])
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
			idx := b.artistList.GetCurrentItem()
			b.handleArtistSelected(idx, b.currentArtist)
		}
	}
}

func (b *BrowserPage) handleAddArtistToQueue() {
	currentIndex := b.artistList.GetCurrentItem()

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
func (b *BrowserPage) handleArtistSelected(idx int, artist subsonic.Artist) {
	// Refresh the artist and update the object list
	artist, err := b.ui.connection.GetArtist(artist.Id)
	if err != nil {
		b.logger.PrintError("handleArtistSelected", err)
	}
	b.logger.Printf("handleArtistSelected debug: idx=%d, artist=%q", idx, artist.Name)
	if idx >= len(b.artistObjectList) {
		b.logger.Printf("error: handleArtistSelected index %d > %d size of artist object list", idx, len(b.artistObjectList))
		return
	}
	b.logger.Printf("handleArtistSelected debug: setting artist object list %d to %q", idx, artist.Name)
	b.currentArtist, b.artistObjectList[idx] = artist, artist

	b.entityList.Clear()
	b.currentAlbum = subsonic.Album{}

	b.entityList.Box.SetTitle(" album ")

	for _, album := range artist.Albums {
		title := entityListTextFormat(album.Id, album.Name, true, b.ui.starIdList)
		b.entityList.AddItem(title, "", 0, func() { b.handleEntitySelected(album.Id) })
	}
}

const VARIOUS_ARTISTS = "Various Artists"

// hasArtist tests whether artist is the artist, or is in either
// the artist or album artist lists
func hasArtist(song subsonic.Entity, artist subsonic.Artist) bool {
	// Navidrome doesn't correctly populate artist fields, so we have to jump through some hoops here.
	// The convention among Subsonic servers (and many media servers, in general) is that albums with
	// multiple artists:
	// 1. Set the artist field to the actual artist of the song
	// 2. Set the albumArtist field to "Various Artists"
	// Navidrome does not return the albumArtists field through the API, which means that clients
	// can't check this convention.
	//
	// This code wants to help the caller determine whether a given song matches a
	// specific artist, even when the song is part of a collection. The issue is when
	// the user is browsing under "Various Artists"; gonic populates the
	// "albumArtists" field in each song, allowing us to match the "Various Artists"
	// artist with the song. Since Navidrome does not, no songs will match.
	//
	// We work around this by testing whether the artist is "Various Artist"; if so, we
	// return "true". It's a hack, but the cases where the results are undesireable should
	// be edge cases.
	if artist.Name == VARIOUS_ARTISTS {
		return true
	}
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
			func() {
				idx := b.artistList.GetCurrentItem()
				b.handleArtistSelected(idx, b.currentArtist)
			})
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
		// We're viewing the artist's albums, so find the album the user wants to add
		if currentIndex >= len(b.currentArtist.Albums) {
			b.logger.Printf("error: handleAddEntityToX invalid state, index %d > %d number of albums", currentIndex, len(b.currentArtist.Albums))
			return
		}
		album := b.currentArtist.Albums[currentIndex]
		// The album may be sparse; if so, populate it
		if len(album.Songs) == 0 {
			a, e := b.ui.connection.GetAlbum(album.Id)
			if e != nil {
				b.logger.Printf("error: handleAddEntityToX failed to get album %s: %s", album.Id, e)
				return
			}
			b.currentArtist.Albums[currentIndex], album = a, a
		}
		// This code handles the case where Artist X has a song in Album Y, but Y also
		// has songs from other artists.
		for _, song := range album.Songs {
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
