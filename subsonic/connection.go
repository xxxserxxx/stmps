// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package subsonic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"

	"github.com/spezifisch/stmps/logger"
)

const MAX_RANDOM_SONGS = 50

type Connection struct {
	Username         string
	Password         string
	Host             string
	PlaintextAuth    bool
	Scrobble         bool
	RandomSongNumber uint

	clientName    string
	clientVersion string

	logger         logger.LoggerInterface
	directoryCache map[string]Directory
	albumCache     map[string]Album
	artistCache    map[string]Artist
	coverArts      map[string]image.Image
}

func Init(logger logger.LoggerInterface) *Connection {
	c := Connection{
		clientName:    "example",
		clientVersion: "1.8.0",

		logger: logger,
	}
	c.ClearCache()
	return &c
}

func (s *Connection) SetClientInfo(name, version string) {
	s.clientName = name
	s.clientVersion = version
}

func (s *Connection) ClearCache() {
	s.directoryCache = make(map[string]Directory)
	s.artistCache = make(map[string]Artist)
	s.albumCache = make(map[string]Album)
	s.coverArts = make(map[string]image.Image)
}

func (s *Connection) RemoveDirectoryCacheEntry(key string) {
	delete(s.directoryCache, key)
}

func (s *Connection) RemoveArtistCacheEntry(key string) {
	delete(s.artistCache, key)
}

func (s *Connection) RemoveAlbumCacheEntry(key string) {
	delete(s.albumCache, key)
}

func defaultQuery(connection *Connection) url.Values {
	query := url.Values{}
	if connection.PlaintextAuth {
		query.Set("p", connection.Password)
	} else {
		token, salt := authToken(connection.Password)
		query.Set("t", token)
		query.Set("s", salt)
	}
	query.Set("u", connection.Username)
	query.Set("v", connection.clientVersion)
	query.Set("c", connection.clientName)
	query.Set("f", "json")

	return query
}

// GetServerInfo pings the server and returns the response, which contains basic
// information about the server
// https://opensubsonic.netlify.app/docs/endpoints/ping/
func (connection *Connection) GetServerInfo() (Response, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/ping" + "?" + query.Encode()
	return connection.GetResponse("GetServerInfo", requestUrl)
}

// GetIndexes returns an indexed structure of all artists
// https://opensubsonic.netlify.app/docs/endpoints/getindexes/
func (connection *Connection) GetIndexes() (Indexes, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getIndexes" + "?" + query.Encode()
	i, e := connection.GetResponse("GetIndexes", requestUrl)
	return i.Indexes, e
}

// GetIndexes returns an indexed structure of all artists
// Artists in the response are _not_ sorted
// https://opensubsonic.netlify.app/docs/endpoints/getartists/
func (connection *Connection) GetArtists() (Indexes, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getArtists" + "?" + query.Encode()
	i, e := connection.GetResponse("GetArtists", requestUrl)
	return i.Artists, e
}

// GetArtist gets information about a single artist.
// If the item is in the cache, the cached item is returned; if not, it is put
// in the cache and returned.
// The albums in the response are sorted before return.
// https://opensubsonic.netlify.app/docs/endpoints/getartist/
func (connection *Connection) GetArtist(id string) (Artist, error) {
	if cachedArtist, present := connection.artistCache[id]; present {
		return cachedArtist, nil
	}

	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/getArtist" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetArtist", requestUrl)
	if err != nil {
		return resp.Artist, err
	}
	artist := resp.Artist

	// on an unsuccessful fetch, return an error
	if resp.Status != "ok" {
		return artist, fmt.Errorf("server reported an error for GetArtist(%s): %s", id, resp.Status)
	}

	sort.Slice(artist.Albums, func(i, j int) bool {
		return artist.Albums[i].Name < artist.Albums[j].Name
	})
	connection.artistCache[id] = artist

	return artist, nil
}

// GetAlbum gets information about a specific album
// If the item is in the cache, the cached item is returned; if not, it is put
// in the cache and returned.
// The songs in the album are sorted before return.
// https://opensubsonic.netlify.app/docs/endpoints/getalbum/
func (connection *Connection) GetAlbum(id string) (Album, error) {
	// FIXME implement an LPR for the album cache, lest we end up with the entire DB local
	if cachedResponse, present := connection.albumCache[id]; present {
		// This is because Albums that were fetched as Directories aren't populated correctly
		if cachedResponse.Name != "" {
			return cachedResponse, nil
		}
	}

	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/getAlbum" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetAlbum", requestUrl)
	if err != nil {
		return resp.Album, err
	}
	album := resp.Album

	// on an unsuccessful fetch, return an error
	if resp.Status != "ok" {
		return album, fmt.Errorf("server reported an error for GetAlbum(%s): %s", id, resp.Status)
	}

	sort.Slice(album.Songs, func(i, j int) bool {
		return album.Songs[i].Title < album.Songs[j].Title
	})
	connection.albumCache[id] = album

	return album, nil
}

// GetMusicDirector fetches a listing of all files in a music directory, by ID.
// If the item is in the cache, the cached item is returned; if not, it is put
// in the cache and returned.
// The entities in the directory are sorted before return.
// https://opensubsonic.netlify.app/docs/endpoints/getmusicdirectory/
func (connection *Connection) GetMusicDirectory(id string) (Directory, error) {
	if cachedResponse, present := connection.directoryCache[id]; present {
		return cachedResponse, nil
	}

	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/getMusicDirectory" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetMusicDirectory", requestUrl)
	if err != nil {
		return resp.Directory, err
	}
	directory := resp.Directory

	// on an unsuccessful fetch, return an error
	if resp.Status != "ok" {
		return directory, fmt.Errorf("server reported an error for GetMusicDirectory(%s): %s", id, resp.Status)
	}

	sort.Sort(directory.Entities)
	connection.directoryCache[id] = directory

	return directory, nil
}

// GetCoverArt fetches album art from the server, by ID. If id is empty, an
// error is returned. If, for some reason, the server response can't be parsed
// into an image, an error is returned.
// This function can process images of mime types
// - image/png
// - image/jpeg
// - image/gif
// If the item is in the cache, the cached item is returned; if not, it is put
// in the cache and returned.
// https://opensubsonic.netlify.app/docs/endpoints/getcoverart/
func (connection *Connection) GetCoverArt(id string) (image.Image, error) {
	if id == "" {
		return nil, fmt.Errorf("GetCoverArt: no ID provided")
	}
	if rv, ok := connection.coverArts[id]; ok {
		return rv, nil
	}
	query := defaultQuery(connection)
	query.Set("id", id)
	query.Set("f", "image/png")
	caller := "GetCoverArt"
	res, err := http.Get(connection.Host + "/rest/getCoverArt" + "?" + query.Encode())
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to make GET request: %v", caller, err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	} else {
		return nil, fmt.Errorf("[%s] response body is nil", caller)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[%s] unexpected status code: %d, status: %s", caller, res.StatusCode, res.Status)
	}

	if len(res.Header["Content-Type"]) == 0 {
		return nil, fmt.Errorf("[%s] unknown image type (no content-type from server)", caller)
	}
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[%s] failed to read response body: %v", caller, err)
	}
	var art image.Image
	switch res.Header["Content-Type"][0] {
	case "image/png":
		art, err = png.Decode(bytes.NewReader(responseBody))
	case "image/jpeg":
		art, err = jpeg.Decode(bytes.NewReader(responseBody))
	case "image/gif":
		art, err = gif.Decode(bytes.NewReader(responseBody))
	default:
		return nil, fmt.Errorf("[%s] unhandled image type %s: %v", caller, res.Header["Content-Type"][0], err)
	}
	if art != nil {
		// FIXME connection.coverArts shouldn't grow indefinitely. Add some LRU cleanup after loading a few hundred cover arts.
		connection.coverArts[id] = art
	}
	return art, err
}

// GetRandomSongs fetches a number of random songs. The results are not sorted.
// If a song Id is provided, songs similar to that song will be selected.
// The function returns Connection.RandomSongNumber or fewer songs; if it is 0,
// then MAX_RANDOM_SONGS are returned.
func (connection *Connection) GetRandomSongs(Id string) (Entities, error) {
	query := defaultQuery(connection)

	size := fmt.Sprintf("%d", MAX_RANDOM_SONGS)
	if connection.RandomSongNumber > 0 && connection.RandomSongNumber < 500 {
		size = fmt.Sprintf("%d", connection.RandomSongNumber)
	}

	if Id == "" {
		query.Set("size", size)
		requestUrl := connection.Host + "/rest/getRandomSongs?" + query.Encode()
		resp, err := connection.GetResponse("GetRandomSongs", requestUrl)
		return resp.RandomSongs.Songs, err
	}

	query.Set("id", Id)
	query.Set("count", size)
	requestUrl := connection.Host + "/rest/getSimilarSongs?" + query.Encode()
	resp, err := connection.GetResponse("GetSimilar", requestUrl)
	return resp.SimilarSongs.Songs, err
}

func (connection *Connection) ScrobbleSubmission(id string, isSubmission bool) (Response, error) {
	query := defaultQuery(connection)
	query.Set("id", id)

	// optional field, false for "now playing", true for "submission"
	query.Set("submission", strconv.FormatBool(isSubmission))

	requestUrl := connection.Host + "/rest/scrobble" + "?" + query.Encode()
	resp, err := connection.GetResponse("ScrobbleSubmission", requestUrl)
	return resp, err
}

func (connection *Connection) GetStarred() (Results, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getStarred" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetStarred", requestUrl)
	return resp.Starred, err
}

func (connection *Connection) ToggleStar(id string, starredItems map[string]struct{}) (Response, error) {
	query := defaultQuery(connection)
	query.Set("id", id)

	_, ok := starredItems[id]
	var action = "star"
	// If the key exists, we're unstarring
	if ok {
		action = "unstar"
	}

	requestUrl := connection.Host + "/rest/" + action + "?" + query.Encode()
	resp, err := connection.GetResponse("ToggleStar", requestUrl)
	if err != nil {
		if ok {
			delete(starredItems, id)
		} else {
			starredItems[id] = struct{}{}
		}
		return resp, err
	}
	return resp, nil
}

// FIXME this diverges from the rest of the code by recursively fetching all the data, which is why all of the background loading code was necessary. Strip all that out, and have playlists load as the user scrolls
func (connection *Connection) GetPlaylists() (Playlists, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getPlaylists" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetPlaylists", requestUrl)
	if err != nil {
		return resp.Playlists, err
	}
	playlists := resp.Playlists

	for i := 0; i < len(playlists.Playlists); i++ {
		playlist := playlists.Playlists[i]

		if playlist.SongCount == 0 {
			continue
		}

		pl, err := connection.GetPlaylist(string(playlist.Id))

		if err != nil {
			return Playlists{Playlists: make([]Playlist, 0)}, err
		}

		playlists.Playlists[i].Entries = pl.Entries

	}

	return playlists, nil
}

func (connection *Connection) GetPlaylist(id string) (Playlist, error) {
	query := defaultQuery(connection)
	query.Set("id", id)

	requestUrl := connection.Host + "/rest/getPlaylist" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetPlaylist", requestUrl)
	return resp.Playlist, err
}

// CreatePlaylist creates or updates a playlist on the server.
// If id is provided, the existing playlist with that ID is updated with the new song list.
// If name is provided, a new playlist is created with the song list.
// Either id or name _must_ be populated, or the function returns an error.
// If _both_ id and name are poplated, the function returns an error.
// songIds may be nil, in which case the new playlist is created empty, or all
// songs are removed from the existing playlist.
func (connection *Connection) CreatePlaylist(id, name string, songIds []string) (Playlist, error) {
	if (id == "" && name == "") || (id != "" && name != "") {
		return Playlist{}, errors.New("CreatePlaylist: exactly one of id or name must be provided")
	}
	query := defaultQuery(connection)
	if id != "" {
		query.Set("id", id)
	} else {
		query.Set("name", name)
	}
	for _, sid := range songIds {
		query.Add("songId", sid)
	}
	requestUrl := connection.Host + "/rest/createPlaylist" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetPlaylist", requestUrl)
	return resp.Playlist, err
}

func (connection *Connection) GetResponse(caller, requestUrl string) (Response, error) {
	zero := Response{}
	res, err := http.Get(requestUrl)
	if err != nil {
		return zero, fmt.Errorf("[%s] failed to make GET request: %v", caller, err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	} else {
		return zero, fmt.Errorf("[%s] response body is nil", caller)
	}

	if res.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("[%s] unexpected status code: %d, status: %s", caller, res.StatusCode, res.Status)
	}

	responseBody, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return zero, fmt.Errorf("[%s] failed to read response body: %v", caller, readErr)
	}

	var decodedBody responseWrapper
	err = json.Unmarshal(responseBody, &decodedBody)
	if err != nil {
		return zero, fmt.Errorf("[%s] failed to unmarshal response body: %v", caller, err)
	}

	return decodedBody.Response, nil
}

func (connection *Connection) DeletePlaylist(id string) error {
	query := defaultQuery(connection)
	query.Set("id", id)
	requestUrl := connection.Host + "/rest/deletePlaylist" + "?" + query.Encode()
	_, err := http.Get(requestUrl)
	return err
}

func (connection *Connection) AddSongToPlaylist(playlistId string, songId string) error {
	query := defaultQuery(connection)
	query.Set("playlistId", string(playlistId))
	query.Set("songIdToAdd", string(songId))
	requestUrl := connection.Host + "/rest/updatePlaylist" + "?" + query.Encode()
	_, err := http.Get(requestUrl)
	return err
}

func (connection *Connection) RemoveSongFromPlaylist(playlistId string, songIndex int) error {
	query := defaultQuery(connection)
	query.Set("playlistId", playlistId)
	query.Set("songIndexToRemove", strconv.Itoa(songIndex))
	requestUrl := connection.Host + "/rest/updatePlaylist" + "?" + query.Encode()
	_, err := http.Get(requestUrl)
	return err
}

// note that this function does not make a request, it just formats the play url
// to pass to mpv
func (connection *Connection) GetPlayUrl(entity Entity) string {
	// we don't want to call stream on a directory
	if entity.IsDirectory {
		return ""
	}

	query := defaultQuery(connection)
	query.Set("id", entity.Id)
	return connection.Host + "/rest/stream" + "?" + query.Encode()
}

// Search uses the Subsonic search3 API to query a server for all songs that have
// ID3 tags that match the query. The query is global, in that it matches in any
// ID3 field.
// https://www.subsonic.org/pages/api.jsp#search3
func (connection *Connection) Search(searchTerm string, artistOffset, albumOffset, songOffset int) (Results, error) {
	query := defaultQuery(connection)
	query.Set("query", searchTerm)
	query.Set("artistOffset", strconv.Itoa(artistOffset))
	query.Set("albumOffset", strconv.Itoa(albumOffset))
	query.Set("songOffset", strconv.Itoa(songOffset))
	requestUrl := connection.Host + "/rest/search3" + "?" + query.Encode()
	res, err := connection.GetResponse("Search", requestUrl)
	return Results(res.SearchResult3), err
}

// StartScan tells the Subsonic server to initiate a media library scan. Whether
// this is a deep or surface scan is dependent on the server implementation.
// https://subsonic.org/pages/api.jsp#startScan
func (connection *Connection) StartScan() error {
	query := defaultQuery(connection)
	requestUrl := fmt.Sprintf("%s/rest/startScan?%s", connection.Host, query.Encode())
	if res, err := connection.GetResponse("StartScan", requestUrl); err != nil {
		return err
	} else if !res.ScanStatus.Scanning {
		return fmt.Errorf("server returned false for scan status on scan attempt")
	}
	return nil
}

func (connection *Connection) SavePlayQueue(queueIds []string, current string, position int) error {
	query := defaultQuery(connection)
	for _, songId := range queueIds {
		query.Add("id", songId)
	}
	query.Set("current", current)
	query.Set("position", fmt.Sprintf("%d", position))
	requestUrl := fmt.Sprintf("%s/rest/savePlayQueue?%s", connection.Host, query.Encode())
	_, err := connection.GetResponse("SavePlayQueue", requestUrl)
	return err
}

func (connection *Connection) LoadPlayQueue() (PlayQueue, error) {
	query := defaultQuery(connection)
	requestUrl := fmt.Sprintf("%s/rest/getPlayQueue?%s", connection.Host, query.Encode())
	resp, err := connection.GetResponse("GetPlayQueue", requestUrl)
	return resp.PlayQueue, err
}

// GetLyricsBySongId fetches time synchronized song lyrics. If the server does
// not support this, an error is returned.
func (connection *Connection) GetLyricsBySongId(id string) ([]StructuredLyrics, error) {
	if id == "" {
		return []StructuredLyrics{}, fmt.Errorf("GetLyricsBySongId: no ID provided")
	}
	query := defaultQuery(connection)
	query.Set("id", id)
	query.Set("f", "json")
	caller := "GetLyricsBySongId"
	res, err := http.Get(connection.Host + "/rest/getLyricsBySongId" + "?" + query.Encode())
	if err != nil {
		return []StructuredLyrics{}, fmt.Errorf("[%s] failed to make GET request: %v", caller, err)
	}

	if res.Body != nil {
		defer res.Body.Close()
	} else {
		return []StructuredLyrics{}, fmt.Errorf("[%s] response body is nil", caller)
	}

	if res.StatusCode != http.StatusOK {
		return []StructuredLyrics{}, fmt.Errorf("[%s] unexpected status code: %d, status: %s", caller, res.StatusCode, res.Status)
	}

	if len(res.Header["Content-Type"]) == 0 {
		return []StructuredLyrics{}, fmt.Errorf("[%s] unknown image type (no content-type from server)", caller)
	}

	responseBody, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return []StructuredLyrics{}, fmt.Errorf("[%s] failed to read response body: %v", caller, readErr)
	}

	var decodedBody responseWrapper
	err = json.Unmarshal(responseBody, &decodedBody)
	if err != nil {
		return []StructuredLyrics{}, fmt.Errorf("[%s] failed to unmarshal response body: %v", caller, err)
	}
	return decodedBody.Response.LyricsList.StructuredLyrics, nil
}

func (connection *Connection) GetGenres() ([]GenreEntry, error) {
	query := defaultQuery(connection)
	requestUrl := connection.Host + "/rest/getGenres" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetGenres", requestUrl)
	if err != nil {
		return resp.Genres.Genres, err
	}
	return resp.Genres.Genres, nil
}

func (connection *Connection) GetSongsByGenre(genre string, offset int, musicFolderID string) (Entities, error) {
	query := defaultQuery(connection)
	query.Add("genre", genre)
	if offset != 0 {
		query.Add("offset", strconv.Itoa(offset))
	}
	if musicFolderID != "" {
		query.Add("musicFolderId", musicFolderID)
	}
	requestUrl := connection.Host + "/rest/getSongsByGenre" + "?" + query.Encode()
	resp, err := connection.GetResponse("GetPlaylists", requestUrl)
	if err != nil {
		return resp.SongsByGenre.Songs, err
	}
	return resp.SongsByGenre.Songs, nil
}
