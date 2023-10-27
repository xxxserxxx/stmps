// Copyright 2023 The STMP Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "github.com/wildeyedskies/stmp/mpvplayer"

func (ui *Ui) SendEvent(event mpvplayer.UiEvent) {
	ui.mpvEvents <- event
}
