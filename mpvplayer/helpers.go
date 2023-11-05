// Copyright 2023 The STMPS Authors
// SPDX-License-Identifier: GPL-3.0-only

package mpvplayer

import (
	"errors"

	"github.com/spezifisch/go-mpv"
)

func (p *Player) getPropertyInt64(name string) (int64, error) {
	value, err := p.instance.GetProperty(name, mpv.FORMAT_INT64)
	if err != nil {
		return 0, err
	} else if value == nil {
		return 0, errors.New("nil value")
	}
	return value.(int64), err
}

func (p *Player) getPropertyBool(name string) (bool, error) {
	value, err := p.instance.GetProperty(name, mpv.FORMAT_FLAG)
	if err != nil {
		return false, err
	} else if value == nil {
		return false, errors.New("nil value")
	}
	return value.(bool), err
}
