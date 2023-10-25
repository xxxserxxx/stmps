package main

import "github.com/wildeyedskies/stmp/mpv"

func (ui *Ui) SendEvent(event mpv.UiEvent) {
	ui.mpvEvents <- event
}
