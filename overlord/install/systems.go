// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2023 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package install

import (
	"fmt"
	"os"

	"github.com/snapcore/snapd/boot"
)

func MaybeReadModeenv() (*boot.Modeenv, error) {
	modeenv, err := boot.ReadModeenv("")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot read modeenv: %v", err)
	}
	return modeenv, nil
}

type SystemAction struct {
	Title string
	Mode  string
}

type System struct {
	// Current is true when the system running now was installed from that
	// seed
	Current bool
	// Label of the seed system
	Label string
	// Model assertion of the system
	Model *asserts.Model
	// Brand information
	Brand *asserts.Account
	// Actions available for this system
	Actions []SystemAction
}

var defaultSystemActions = []SystemAction{
	{Title: "Install", Mode: "install"},
}
var currentSystemActions = []SystemAction{
	{Title: "Reinstall", Mode: "install"},
	{Title: "Recover", Mode: "recover"},
	{Title: "Factory reset", Mode: "factory-reset"},
	{Title: "Run normally", Mode: "run"},
}
var recoverSystemActions = []SystemAction{
	{Title: "Reinstall", Mode: "install"},
	{Title: "Factory reset", Mode: "factory-reset"},
	{Title: "Run normally", Mode: "run"},
}

