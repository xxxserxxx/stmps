package main

import "github.com/wildeyedskies/stmp/mpvplayer"

func (ui *Ui) SendEvent(event mpvplayer.UiEvent) {
	ui.mpvEvents <- event
}
