// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package remote

import (
	"errors"
	"math"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/spezifisch/stmps/logger"
)

type MprisPlayer struct {
	dbus   *dbus.Conn
	player ControlledPlayer
	logger logger.LoggerInterface

	metadata map[string]interface{}
}

func RegisterMprisPlayer(player ControlledPlayer, logger_ logger.LoggerInterface) (mpp *MprisPlayer, err error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}

	mpp = &MprisPlayer{
		dbus:   conn,
		player: player,
		logger: logger_,
		metadata: map[string]interface{}{
			"mpris:trackid":     "",
			"mpris:length":      int64(0),
			"xesam:album":       "",
			"xesam:albumArtist": "",
			"xesam:artist":      []string{},
			"xesam:composer":    []string{},
			"xesam:genre":       []string{},
			"xesam:title":       "",
			"xesam:trackNumber": int(0),
		},
	}

	var mprisPlayer = map[string]*prop.Prop{
		"CanControl":     {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanGoNext":      {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanPause":       {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanPlay":        {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanSeek":        {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanGoPrevious":  {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"Metadata":       {Value: mpp.metadata, Writable: false, Emit: prop.EmitTrue, Callback: nil},
		"Volume":         {Value: float64(0.0), Writable: true, Emit: prop.EmitTrue, Callback: mpp.volumeChange},
		"PlaybackStatus": {Value: "", Writable: false, Emit: prop.EmitFalse, Callback: nil},
	}

	var mediaPlayer = map[string]*prop.Prop{
		"CanQuit":             {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"HasTrackList":        {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"Identity":            {Value: "stmps", Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"IconName":            {Value: "stmps-icon", Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"SupportedUriSchemes": {Value: "", Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"SupportedMimeTypes":  {Value: "", Writable: false, Emit: prop.EmitFalse, Callback: nil},
	}

	props, err := prop.Export(
		conn,
		"/org/mpris/MediaPlayer2",
		map[string]map[string]*prop.Prop{
			"org.mpris.MediaPlayer2":        mediaPlayer,
			"org.mpris.MediaPlayer2.Player": mprisPlayer,
		},
	)
	if err != nil {
		logger_.PrintError("prop.Export error", err)
		return
	}

	n := &introspect.Node{
		Name: "/org/mpris/MediaPlayer2",
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:       "org.mpris.MediaPlayer2.Player",
				Methods:    introspect.Methods(mpp),
				Properties: props.Introspection("org.mpris.MediaPlayer2.Player"), // we implement the standard interface
			},
			{
				Name:       "org.mpris.MediaPlayer2",
				Methods:    []introspect.Method{},
				Properties: props.Introspection("org.mpris.MediaPlayer2"),
			},
		},
	}
	err = conn.Export(introspect.NewIntrospectable(n), "/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Introspectable")
	if err != nil {
		logger_.PrintError("conn.Export error", err)
		return
	}

	// our unique name
	name := "org.mpris.MediaPlayer2.stmps"
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil {
		logger_.PrintError("conn.RequestName error", err)
		return
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		err = errors.New("name already owned")
		logger_.PrintError("conn.RequestName reply error", err)
		return
	}
	return
}

func (m *MprisPlayer) Close() {
	if err := m.dbus.Close(); err != nil {
		m.logger.PrintError("mpp Close", err)
	}
}

// Mandatory functions
func (m *MprisPlayer) Stop() {
	if err := m.player.Stop(); err != nil {
		m.logger.PrintError("mpp Stop", err)
	}
}

func (m *MprisPlayer) Next() {
	if err := m.player.NextTrack(); err != nil {
		m.logger.PrintError("mpp PlayNextTrack", err)
	}
	//TODO updateQueueList(ui.player, ui.queueList, ui.starIdList)
}

// set paused
func (m *MprisPlayer) Pause() {
	if paused, err := m.player.IsPaused(); err != nil {
		m.logger.PrintError("mpp IsPaused", err)
	} else if !paused {
		if err = m.player.Pause(); err != nil {
			m.logger.PrintError("mpp Pause", err)
		}
	}
}

// set playing
func (m *MprisPlayer) Play() {
	if playing, err := m.player.IsPlaying(); err != nil {
		m.logger.PrintError("mpp IsPlaying", err)
	} else if !playing {
		if err = m.player.Pause(); err != nil {
			m.logger.PrintError("mpp Pause", err)
		}
	}
}

func (m *MprisPlayer) PlayPause() {
	if err := m.player.Pause(); err != nil {
		m.logger.PrintError("mpp Pause", err)
	}
}

func (m *MprisPlayer) OpenUri(string) {
	// TODO not implemented
}
func (m *MprisPlayer) Previous() {
	// TODO not implemented
}
func (m *MprisPlayer) Seek(int) {
	// TODO not implemented
}
func (m *MprisPlayer) Seeked(int) {
	// TODO not implemented
}
func (m *MprisPlayer) SetPosition(string, int) {
	// TODO not implemented
}

func (m *MprisPlayer) volumeChange(c *prop.Change) *dbus.Error {
	fVol := c.Value.(float64)

	// convert to %
	percentVol := int(math.Round(fVol * 100))
	if err := m.player.SetVolume(percentVol); err != nil {
		m.logger.PrintError("volumeChange", err)
	} else {
		m.logger.Printf("mpris: adjust volume %f -> %d%%", fVol, percentVol)
	}
	return nil
}

// OnSongChange method to be called by eventLoop
func (m *MprisPlayer) OnSongChange(currentSong TrackInterface) {
	m.metadata["mpris:trackid"] = "/org/mpris/MediaPlayer2/track/" + currentSong.GetId()
	m.metadata["mpris:length"] = int64(currentSong.GetDuration() * 1000000) // Duration in microseconds
	m.metadata["xesam:album"] = currentSong.GetAlbum()                      // Album name
	m.metadata["xesam:albumArtist"] = currentSong.GetAlbumArtist()          // Album artist
	m.metadata["xesam:artist"] = []string{currentSong.GetArtist()}          // List of artists
	m.metadata["xesam:composer"] = []string{}                               // List of composers, empty
	m.metadata["xesam:genre"] = []string{}                                  // List of genres, empty
	m.metadata["xesam:title"] = currentSong.GetTitle()                      // Track title
	m.metadata["xesam:trackNumber"] = currentSong.GetTrackNumber()          // Track number

	m.logger.Printf("mpris: Updated metadata: %+v", m.metadata)

	// Emit the PropertiesChanged signal to notify clients about the metadata change
	err := m.dbus.Emit("/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Properties.PropertiesChanged",
		"org.mpris.MediaPlayer2.Player", map[string]interface{}{
			"Metadata": m.metadata,
		}, []string{})

	if err != nil {
		m.logger.PrintError("mpris: Emit PropertiesChanged", err)
	}
}
