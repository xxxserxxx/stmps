// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package remote

import (
	"errors"
	"math"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
	"github.com/wildeyedskies/stmp/logger"
	"github.com/wildeyedskies/stmp/mpvplayer"
)

type MprisPlayer struct {
	dbus   *dbus.Conn
	player *mpvplayer.Player
	logger logger.LoggerInterface

	lastVolume float64
}

func RegisterMprisPlayer(player *mpvplayer.Player, logger_ logger.LoggerInterface) (mpp *MprisPlayer, err error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}

	parts := []string{"", "org", "mpris", "MediaPlayer2", "Player"}
	name := strings.Join(parts[1:], ".")
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

	propSpec := map[string]map[string]*prop.Prop{
		"org.mpris.MediaPlayer2.Player": {
			"CanControl":     {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"CanGoNext":      {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"CanPause":       {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"CanPlay":        {Value: true, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"CanSeek":        {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"CanGoPrevious":  {Value: false, Writable: false, Emit: prop.EmitFalse, Callback: nil},
			"Metadata":       {Value: metadata, Writable: false, Emit: prop.EmitTrue, Callback: nil},
			"Volume":         {Value: float64(0.0), Writable: true, Emit: prop.EmitTrue, Callback: mpp.volumeChange},
			"PlaybackStatus": {Value: "", Writable: false, Emit: prop.EmitFalse, Callback: nil},
		},
	}

	props, err := prop.Export(conn, "/org/mpris/MediaPlayer2", propSpec)
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
				Properties: props.Introspection("org.mpris.MediaPlayer2.Player"),
			},
		},
	}
	err = conn.Export(introspect.NewIntrospectable(n), "/org/mpris/MediaPlayer2", "org.freedesktop.DBus.Introspectable")
	if err != nil {
		return
	}

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
	if err := m.player.PlayNextTrack(); err != nil {
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
	// get volume change value as float where 1.0 = 100%
	fVol := c.Value.(float64)
	fDelta := fVol - m.lastVolume
	m.lastVolume = fVol

	// convert to %
	pcDelta := int64(math.Round(fDelta * 100))
	if err := m.player.AdjustVolume(pcDelta); err != nil {
		m.logger.PrintError("volumeChange", err)
	} else {
		m.logger.Printf("mpris: adjust volume %f d%f -> %d%%", fVol, fDelta, pcDelta)
	}
	return nil
}
