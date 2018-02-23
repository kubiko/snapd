// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2016-2017 Canonical Ltd
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

package builtin

const buildToolsSummary = `allows building on device`

const buildToolsBaseDeclarationSlots = `
  build-tools:
    allow-installation:
      slot-snap-type:
        - core
    deny-auto-connection: true
`

const buildToolsConnectedPlugAppArmor = `
# Description: Allows building on device, use of build tools

# Capabilities needed by npm install
capability dac_override,
capability chown,
capability fowner,
capability fsetid,
`
const buildToolsConnectedPlugSecComp = `
# Description: permissions to use build tools
chown
fchown
`

func init() {
	registerIface(&commonInterface{
		name:                  "build-tools",
		summary:               buildToolsSummary,
		implicitOnCore:        true,
		implicitOnClassic:     true,
		baseDeclarationSlots:  buildToolsBaseDeclarationSlots,
		connectedPlugAppArmor: buildToolsConnectedPlugAppArmor,
		connectedPlugSecComp:  buildToolsConnectedPlugSecComp,
		reservedForOS:         true,
	})
}
