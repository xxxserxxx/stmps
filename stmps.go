// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/spezifisch/stmps/remote"
	"github.com/spezifisch/stmps/subsonic"
	tviewcommand "github.com/spezifisch/tview-command"

	// TODO consider replacing viper with claptrap
	"github.com/spf13/viper"
)

// TODO Update screenshots in the README
// TODO Add mocking library
// TODO Get unit tests up to some non-embarassing percentage
// TODO Merge feature_27_save_queue / issue-54-save-queue-on-exit / seekable-queue-load, and finish the restoring play location on first run, or hotkey

var osExit = os.Exit  // A variable to allow mocking os.Exit in tests
var headlessMode bool // This can be set to true during tests
var testMode bool     // This can be set to true during tests, too

const DEVELOPMENT = "development"

// APIVersion is the OpenSubsonic API version we talk, communicated to the server
const APIVersion = "1.8.0"

// Name is the client name we tell the server
var Name string = "stmps"

// Version is the program version; usually set from BuildInfo
var Version string = DEVELOPMENT

func readConfig(configFile *string) error {
	required_properties := []string{"auth.username", "auth.password", "server.host"}

	if configFile != nil && *configFile != "" {
		// use custom config file
		viper.SetConfigFile(*configFile)
	} else {
		// lookup default dirs
		viper.SetConfigName("stmp") // TODO this should be stmps
		viper.SetConfigType("toml")
		viper.AddConfigPath("$HOME/.config/stmp") // TODO this should be stmps only
		viper.AddConfigPath("$HOME/.config/stmps")
		viper.AddConfigPath(".")
	}

	// read it
	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("Config file error: %s\n", err)
	}

	// validate
	for _, prop := range required_properties {
		if !viper.IsSet(prop) {
			return fmt.Errorf("Config property %s is required\n", prop)
		}
	}

	return nil
}

// parseConfig takes the first non-flag arguments from flags and parses it
// into the viper config.
func parseConfig() {
	if u, e := url.Parse(flag.Arg(0)); e == nil {
		// If credentials were provided
		if len(u.User.Username()) > 0 {
			viper.Set("auth.username", u.User.Username())
			// If the password wasn't provided, the program will fail as normal
			if p, s := u.User.Password(); s {
				viper.Set("auth.password", p)
			}
		}
		// Blank out the credentials so we can use the URL formatting
		u.User = nil
		viper.Set("server.host", u.String())
	} else {
		fmt.Printf("Invalid server format; must be a valid URL!")
		fmt.Printf("Usage: %s <args> [http[s]://[user:pass@]server:port]\n", os.Args[0])
		osExit(1)
	}
}

// initCommandHandler sets up tview-command as main input handler
func initCommandHandler(logger *logger.Logger) {
	tviewcommand.SetLogHandler(func(msg string) {
		logger.Print(msg)
	})

	configPath := "HACK.commands.toml"

	// Load the configuration file
	config, err := tviewcommand.LoadConfig(configPath)
	if err != nil || config == nil {
		logger.PrintError("Failed to load command-shortcut config", err)
	}

	//env := keybinding.SetupEnvironment()
	//keybinding.RegisterCommands(env)
}

// return codes:
// 0 - OK
// 1 - generic errors
// 2 - main config errors
// 2 - keybinding config errors
func main() {
	// parse flags and config
	// TODO help should better explain the arguments, especially the currently undocumented server URL argument
	help := flag.Bool("help", false, "Print usage")
	enableMpris := flag.Bool("mpris", false, "Enable MPRIS2")
	list := flag.Bool("list", false, "list server data")
	pl := flag.Bool("playlists", false, "include playlist info (only used with --list; playlists can take a long time to load)")
	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")
	configFile := flag.String("config", "", "use config `file`")
	version := flag.Bool("version", false, "print the stmps version and exit")

	flag.Parse()
	if *help {
		fmt.Printf("USAGE: %s <args> [[user:pass@]server:port]\n", os.Args[0])
		flag.Usage()
		osExit(0)
	}
	if Version == DEVELOPMENT {
		if bi, ok := debug.ReadBuildInfo(); ok {
			Version = bi.Main.Version
		}
	}
	if *version {
		fmt.Printf("stmps %s", Version)
		osExit(0)
	}

	// cpu/memprofile code straight from https://pkg.go.dev/runtime/pprof
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	// config gathering
	if len(flag.Args()) > 0 {
		parseConfig()
	}

	if err := readConfig(configFile); err != nil {
		if configFile == nil {
			fmt.Fprintf(os.Stderr, "Failed to read configuration: configuration file is nil\n")
		} else {
			fmt.Fprintf(os.Stderr, "Failed to read configuration from file '%s': %v\n", *configFile, err)
		}
		osExit(2)
	}

	logger := logger.Init()
	initCommandHandler(logger)

	// init mpv engine
	player, err := mpvplayer.NewPlayer(logger)
	if err != nil {
		fmt.Println("Unable to initialize mpv. Is mpv installed?")
		osExit(1)
	}

	var mprisPlayer *remote.MprisPlayer
	// init mpris2 player control (linux only but fails gracefully on other systems)
	if *enableMpris {
		mprisPlayer, err = remote.RegisterMprisPlayer(player, logger)
		if err != nil {
			fmt.Printf("Unable to register MPRIS with DBUS: %s\n", err)
			fmt.Println("Try running without MPRIS")
			osExit(1)
		}
		defer mprisPlayer.Close()
	}

	// init macos mediaplayer control
	if runtime.GOOS == "darwin" {
		if err = remote.RegisterMPMediaHandler(player, logger); err != nil {
			fmt.Printf("Unable to initialize MediaPlayer bindings: %s\n", err)
			osExit(1)
		} else {
			logger.Print("MacOS MediaPlayer registered")
		}
	}

	if testMode {
		fmt.Println("Running in test mode for testing.")
		osExit(0x23420001)
		return
	}

	connection := subsonic.Init(logger)
	connection.SetClientInfo(Name, APIVersion)
	connection.Username = viper.GetString("auth.username")
	connection.Password = viper.GetString("auth.password")
	connection.Host = viper.GetString("server.host")
	connection.PlaintextAuth = viper.GetBool("auth.plaintext")
	connection.Scrobble = viper.GetBool("server.scrobble")
	connection.RandomSongNumber = viper.GetUint("client.random-songs")

	artistInd, err := connection.GetArtists()
	if err != nil {
		fmt.Printf("Error fetching indexes from server: %s\n", err)
		osExit(1)
	}
	// Sparse artist information: id, name, albumCount, coverArt, artistImageUrl
	artists := make([]subsonic.Artist, 0)
	artistCount := 0
	albumCount := 0
	for _, ind := range artistInd.Index {
		artistCount += len(ind.Artists)
		for _, art := range ind.Artists {
			albumCount += art.AlbumCount
			artists = append(artists, art)
		}
	}

	if *list {
		var playlists subsonic.Playlists
		var err error
		wg := sync.WaitGroup{}
		wg.Add(1)
		if *pl {
			go func() {
				playlists, err = connection.GetPlaylists()
				wg.Done()
			}()
		}
		if si, err := connection.GetServerInfo(); err == nil {
			fmt.Printf("Server %-20s: %s\n", "status", si.Status)
			fmt.Printf("Server %-20s: %s\n", "Subsonic API version", si.Version)
			fmt.Printf("Server %-20s: %s\n", "type", si.Type)
			fmt.Printf("Server %-20s: %s\n", "version", si.ServerVersion)
			fmt.Printf("Server %-20s: %t\n", "is OpenSubsonic", si.OpenSubsonic)
		} else {
			fmt.Printf("\n  Error fetching playlists from server: %s\n", err)
		}
		indexes, err := connection.GetIndexes()
		fmt.Printf("%-27s: %d\n", "Indexes", len(indexes.Index))
		directoryCount := 0
		subDirCount := 0
		for _, ind := range indexes.Index {
			directoryCount += len(ind.Artists)
			for _, art := range ind.Artists {
				subDirCount += art.AlbumCount
			}
		}
		fmt.Printf("%-27s: %d\n", "Directories", directoryCount)
		fmt.Printf("%-27s: %d\n", "Subdirectories", subDirCount)
		fmt.Printf("%-27s: %d\n", "Artists", artistCount)
		fmt.Printf("%-27s: %d\n", "Albums", albumCount)
		if st, err := connection.GetStarred(); err == nil {
			fmt.Printf("%-27s: %d\n", "Starred albums", len(st.Albums))
			fmt.Printf("%-27s: %d\n", "Starred artists", len(st.Artists))
			fmt.Printf("%-27s: %d\n", "Starred songs", len(st.Songs))
		}
		if *pl {
			var spinnerText []rune = []rune(viper.GetString("ui.spinner"))
			if len(spinnerText) == 0 {
				spinnerText = []rune("┤┘┴└├┌┬┐")
			}
			fmt.Printf("%-27s: %c", "Playlists", spinnerText[0])
			stop := make(chan struct{})
			go func() {
				defer wg.Done()
				spinnerMax := len(spinnerText) - 1
				timer := time.NewTicker(500 * time.Millisecond)
				defer timer.Stop()
				idx := 1
				for {
					select {
					case <-timer.C:
						fmt.Printf("\b%c", spinnerText[idx])
						idx++
						if idx > spinnerMax {
							idx = 0
						}
					case <-stop:
						fmt.Printf("\b")
						return
					}
				}
			}()
			wg.Wait()
			wg.Add(1)
			stop <- struct{}{}
			wg.Wait()
			if err == nil {
				fmt.Printf("%d\n", len(playlists.Playlists))
				for _, pl := range playlists.Playlists {
					if len(pl.Entries) > 0 {
						fmt.Printf("  %25s: %d\n", pl.Name, len(pl.Entries))
					}
				}
			} else {
				fmt.Printf("\n  Error fetching playlists from server: %s\n", err)
			}
		}

		osExit(0)
	}

	if headlessMode {
		fmt.Println("Running in headless mode for testing.")
		osExit(0)
		return
	}

	ui := InitGui(artists, connection, player, logger, mprisPlayer)

	// run main loop
	if err := ui.Run(); err != nil {
		panic(err)
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close() // error handling omitted for example
		runtime.GC()    // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}
