// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package subsonic

import (
	"encoding/json"

	"strconv"
	"strings"
)

type Ider interface {
	ID() string
}

// response structs
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Directory struct {
	Id       string   `json:"id"`
	Parent   string   `json:"parent"`
	Name     string   `json:"name"`
	Entities Entities `json:"child"`
}

func (s Directory) ID() string {
	return s.Id
}

// Songs (and Playlists) are here because of how the Subsonic API structures the
// JSON, frequently nesting things unnecessarily.
type Songs struct {
	Songs Entities `json:"song"`
}

type Results struct {
	Artists []Artist `json:"artist"`
	Albums  []Album  `json:"album"`
	Songs   Entities `json:"song"`
}

type ScanStatus struct {
	Scanning bool `json:"scanning"`
	Count    int  `json:"count"`
}

type PlayQueue struct {
	Current  string   `json:"current"`
	Position int      `json:"position"`
	Entries  Entities `json:"entry"`
}

type EntityBase struct {
	Id            string
	Created       string
	ArtistId      string
	Artist        string
	DisplayArtist string
	Album         string
	Duration      int
	Genre         string
	Year          int
	CoverArtId    string `json:"coverArt"`
	// Title is only available for Albums from gonic
	Title string
	// Artists is only available for Entities from gonic
	Artists []Artist
	// MusicBrainzId is only available for Albums from Navidrome
	MusicBrainzId string
}

type Artist struct {
	Id             string
	Name           string
	AlbumCount     int
	ArtistImageUrl string
	Albums         []Album `json:"album"`
}

func (s Artist) ID() string {
	return s.Id
}

type GenreEntries struct {
	Genres []GenreEntry `json:"genre"`
}

type GenreEntry struct {
	SongCount  int    `json:"songCount"`
	AlbumCount int    `json:"albumCount"`
	Name       string `json:"value"`
}

type Album struct {
	EntityBase
	Name      string   `json:"name"`
	SongCount int      `json:"songCount"`
	PlayCount int      `json:"playCount"`
	Songs     Entities `json:"song"`
	Genres    []Genre  `json:"genres"`
	// Compilation is available only from Navidrome
	Compilation bool `json:"isCompilation"`
	// SortName is available only from Navidrome
	SortName string
	// DiscTitles is available only from Navidrome
	DiscTitles []DiscTitle
}

func (s Album) ID() string {
	return s.Id
}

type Genre struct {
	Name string `json:"name"`
}

// Entity could be either a song or a directory, because that's how Subsonic rolls.
type Entity struct {
	EntityBase
	Parent             string
	Path               string
	AlbumId            string
	AlbumArtists       []Artist
	DisplayAlbumArtist string
	BitRate            int
	BitDepth           int
	SamplingRate       int
	ChannelCount       int
	ContentType        string
	IsDirectory        bool `json:"isDir"`
	IsVideo            bool
	Size               int
	Suffix             string
	Track              int
	DiscNumber         int
	Type               string
	ReplayGain         string
}

// #####################################
// Methods allowing Entity to implement
// remote.TrackInterface
// #####################################
func (e Entity) GetId() string {
	return e.Id
}
func (e Entity) GetArtist() string {
	return e.Artist
}
func (e Entity) GetTitle() string {
	return e.Title
}
func (e Entity) GetDuration() int {
	return e.Duration
}
func (e Entity) GetAlbumArtist() string {
	return e.Artist
}
func (e Entity) GetAlbum() string {
	return e.Album
}
func (e Entity) GetTrackNumber() int {
	return e.Track
}
func (e Entity) GetDiscNumber() int {
	return e.DiscNumber
}
func (e Entity) IsValid() bool {
	return true
}

// Return the title if present, otherwise fallback to the file path
func (e Entity) GetSongTitle() string {
	if e.Title != "" {
		return e.Title
	}

	// we get around the weird edge case where a path ends with a '/' by just
	// returning nothing in that instance, which shouldn't happen unless
	// subsonic is being weird
	if e.Path == "" || strings.HasSuffix(e.Path, "/") {
		return ""
	}

	lastSlash := strings.LastIndex(e.Path, "/")

	if lastSlash == -1 {
		return e.Path
	}

	return e.Path[lastSlash+1 : len(e.Path)]
}

// Entities is a sortable list of entities.
// Directories are first, then in alphabelical order. Entities are sorted by
// track number, if they have track numbers; otherwise, they're sorted
// alphabetically.
type Entities []Entity

func (s Entities) Len() int      { return len(s) }
func (s Entities) Swap(i, j int) { s[j], s[i] = s[i], s[j] }
func (s Entities) Less(i, j int) bool {
	// Directories are before tracks, alphabetically
	if s[i].IsDirectory {
		if s[j].IsDirectory {
			return s[i].Title < s[j].Title
		}
		return true
	}
	// Disk and track numbers are only relevant within the same parent
	if s[i].Parent == s[j].Parent {
		// sort first by DiskNumber
		if s[i].DiscNumber == s[j].DiscNumber {
			// Tracks on the same disk are sorted by track
			return s[i].Track < s[j].Track
		}
		return s[i].DiscNumber < s[j].DiscNumber
	}
	// If we get here, the songs are either from different albums, or else
	// they're on the same disk

	return s[i].Title < s[j].Title
}

type DiscTitle struct {
	Disc  int
	Title string
}

type Indexes struct {
	LastModified    int
	IgnoredArticles string
	Index           []Index
}

type Index struct {
	Name    string
	Artists []Artist `json:"artist"`
}

type Playlists struct {
	Playlists []Playlist `json:"playlist"`
}

type Playlist struct {
	Id        Id
	Name      string
	SongCount int
	Comment   string
	Owner     string
	Public    bool
	Duration  int
	Created   string
	Changed   string
	Entries   Entities `json:"entry"`
}

type Info struct{}

type responseWrapper struct {
	Response Response `json:"subsonic-response"`
}

type Response struct {
	Status        string
	Version       string
	Type          string
	ServerVersion string
	OpenSubsonic  bool

	// There's no better way to do this, because Go generics are useless
	RandomSongs   Songs
	SimilarSongs  Songs
	Starred       Results
	SearchResult3 Results
	Directory     Directory
	Album         Album
	Artists       Indexes
	Artist        Artist
	ScanStatus    ScanStatus
	PlayQueue     PlayQueue
	Genres        GenreEntries
	SongsByGenre  Songs
	Indexes       Indexes
	LyricsList    LyricsList
	Playlists     Playlists
	Playlist      Playlist

	Error Error
}

type Id string

func (si *Id) UnmarshalJSON(b []byte) error {
	if b[0] == '"' {
		return json.Unmarshal(b, (*string)(si))
	}
	var i int
	if err := json.Unmarshal(b, &i); err != nil {
		return err
	}
	s := strconv.Itoa(i)
	*si = Id(s)
	return nil
}

type LyricsList struct {
	StructuredLyrics []StructuredLyrics `json:"structuredLyrics"`
}

type StructuredLyrics struct {
	Lang   string       `json:"lang"`
	Synced bool         `json:"synced"`
	Lines  []LyricsLine `json:"line"`
}

type LyricsLine struct {
	Start int64  `json:"start"`
	Value string `json:"value"`
}
