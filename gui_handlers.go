package main

import (
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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

		ui.entityList.AddItem(title, "", 0, handler)
	}
}

func (ui *Ui) handlePlaylistSelected(playlist subsonic.SubsonicPlaylist) {
	ui.selectedPlaylist.Clear()

	for _, entity := range playlist.Entries {
		var title string
		var handler func()

		var id = entity.Id

		title = entity.GetSongTitle()
		handler = makeSongHandler(id, ui.connection.GetPlayUrl(&entity), title, entity.Artist, entity.Duration, ui)

		ui.selectedPlaylist.AddItem(title, "", 0, handler)
	}
}

func (ui *Ui) handleAddRandomSongs() {
	ui.addRandomSongsToQueue()
	ui.updateQueue()
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

func (ui *Ui) handleAddPlaylistSongToQueue() {
	playlistIndex := ui.playlistList.GetCurrentItem()
	entityIndex := ui.selectedPlaylist.GetCurrentItem()

	if playlistIndex < 0 || entityIndex < 0 {
		return
	}

	if entityIndex+1 < ui.selectedPlaylist.GetItemCount() {
		ui.selectedPlaylist.SetCurrentItem(entityIndex + 1)
	}

	// TODO add some bounds checking here
	if playlistIndex == -1 || entityIndex == -1 {
		return
	}

	entity := ui.playlists[playlistIndex].Entries[entityIndex]
	ui.addSongToQueue(&entity)

	ui.updateQueue()
}

func (ui *Ui) handleAddPlaylistToQueue() {
	currentIndex := ui.playlistList.GetCurrentItem()
	if currentIndex < 0 || currentIndex >= ui.playlistList.GetItemCount() {
		return
	}

	// focus next entry
	if currentIndex+1 < ui.playlistList.GetItemCount() {
		ui.playlistList.SetCurrentItem(currentIndex + 1)
	}

	playlist := ui.playlists[currentIndex]

	for _, entity := range playlist.Entries {
		ui.addSongToQueue(&entity)
	}

	ui.updateQueue()
}

func (ui *Ui) handleAddSongToPlaylist(playlist *subsonic.SubsonicPlaylist) {
	currentIndex := ui.entityList.GetCurrentItem()

	// if we have a parent directory subtract 1 to account for the [..]
	// which would be index 0 in that case with index 1 being the first entity
	if ui.currentDirectory.Parent != "" {
		currentIndex--
	}

	if currentIndex < 0 || len(ui.currentDirectory.Entities) < currentIndex {
		return
	}

	entity := ui.currentDirectory.Entities[currentIndex]

	if !entity.IsDirectory {
		if err := ui.connection.AddSongToPlaylist(string(playlist.Id), entity.Id); err != nil {
			ui.logger.PrintError("AddSongToPlaylist", err)
			return
		}
	}
	// update the playlists
	response, err := ui.connection.GetPlaylists()
	if err != nil {
		ui.logger.PrintError("GetPlaylists", err)
	}
	ui.playlists = response.Playlists.Playlists

	ui.playlistList.Clear()
	ui.addToPlaylistList.Clear()

	for _, playlist := range ui.playlists {
		ui.playlistList.AddItem(playlist.Name, "", 0, nil)
		ui.addToPlaylistList.AddItem(playlist.Name, "", 0, nil)
	}

	if currentIndex+1 < ui.entityList.GetItemCount() {
		ui.entityList.SetCurrentItem(currentIndex + 1)
	}
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

func (ui *Ui) addStarredToList() {
	response, err := ui.connection.GetStarred()
	if err != nil {
		ui.logger.Printf("addStarredToList %s", err.Error())
	}
	for _, e := range response.Starred.Song {
		// We're storing empty struct as values as we only want the indexes
		// It's faster having direct index access instead of looping through array values
		ui.starIdList[e.Id] = struct{}{}
	}
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

func (ui *Ui) newPlaylist(name string) {
	response, err := ui.connection.CreatePlaylist(name)
	if err != nil {
		ui.logger.Printf("newPlaylist: CreatePlaylist %s -- %s", name, err.Error())
		return
	}

	ui.playlists = append(ui.playlists, response.Playlist)

	ui.playlistList.AddItem(response.Playlist.Name, "", 0, nil)
	ui.addToPlaylistList.AddItem(response.Playlist.Name, "", 0, nil)
}

func (ui *Ui) deletePlaylist(index int) {
	if index == -1 || len(ui.playlists) <= index {
		return
	}

	playlist := ui.playlists[index]

	if index == 0 {
		ui.playlistList.SetCurrentItem(1)
	}

	// Removes item with specified index
	ui.playlists = append(ui.playlists[:index], ui.playlists[index+1:]...)

	ui.playlistList.RemoveItem(index)
	ui.addToPlaylistList.RemoveItem(index)
	if err := ui.connection.DeletePlaylist(string(playlist.Id)); err != nil {
		ui.logger.PrintError("deletePlaylist", err)
	}
}

func makeSongHandler(id string, uri string, title string, artist string, duration int,
	ui *Ui) func() {
	return func() {
		// there's no good way to output an error here
		_ = ui.player.Play(id, uri, title, artist, duration)
		ui.updateQueue()
	}
}

func (ui *Ui) makeEntityHandler(directoryId string) func() {
	return func() {
		ui.handleEntitySelected(directoryId)
	}
}
