// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2022 Canonical Ltd
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

package image

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/snapcore/snapd/asserts/snapasserts"
	"github.com/snapcore/snapd/snap"
)

// SeedManifestValidationSet represents a validation set as noted
// in the seed manifest. A validation set can optionally be pinned,
// but the sequence will always be set to the sequence that was used
// during the image build.
type SeedManifestValidationSet struct {
	AccountID string
	Name      string
	Sequence  int
	Pinned    bool
}

func (vs *SeedManifestValidationSet) String() string {
	if vs.Pinned {
		return fmt.Sprintf("%s/%s=%d", vs.AccountID, vs.Name, vs.Sequence)
	} else {
		return fmt.Sprintf("%s/%s %d", vs.AccountID, vs.Name, vs.Sequence)
	}
}

// The seed.manifest generated by ubuntu-image contains entries in the following
// format:
// <account-id>/<name>=<sequence>
// <account-id>/<name> <sequence>
// <snap-name> <snap-revision>
type SeedManifest struct {
	valsets       []*SeedManifestValidationSet
	snapRevisions map[string]snap.Revision
	used          map[string]snap.Revision
}

func NewSeedManifest() *SeedManifest {
	return &SeedManifest{
		snapRevisions: make(map[string]snap.Revision),
		used:          make(map[string]snap.Revision),
	}
}

// SeedManifestFromSnapRevisions is only here for usage in tests to simplify
// testing contents of ImageManifest as rules/used are not exported.
func SeedManifestFromSnapRevisions(rules map[string]snap.Revision) *SeedManifest {
	im := NewSeedManifest()
	im.snapRevisions = rules
	return im
}

func (sm *SeedManifest) SetAllowedSnapRevision(snapName string, revision int) error {
	if revision == 0 {
		return fmt.Errorf("cannot add a rule for a zero-value revision")
	}
	sm.snapRevisions[snapName] = snap.R(revision)
	return nil
}

func (sm *SeedManifest) MarkValidationSetUsed(accountID, name string, sequence int, pinned bool) error {
	if sequence <= 0 {
		return fmt.Errorf("cannot mark validation-set used, sequence must be set")
	}

	sm.valsets = append(sm.valsets, &SeedManifestValidationSet{
		AccountID: accountID,
		Name:      name,
		Sequence:  sequence,
		Pinned:    pinned,
	})
	return nil
}

func (sm *SeedManifest) MarkSnapRevisionUsed(snapName string, revision int) error {
	rev := snap.R(revision)
	if rule, ok := sm.snapRevisions[snapName]; ok {
		if rule != rev {
			return fmt.Errorf("revision does not match the value specified by revisions rules (%s != %s)", rev, rule)
		}
	}
	sm.used[snapName] = rev
	return nil
}

func (sm *SeedManifest) AllowedRevision(snapName string) snap.Revision {
	return sm.snapRevisions[snapName]
}

func (sm *SeedManifest) ValidationSets() []*SeedManifestValidationSet {
	return sm.valsets
}

func parsePinnedValidationSet(sm *SeedManifest, vs string) error {
	acc, name, seq, err := snapasserts.ParseValidationSet(vs)
	if err != nil {
		return err
	}
	return sm.MarkValidationSetUsed(acc, name, seq, true)
}

func parseUnpinnedValidationSet(sm *SeedManifest, vs, seqStr string) error {
	acc, name, _, err := snapasserts.ParseValidationSet(vs)
	if err != nil {
		return err
	}
	seq, err := strconv.Atoi(seqStr)
	if err != nil {
		return fmt.Errorf("invalid formatted validation-set sequence: %q", seqStr)
	}
	return sm.MarkValidationSetUsed(acc, name, seq, false)
}

func parseSnapRevision(sm *SeedManifest, sn, revStr string) error {
	if err := snap.ValidateName(sn); err != nil {
		return err
	}

	rev, err := snap.ParseRevision(revStr)
	if err != nil {
		return err
	}

	// Values that are higher than 0 indicate the revision comes from the store, and values
	// lower than 0 indicate the snap was sourced locally. We allow both in the seed.manifest as
	// long as the user can provide us with the correct snaps. The only number we won't accept is
	// 0.
	if rev.Unset() {
		return fmt.Errorf("cannot use revision %d for snap %q: revision must not be 0", rev, sn)
	}
	return sm.SetAllowedSnapRevision(sn, rev.N)
}

// ReadSeedManifest reads a seed.manifest generated by ubuntu-image, and returns
// a map containing the snap names and their revisions.
func ReadSeedManifest(manifestFile string) (*SeedManifest, error) {
	f, err := os.Open(manifestFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sm := NewSeedManifest()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") {
			return nil, fmt.Errorf("line cannot start with any spaces: %q", line)
		}

		tokens := strings.Fields(line)

		if len(tokens) == 1 && strings.Contains(tokens[0], "/") {
			// Pinned validation-set: <account-id>/<name>=<sequence>
			if err := parsePinnedValidationSet(sm, tokens[0]); err != nil {
				return nil, err
			}
		} else if len(tokens) == 2 {
			if strings.Contains(tokens[0], "/") {
				// Unpinned validation-set: <account-id>/<name> <sequence>
				if err := parseUnpinnedValidationSet(sm, tokens[0], tokens[1]); err != nil {
					return nil, err
				}
			} else {
				// Snap revision: <snap> <revision>
				if err := parseSnapRevision(sm, tokens[0], tokens[1]); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, fmt.Errorf("line is illegally formatted: %q", line)
		}
	}
	return sm, nil
}

// Write generates the seed.manifest contents from the provided map of
// snaps and their revisions, and stores them in the given file path
func (sm *SeedManifest) Write(filePath string) error {
	if len(sm.used) == 0 {
		return nil
	}

	keys := make([]string, 0, len(sm.used))
	for k := range sm.used {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := bytes.NewBuffer(nil)
	for _, vs := range sm.valsets {
		fmt.Fprintf(buf, "%s\n", vs)
	}
	for _, key := range keys {
		fmt.Fprintf(buf, "%s %s\n", key, sm.used[key])
	}
	return ioutil.WriteFile(filePath, buf.Bytes(), 0755)
}
