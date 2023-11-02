// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/spezifisch/stmps/remote"
	"github.com/spezifisch/stmps/subsonic"
	"github.com/spf13/viper"
)

func readConfig() {
	required_properties := []string{"auth.username", "auth.password", "server.host"}

	viper.SetConfigName("stmp")
	viper.SetConfigType("toml")
	viper.AddConfigPath("$HOME/.config/stmp")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		fmt.Printf("Config file error: %s \n", err)
		os.Exit(1)
	}

	for _, prop := range required_properties {
		if !viper.IsSet(prop) {
			fmt.Printf("Config property %s is required\n", prop)
		}
	}
}

func main() {
	help := flag.Bool("help", false, "Print usage")
	enableMpris := flag.Bool("mpris", false, "Enable MPRIS2")
	flag.Parse()
	if *help {
		fmt.Printf("USAGE: %s <args>\n", os.Args[0])
		flag.Usage()
		os.Exit(0)
	}

	readConfig()

	logger := logger.Init()

	connection := subsonic.Init(logger)
	connection.Username = viper.GetString("auth.username")
	connection.Password = viper.GetString("auth.password")
	connection.Host = viper.GetString("server.host")
	connection.PlaintextAuth = viper.GetBool("auth.plaintext")
	connection.Scrobble = viper.GetBool("server.scrobble")

	indexResponse, err := connection.GetIndexes()
	if err != nil {
		fmt.Printf("Error fetching indexes from server: %s\n", err)
		os.Exit(1)
	}
	playlistResponse, err := connection.GetPlaylists()
	if err != nil {
		fmt.Printf("Error fetching indexes from server: %s\n", err)
		os.Exit(1)
	}

	// init mpv engine
	player, err := mpvplayer.NewPlayer(logger)
	if err != nil {
		fmt.Println("Unable to initialize mpv. Is mpv installed?")
		os.Exit(1)
	}

	// init mpris2 player control (linux only but fails gracefully on other systems)
	if *enableMpris {
		mpris, err := remote.RegisterMprisPlayer(player, logger)
		if err != nil {
			fmt.Printf("Unable to register MPRIS with DBUS: %s\n", err)
			fmt.Println("Try running without MPRIS")
			os.Exit(1)
		}
		defer mpris.Close()
	}

	// init macos mediaplayer control
	if runtime.GOOS == "darwin" {
		if err = remote.RegisterMPMediaHandler(player, logger); err != nil {
			fmt.Printf("Unable to initialize MediaPlayer bindings: %s\n", err)
			os.Exit(1)
		} else {
			logger.Print("MacOS MediaPlayer registered")
		}
	}

	ui := InitGui(&indexResponse.Indexes.Index,
		&playlistResponse.Playlists.Playlists,
		connection,
		player,
		logger)

	// run main loop
	if err := ui.Run(); err != nil {
		panic(err)
	}
}
