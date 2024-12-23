// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/subsonic"
)

type SearchPage struct {
	Root               *tview.Flex
	AddToPlaylistModal tview.Primitive

	columnsFlex *tview.Flex

	artistList  *tview.List
	albumList   *tview.List
	songList    *tview.List
	searchField *tview.InputField
	queryGenre  bool

	artists []subsonic.Artist
	albums  []subsonic.Album
	songs   []subsonic.Entity

	// external refs
	ui     *Ui
	logger logger.LoggerInterface
}

func (ui *Ui) createSearchPage() *SearchPage {
	searchPage := SearchPage{
		ui:     ui,
		logger: ui.logger,
	}

	// artist list
	searchPage.artistList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.artistList.Box.
		SetTitle(" artist matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// album list
	searchPage.albumList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.albumList.Box.
		SetTitle(" album matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// song list
	searchPage.songList = tview.NewList().
		ShowSecondaryText(false)
	searchPage.songList.Box.
		SetTitle(" song matches ").
		SetTitleAlign(tview.AlignLeft).
		SetBorder(true)

	// search bar
	searchPage.searchField = tview.NewInputField().
		SetLabel("search:").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetDoneFunc(func(key tcell.Key) {
			searchPage.aproposFocus()
		})

	searchPage.columnsFlex = tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(searchPage.artistList, 0, 1, true).
		AddItem(searchPage.albumList, 0, 1, false).
		AddItem(searchPage.songList, 0, 1, false)

	searchPage.Root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(searchPage.columnsFlex, 0, 1, true).
		AddItem(searchPage.searchField, 1, 1, false)

	searchPage.artistList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.songList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.albumList)
			return nil
		case tcell.KeyEnter:
			if len(searchPage.artists) != 0 {
				idx := searchPage.artistList.GetCurrentItem()
				searchPage.addArtistToQueue(searchPage.artists[idx])
				return nil
			}
			return event
		}

		switch event.Rune() {
		case 'a':
			if len(searchPage.artists) != 0 {
				idx := searchPage.artistList.GetCurrentItem()
				searchPage.logger.Printf("artistList adding (%d) %s", idx, searchPage.artists[idx].Name)
				searchPage.addArtistToQueue(searchPage.artists[idx])
				return nil
			}
			return event
		case '/':
			searchPage.searchField.SetLabel("search:")
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'g':
			searchPage.albumList.Clear()
			searchPage.artistList.Clear()
			searchPage.songList.Clear()
			if searchPage.queryGenre {
				searchPage.albumList.SetTitle(" album matches ")
			} else {
				searchPage.populateGenres()
				searchPage.albumList.SetTitle(fmt.Sprintf(" genres (%d) ", searchPage.albumList.GetItemCount()))
				searchPage.ui.app.SetFocus(searchPage.albumList)
			}
			searchPage.queryGenre = !searchPage.queryGenre
			return nil
		}

		return event
	})
	search := make(chan string, 5)
	searchPage.albumList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.artistList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.songList)
			return nil
		case tcell.KeyEnter:
			if !searchPage.queryGenre {
				idx := searchPage.albumList.GetCurrentItem()
				if idx >= 0 && idx < len(searchPage.albums) {
					searchPage.addAlbumToQueue(searchPage.albums[idx])
					return nil
				}
				return event
			} else {
				search <- ""
				searchPage.artistList.Clear()
				searchPage.artists = make([]subsonic.Artist, 0)
				searchPage.songList.Clear()
				searchPage.songs = make([]subsonic.Entity, 0)

				idx := searchPage.albumList.GetCurrentItem()
				// searchPage.logger.Printf("current item index = %d; albumList len = %d", idx, searchPage.albumList.GetItemCount())
				queryStr, _ := searchPage.albumList.GetItemText(idx)
				search <- queryStr
				return nil
			}
		}

		switch event.Rune() {
		case 'a':
			if searchPage.queryGenre {
				idx := searchPage.albumList.GetCurrentItem()
				if idx < searchPage.albumList.GetItemCount() {
					genre, _ := searchPage.albumList.GetItemText(idx)
					searchPage.addGenreToQueue(genre)
				}
				return nil
			}
			idx := searchPage.albumList.GetCurrentItem()
			if idx >= 0 && idx < len(searchPage.albums) {
				searchPage.addAlbumToQueue(searchPage.albums[idx])
				return nil
			}
			return event
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'g':
			searchPage.albumList.Clear()
			searchPage.artistList.Clear()
			searchPage.songList.Clear()
			if searchPage.queryGenre {
				searchPage.albumList.SetTitle(" album matches ")
			} else {
				searchPage.populateGenres()
				searchPage.albumList.SetTitle(fmt.Sprintf(" genres (%d) ", searchPage.albumList.GetItemCount()))
				searchPage.ui.app.SetFocus(searchPage.albumList)
			}
			searchPage.queryGenre = !searchPage.queryGenre
			return nil
		}

		return event
	})
	searchPage.songList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			ui.app.SetFocus(searchPage.albumList)
			return nil
		case tcell.KeyRight:
			ui.app.SetFocus(searchPage.artistList)
			return nil
		case tcell.KeyEnter:
			if len(searchPage.artists) != 0 {
				idx := searchPage.songList.GetCurrentItem()
				ui.addSongToQueue(searchPage.songs[idx])
				ui.queuePage.UpdateQueue()
				return nil
			}
			return event
		}

		switch event.Rune() {
		case 'a':
			if len(searchPage.artists) != 0 {
				idx := searchPage.songList.GetCurrentItem()
				ui.addSongToQueue(searchPage.songs[idx])
				ui.queuePage.updateQueue()
				return nil
			}
			return event
		case '/':
			searchPage.ui.app.SetFocus(searchPage.searchField)
			return nil
		case 'g':
			searchPage.albumList.Clear()
			searchPage.artistList.Clear()
			searchPage.songList.Clear()
			if searchPage.queryGenre {
				searchPage.albumList.SetTitle(" album matches ")
			} else {
				searchPage.populateGenres()
				searchPage.albumList.SetTitle(fmt.Sprintf(" genres (%d) ", searchPage.albumList.GetItemCount()))
				searchPage.ui.app.SetFocus(searchPage.albumList)
			}
			searchPage.queryGenre = !searchPage.queryGenre
			return nil
		}

		return event
	})
	searchPage.searchField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyESC:
			searchPage.aproposFocus()
		case tcell.KeyEnter:
			search <- ""
			searchPage.artistList.Clear()
			if !searchPage.queryGenre {
				searchPage.albumList.Clear()
				searchPage.albums = make([]subsonic.Album, 0)
			}
			searchPage.songList.Clear()
			searchPage.songs = make([]subsonic.Entity, 0)

			queryStr := searchPage.searchField.GetText()
			search <- queryStr
			searchPage.aproposFocus()
		default:
			return event
		}
		return nil
	})
	go searchPage.search(search)

	return &searchPage
}

func (s *SearchPage) search(search chan string) {
	var query string
	var artOff, albOff, songOff int
	more := make(chan struct{}, 5)
	for {
		// quit searching if we receive an interrupt
		select {
		case query = <-search:
			artOff = 0
			albOff = 0
			songOff = 0
			for len(more) > 0 {
				<-more
			}
			if query == "" {
				continue
			}
		case <-more:
		}
		if s.queryGenre {
			s.logger.Printf("genre %q %d", query, songOff)
			songs, err := s.ui.connection.GetSongsByGenre(query, songOff, "")
			if err != nil {
				s.logger.PrintError("SearchPage.search GetSongsByGenre", err)
				continue
			}
			if len(songs) == 0 {
				s.logger.Printf("found a total of %d songs", songOff)
				continue
			}
			s.ui.app.QueueUpdate(func() {
				if songOff == 0 {
					s.artistList.Box.SetTitle(" artist matches ")
				}
				for _, song := range songs {
					s.songList.AddItem(tview.Escape(song.Title), "", 0, nil)
					s.songs = append(s.songs, song)
				}
				s.songList.Box.SetTitle(fmt.Sprintf(" genre song matches (%d) ", len(s.songs)))
				songOff += len(songs)
				more <- struct{}{}
			})
		} else {
			s.logger.Printf("search query %q %d/%d/%d", query, artOff, albOff, songOff)
			results, err := s.ui.connection.Search(query, artOff, albOff, songOff)
			if err != nil {
				s.logger.PrintError("SearchPage.search Search", err)
				return
			}
			s.logger.Printf("query returned %d/%d/%d", artOff, albOff, songOff)
			// Quit searching if there are no more results
			if len(results.Artists) == 0 &&
				len(results.Albums) == 0 &&
				len(results.Songs) == 0 {
				continue
			}
			s.ui.app.QueueUpdate(func() {
				query = strings.ToLower(query)
				for _, artist := range results.Artists {
					if strings.Contains(strings.ToLower(artist.Name), query) {
						s.artistList.AddItem(tview.Escape(artist.Name), "", 0, nil)
						s.artists = append(s.artists, artist)
					}
				}
				s.artistList.Box.SetTitle(fmt.Sprintf(" artist matches (%d) ", len(s.artists)))
				for _, album := range results.Albums {
					if strings.Contains(strings.ToLower(album.Name), query) {
						s.albumList.AddItem(tview.Escape(album.Name), "", 0, nil)
						s.albums = append(s.albums, album)
					}
				}
				s.albumList.Box.SetTitle(fmt.Sprintf(" album matches (%d) ", len(s.albums)))
				for _, song := range results.Songs {
					if strings.Contains(strings.ToLower(song.Title), query) {
						s.songList.AddItem(tview.Escape(song.Title), "", 0, nil)
						s.songs = append(s.songs, song)
					}
				}
				s.songList.Box.SetTitle(fmt.Sprintf(" song matches (%d) ", len(s.songs)))
				artOff += len(results.Artists)
				albOff += len(results.Albums)
				songOff += len(results.Songs)
				more <- struct{}{}
			})
		}

		// Only do this the one time, to prevent loops from stealing the user's focus
		s.aproposFocus()

		s.ui.app.Draw()
	}
}

func (s *SearchPage) addGenreToQueue(query string) {
	var songOff int
	for {
		songs, err := s.ui.connection.GetSongsByGenre(query, songOff, "")
		if err != nil {
			s.logger.PrintError("SearchPage.addGenreToQueue", err)
			return
		}
		if len(songs) == 0 {
			break
		}
		for _, song := range songs {
			s.ui.addSongToQueue(song)
		}
		songOff += len(songs)
	}
	s.logger.Printf("added a total of %d songs to the queue for %q", songOff, query)
	s.ui.queuePage.UpdateQueue()
}

func (s *SearchPage) addArtistToQueue(entity subsonic.Ider) {
	artist, err := s.ui.connection.GetArtist(entity.ID())
	if err != nil {
		s.logger.Printf("addArtistToQueue: GetArtist %s -- %s", entity.ID(), err.Error())
		return
	}

	artistId := artist.Id
	for _, album := range artist.Albums {
		response, err := s.ui.connection.GetAlbum(album.Id)
		if err != nil {
			s.logger.Printf("error getting album %s while adding artist to queue", album.Id)
			return
		}
		sort.Sort(response.Songs)
		// We make sure we add only albums who's artists match the artist
		// being added; this prevents collection albums with many different
		// artists that show up in the Album column having _all_ of the songs
		// on the album -- even ones that don't match the artist -- from
		// being added when the user adds an album from the search results.
		for _, e := range response.Songs {
			// Depending on the server implementation, the server may or may not
			// respond with a list of artists. If either the Artist field matches,
			// or the artist name is in a list of artists, then we add the song.
			if e.ArtistId == artistId {
				s.ui.addSongToQueue(e)
				continue
			}
			for _, art := range e.Artists {
				if art.Id == artistId {
					s.ui.addSongToQueue(e)
					break
				}
			}
		}
	}

	s.ui.queuePage.UpdateQueue()
}

func (s *SearchPage) addAlbumToQueue(entity subsonic.Ider) {
	response, err := s.ui.connection.GetAlbum(entity.ID())
	if err != nil {
		s.logger.Printf("addToQueue: GetMusicDirectory %s -- %s", entity.ID(), err.Error())
		return
	}
	sort.Sort(response.Songs)
	for _, e := range response.Songs {
		s.ui.addSongToQueue(e)
	}
	s.ui.queuePage.UpdateQueue()
}

func (s *SearchPage) aproposFocus() {
	if s.queryGenre {
		s.ui.app.SetFocus(s.songList)
		return
	}
	if len(s.artists) != 0 {
		s.ui.app.SetFocus(s.artistList)
	} else if len(s.albums) != 0 {
		s.ui.app.SetFocus(s.albumList)
	} else if len(s.songs) != 0 {
		s.ui.app.SetFocus(s.songList)
	} else {
		s.ui.app.SetFocus(s.artistList)
	}
}

func (s *SearchPage) populateGenres() {
	genres, err := s.ui.connection.GetGenres()
	if err != nil {
		s.logger.PrintError("populateGenres", err)
		return
	}
	slices.SortFunc(genres, func(a, b subsonic.GenreEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	for _, entry := range genres {
		s.albumList.AddItem(tview.Escape(entry.Name), "", 0, nil)
	}
}
