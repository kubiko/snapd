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
	"fmt"

	"github.com/jessevdk/go-flags"

	"github.com/snapcore/snapd/i18n"
)


type savedCmd struct {
	clientMixin
	durationMixin
	ID         snapshotID `long:"id"`
	Positional struct {
		Snaps []installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"yes"`
}

func (x *savedCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

type saveCmd struct {
	waitMixin
	durationMixin
	Users      string `long:"users"`
	Positional struct {
		Snaps []installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"yes"`
}

func (x *saveCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

type forgetCmd struct {
	waitMixin
	Positional struct {
		ID    snapshotID          `positional-arg-name:"<id>"`
		Snaps []installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"yes" required:"yes"`
}

func (x *forgetCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

type checkSnapshotCmd struct {
	waitMixin
	Users      string `long:"users"`
	Positional struct {
		ID    snapshotID          `positional-arg-name:"<id>"`
		Snaps []installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"yes" required:"yes"`
}

func (x *checkSnapshotCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

type restoreCmd struct {
	waitMixin
	Users      string `long:"users"`
	Positional struct {
		ID    snapshotID          `positional-arg-name:"<id>"`
		Snaps []installedSnapName `positional-arg-name:"<snap>"`
	} `positional-args:"yes" required:"yes"`
}

func (x *restoreCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

func init() {
	addCommand("saved",
		i18n.G("List currently stored snapshots"),
		i18n.G("List currently stored snapshots"),
		func() flags.Commander {
			return &savedCmd{}
		},
		durationDescs.also(map[string]string{
			// TRANSLATORS: This should not start with a lowercase letter.
			"id": i18n.G("Show only a specific snapshot."),
		}),
		nil)

	addCommand("save",
		i18n.G("Save a snapshot of the current data"),
		i18n.G("Save a snapshot of the current data"),
		func() flags.Commander {
			return &saveCmd{}
		}, durationDescs.also(waitDescs).also(map[string]string{
			// TRANSLATORS: This should not start with a lowercase letter.
			"users": i18n.G("Snapshot data of only specific users (comma-separated) (default: all users)"),
		}), nil)

	addCommand("restore",
		i18n.G("Restore a snapshot"),
		i18n.G("Restore a snapshot"),
		func() flags.Commander {
			return &restoreCmd{}
		}, waitDescs.also(map[string]string{
			// TRANSLATORS: This should not start with a lowercase letter.
			"users": i18n.G("Restore data of only specific users (comma-separated) (default: all users)"),
		}), []argDesc{
			{
				name: "<id>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("Set id of snapshot to restore (see 'snap help saved')"),
			}, {
				name: "<snap>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("The snap for which data will be restored"),
			},
		})

	addCommand("forget",
		i18n.G("Delete a snapshot"),
		i18n.G("Delete a snapshot"),
		func() flags.Commander {
			return &forgetCmd{}
		}, waitDescs, []argDesc{
			{
				name: "<id>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("Set id of snapshot to delete (see 'snap help saved')"),
			}, {
				name: "<snap>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("The snap for which data will be deleted"),
			},
		})

	addCommand("check-snapshot",
		i18n.G("Check a snapshot"),
		i18n.G("Check a snapshot"),
		func() flags.Commander {
			return &checkSnapshotCmd{}
		}, waitDescs.also(map[string]string{
			// TRANSLATORS: This should not start with a lowercase letter.
			"users": i18n.G("Check data of only specific users (comma-separated) (default: all users)"),
		}), []argDesc{
			{
				name: "<id>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("Set id of snapshot to verify (see 'snap help saved')"),
			}, {
				name: "<snap>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("The snap for which data will be verified"),
			},
		})

	addCommand("export-snapshot",
		i18n.G("Export a snapshot"),
		i18n.G("Export a snapshot"),
		func() flags.Commander {
			return &exportSnapshotCmd{}
		}, nil, []argDesc{
			{
				name: "<id>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("Set id of snapshot to export"),
			},
			{
				// TRANSLATORS: This should retain < ... >. The file name is the name of an exported snapshot.
				name: i18n.G("<filename>"),
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("The filename of the export"),
			},
		})

	addCommand("import-snapshot",
		i18n.G("Import a snapshot"),
		i18n.G("Import a snapshot"),
		func() flags.Commander {
			return &importSnapshotCmd{}
		}, nil, []argDesc{
			{
				name: "<filename>",
				// TRANSLATORS: This should not start with a lowercase letter.
				desc: i18n.G("Name of the snapshot export file to use"),
			},
		})
}

type exportSnapshotCmd struct {
	clientMixin
	Positional struct {
		ID       snapshotID `positional-arg-name:"<id>"`
		Filename string     `long:"filename"`
	} `positional-args:"yes" required:"yes"`
}

func (x *exportSnapshotCmd) Execute([]string) (err error) {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}

type importSnapshotCmd struct {
	clientMixin
	durationMixin
	Positional struct {
		Filename string `long:"filename"`
	} `positional-args:"yes" required:"yes"`
}

func (x *importSnapshotCmd) Execute([]string) error {
	fmt.Fprintf(Stdout, i18n.G("Not supported\n"))
	return nil
}
