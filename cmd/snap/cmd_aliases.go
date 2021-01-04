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

type cmdAliases struct {
	clientMixin
	Positionals struct {
		Snap installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"true"`
}

var shortAliasesHelp = i18n.G("List aliases in the system")
var longAliasesHelp = i18n.G(`
The aliases command lists all aliases available in the system and their status.

$ snap aliases <snap>

Lists only the aliases defined by the specified snap.

An alias noted as undefined means it was explicitly enabled or disabled but is
not defined in the current revision of the snap, possibly temporarily (e.g.
because of a revert). This can cleared with 'snap alias --reset'.
`)

func init() {
	addCommand("aliases", shortAliasesHelp, longAliasesHelp, func() flags.Commander {
		return &cmdAliases{}
	}, nil, nil)
}

type aliasInfo struct {
	Snap    string
	Command string
	Alias   string
	Status  string
	Auto    string
}

func (x *cmdAliases) Execute(args []string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}
