// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019 Canonical Ltd
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

var shortCreateCohortHelp = i18n.G("Create cohort keys for a set of snaps")
var longCreateCohortHelp = i18n.G(`
The create-cohort command creates a set of cohort keys for a given set of snaps.

A cohort is a view or snapshot of a snap's "channel map" at a given point in
time that fixes the set of revisions for the snap given other constraints
(e.g. channel or architecture). The cohort is then identified by an opaque
per-snap key that works across systems. Installations or refreshes of the snap
using a given cohort key would use a fixed revision for up to 90 days, after
which a new set of revisions would be fixed under that same cohort key and a
new 90 days window started.
`)

type cmdCreateCohort struct {
}

func init() {
	addCommand("create-cohort", shortCreateCohortHelp, longCreateCohortHelp, func() flags.Commander { return &cmdCreateCohort{} }, nil, nil)
}

func (x *cmdCreateCohort) Execute(args []string) error {
	return nil
}
