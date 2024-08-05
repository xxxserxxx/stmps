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
	}

	err = conn.ExportAll(mpp, "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player")
	if err != nil {
		return
	}
	/*
		func (mpp MprisPlayer) Metadata() string {
			if len(mpp.player.Queue) == 0 {
				return ""
			}
			playing := mpp.player.Queue[0]
			return fmt.Sprintf("%s - %s", playing.Artist, playing.Title)
		}
		Shuffle true/false
		LoopStatus "Noneon, "Track", "Playlist"
		Position time_in_us
		MaximumRate, Rate, MinimumRate (float 0-1, x speed)
	*/
	metadata := map[string]interface{}{
		"mpris:trackid":     "",
		"mpris:length":      int64(0),
		"xesam:album":       "",
		"xesam:albumArtist": "",
		"xesam:artist":      []string{},
		"xesam:composer":    []string{},
		"xesam:genre":       []string{},
		"xesam:title":       "",
		"xesam:trackNumber": int(0),
	}

	var mprisPlayer = map[string]*prop.Prop{
		"CanControl":     {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanGoNext":      {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanPause":       {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanPlay":        {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanSeek":        {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanGoPrevious":  {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"Metadata":       {Value: metadata, Writable: false, Emit: prop.EmitTrue, Callback: nil},
		"Volume":         {Value: float64(0.0), Writable: true, Emit: prop.EmitTrue, Callback: mpp.volumeChange},
		"PlaybackStatus": {Value: "", Writable: false, Emit: prop.EmitFalse, Callback: nil},
	}

	var mediaPlayer = map[string]*prop.Prop{
		"CanQuit":             {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"CanRaise":            {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"HasTrackList":        {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
		"Identity":            {Value: "stmps", Writable: false, Emit: prop.EmitFalse, Callback: nil},
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
		},
	}
	err = conn.Export(introspect.NewIntrospectable(n), "/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Introspectable")
	if err != nil {
		return
	}

	// our unique name
	name := "org.mpris.MediaPlayer2.stmps"
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err != nil {
		return
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		err = errors.New("name already owned")
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
	m.logger.Print("mpris: OnSongChange called")

	metadata := map[string]interface{}{
		"mpris:trackid":     "",
		"mpris:length":      int64(currentSong.GetDuration() * 1000000), // duration in microseconds
		"xesam:album":       "",
		"xesam:albumArtist": "",
		"xesam:artist":      []string{currentSong.GetArtist()},
		"xesam:composer":    []string{},
		"xesam:genre":       []string{},
		"xesam:title":       currentSong.GetTitle(),
		"xesam:trackNumber": 0,
	}

	m.logger.Printf("mpris: Emitting PropertiesChanged with metadata: %+v", metadata)

	err := m.dbus.Emit("/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Properties.PropertiesChanged",
		"org.mpris.MediaPlayer2.Player", map[string]map[string]interface{}{
			"Metadata": metadata,
		}, []string{})

	if err != nil {
		m.logger.PrintError("mpris: Emit PropertiesChanged", err)
	}
}
