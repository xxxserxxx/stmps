// Copyright 2023 The STMP Authors
// Copyright 2023 Drew Weymouth and contributors, zackslash
// SPDX-License-Identifier: GPL-3.0-only

//go:build !darwin

package remote

import (
	"errors"

	"github.com/wildeyedskies/stmp/logger"
)

func RegisterMPMediaHandler(_ ControlledPlayer, _ logger.LoggerInterface) error {
	// MPMediaHandler only supports macOS.
	return errors.New("unsupported platform")
}
