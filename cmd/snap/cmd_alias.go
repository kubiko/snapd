// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016 Canonical Ltd
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
	"fmt"

	"github.com/jessevdk/go-flags"

	"github.com/snapcore/snapd/i18n"
)

type cmdAlias struct {
	waitMixin
	Positionals struct {
		SnapApp appName `required:"yes"`
		Alias   string  `required:"yes"`
	} `positional-args:"true"`
}

// TODO: implement a completer for snapApp

var shortAliasHelp = i18n.G("Set up a manual alias")
var longAliasHelp = i18n.G(`
The alias command aliases the given snap application to the given alias.

Once this manual alias is setup the respective application command can be
invoked just using the alias.
`)

func init() {
	addCommand("alias", shortAliasHelp, longAliasHelp, func() flags.Commander {
		return &cmdAlias{}
	}, waitDescs, []argDesc{
		{name: "<snap.app>"},
		// TRANSLATORS: This needs to begin with < and end with >
		{name: i18n.G("<alias>")},
	})
}

func (x *cmdAlias) Execute(args []string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}
