# STMP (subsonic terminal music player)

A terminal client for *sonic music servers. Inspired by ncmpcpp.

[![stmp](https://img.shields.io/aur/version/stmp)](https://aur.archlinux.org/packages/stmp/)
[![stmp-git](https://img.shields.io/aur/version/stmp-git)](https://aur.archlinux.org/packages/stmp-git/)
[![GitHub license](https://img.shields.io/github/license/wildeyedskies/stmp)](https://github.com/wildeyedskies/stmp/blob/main/LICENSE)

## Features

* browse by folder
* queue songs and albums
* volume control

## Dependencies

[mpv](https://mpv.io):

* Linux (Debian/Ubuntu): `apt install libmpv libmpv-dev`
* MacOS (Homebrew): `brew install mpv` (not the cask)

Go build dependencies

* Go 1.16+
* [tview](https://github.com/rivo/tview)
* [go-mpv](https://github.com/yourok/go-mpv/mpv)

### OSX path setup

On OSX if you installed mpv with brew you may need to set the following paths:

```shell
export C_INCLUDE_PATH=/opt/homebrew/include:$C_INCLUDE_PATH
export LIBRARY_PATH=/opt/homebrew/lib:$LIBRARY_PATH
```

## Compiling

stmp should compile normally with `go build`. Cgo is needed for linking the
libmpv header.

## Configuration

stmp looks for a config file called `stmp.toml` in either `$HOME/.config/stmp`
or the directory in which the executible is placed.

### Example configuration

```toml
[auth]
username = 'admin'
password = 'password'
plaintext = true  # Use 'legacy' unsalted password auth. (default: false)

[server]
host = 'https://your-subsonic-host.tld'
scrobble = true   # Use Subsonic scrobbling for last.fm/ListenBrainz (default: false)
```

## Usage

* Q - quit
* 1 - folder view
* 2 - queue view
* 3 - playlist view
* 4 - log (errors, etc) view
* enter - play song (clears current queue)
* d/delete - remove currently selected song from the queue
* D - remove all songs from queue
* a - add album or song to queue
* p - play/pause
* -/= volume down/volume up
* / - Search artists
* n - Continue search forward
* N - Continue search backwards
* r - refresh the list (if in artist directory, only refreshes that artist)
* s - add 50 random songs to the queue
* y - toggle star on song
