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

package builtin

const bootUpdateSummary = `allows reading information about system hardware`

const bootUpdateBaseDeclarationSlots = `
  boot-update:
    allow-installation:
      slot-snap-type:
        - core
      plug-snap-type:
        - gadget
        - kernel
    deny-auto-connection: true
`

const bootUpdateConnectedPlugAppArmor = `
# Description: This interface allows for updating boot components
# of the system. This is reserved because it allows performing potentially
# ireversible changes

# Access boot components
/boot/**/ rw,
/boot/**/** rw,

# disk access
/dev/sd* rw,
/dev/mmcblk* rw,
/dev/disk/** r,
`

func init() {
	registerIface(&commonInterface{
		name:                  "boot-update",
		summary:               bootUpdateSummary,
		implicitOnCore:        true,
		implicitOnClassic:     true,
		baseDeclarationSlots:  bootUpdateBaseDeclarationSlots,
		connectedPlugAppArmor: bootUpdateConnectedPlugAppArmor,
		reservedForOS:         true,
	})
}
