// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2018 Canonical Ltd
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

package main

import (
	"github.com/jessevdk/go-flags"

	"github.com/snapcore/snapd/i18n"
)

type cmdAdviseSnap struct {
	Positionals struct {
		CommandOrPkg string
	} `positional-args:"true"`

	Format string `long:"format" default:"pretty" choice:"pretty" choice:"json"`
	// Command makes advise try to find snaps that provide this command
	Command bool `long:"command"`

	// FromApt tells advise that it got started from an apt hook
	// and needs to communicate over a socket
	FromApt bool `long:"from-apt"`

	// DumpDb dumps the whole advise database
	DumpDb bool `long:"dump-db"`
}

var shortAdviseSnapHelp = i18n.G("Advise on available snaps")
var longAdviseSnapHelp = i18n.G(`
The advise-snap command searches for and suggests the installation of snaps.

If --command is given, it suggests snaps that provide the given command.
Otherwise it suggests snaps with the given name.
`)

func init() {
	cmd := addCommand("advise-snap", shortAdviseSnapHelp, longAdviseSnapHelp, func() flags.Commander {
		return &cmdAdviseSnap{}
	}, map[string]string{
		// TRANSLATORS: This should not start with a lowercase letter.
		"command": i18n.G("Advise on snaps that provide the given command"),
		// TRANSLATORS: This should not start with a lowercase letter.
		"dump-db": i18n.G("Dump advise database for use by command-not-found."),
		// TRANSLATORS: This should not start with a lowercase letter.
		"from-apt": i18n.G("Run as an apt hook"),
		// TRANSLATORS: This should not start with a lowercase letter.
		"format": i18n.G("Use the given output format"),
	}, []argDesc{
		// TRANSLATORS: This needs to begin with < and end with >
		{name: i18n.G("<command or pkg>")},
	})
	cmd.hidden = true
}

func (x *cmdAdviseSnap) Execute(args []string) error {
	return nil
}

