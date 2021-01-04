// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2014-2020 Canonical Ltd
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

type cmdAutoImport struct {
	clientMixin
	Mount []string `long:"mount" arg-name:"<device path>"`

	ForceClassic bool `long:"force-classic"`
}

var shortAutoImportHelp = i18n.G("Inspect devices for actionable information")

var longAutoImportHelp = i18n.G(`
The auto-import command searches available mounted devices looking for
assertions that are signed by trusted authorities, and potentially
performs system changes based on them.

If one or more device paths are provided via --mount, these are temporarily
mounted to be inspected as well. Even in that case the command will still
consider all available mounted devices for inspection.

Assertions to be imported must be made available in the auto-import.assert file
in the root of the filesystem.
`)

func init() {
	cmd := addCommand("auto-import",
		shortAutoImportHelp,
		longAutoImportHelp,
		func() flags.Commander {
			return &cmdAutoImport{}
		}, map[string]string{
			// TRANSLATORS: This should not start with a lowercase letter.
			"mount": i18n.G("Temporarily mount device before inspecting"),
			// TRANSLATORS: This should not start with a lowercase letter.
			"force-classic": i18n.G("Force import on classic systems"),
		}, nil)
	cmd.hidden = true
}

func (x *cmdAutoImport) Execute(args []string) error {
	return nil
}
