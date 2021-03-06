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

package backend_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/check.v1"

	"github.com/snapcore/snapd/client"
	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/logger"
	"github.com/snapcore/snapd/osutil/sys"
	"github.com/snapcore/snapd/overlord/snapshotstate/backend"
	"github.com/snapcore/snapd/snap"
)

type snapshotSuite struct {
	root      string
	restore   []func()
	tarPath   string
	isTesting bool
}

// silly wrappers to get better failure messages
type isTestingSuite struct{ snapshotSuite }
type noTestingSuite struct{ snapshotSuite }

var _ = check.Suite(&isTestingSuite{snapshotSuite{isTesting: true}})
var _ = check.Suite(&noTestingSuite{snapshotSuite{isTesting: false}})

// tie gocheck into testing
func TestSnapshot(t *testing.T) { check.TestingT(t) }

type tableT struct {
	dir     string
	name    string
	content string
}

func table(si snap.PlaceInfo, homeDir string) []tableT {
	return []tableT{
		{
			dir:     si.DataDir(),
			name:    "foo",
			content: "versioned system canary\n",
		}, {
			dir:     si.CommonDataDir(),
			name:    "bar",
			content: "common system canary\n",
		}, {
			dir:     si.UserDataDir(homeDir),
			name:    "ufoo",
			content: "versioned user canary\n",
		}, {
			dir:     si.UserCommonDataDir(homeDir),
			name:    "ubar",
			content: "common user canary\n",
		},
	}
}

func (s *snapshotSuite) SetUpTest(c *check.C) {
	s.root = c.MkDir()

	dirs.SetRootDir(s.root)

	si := snap.MinimalPlaceInfo("hello-snap", snap.R(42))

	for _, t := range table(si, filepath.Join(dirs.GlobalRootDir, "home/snapuser")) {
		c.Check(os.MkdirAll(t.dir, 0755), check.IsNil)
		c.Check(ioutil.WriteFile(filepath.Join(t.dir, t.name), []byte(t.content), 0644), check.IsNil)
	}

	cur, err := user.Current()
	c.Assert(err, check.IsNil)

	s.restore = append(s.restore, backend.MockUserLookup(func(username string) (*user.User, error) {
		if username != "snapuser" {
			return nil, user.UnknownUserError(username)
		}
		rv := *cur
		rv.Username = username
		rv.HomeDir = filepath.Join(dirs.GlobalRootDir, "home/snapuser")
		return &rv, nil
	}),
		backend.MockIsTesting(s.isTesting),
	)

	s.tarPath, err = exec.LookPath("tar")
	c.Assert(err, check.IsNil)
}

func (s *snapshotSuite) TearDownTest(c *check.C) {
	dirs.SetRootDir("")
	for _, restore := range s.restore {
		restore()
	}
}

func hashkeys(snapshot *client.Snapshot) (keys []string) {
	for k := range snapshot.SHA3_384 {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func (s *snapshotSuite) TestLastSnapshotID(c *check.C) {
	// LastSnapshotSetID is happy without any snapshots
	setID, err := backend.LastSnapshotSetID()
	c.Assert(err, check.IsNil)
	c.Check(setID, check.Equals, uint64(0))

	// create snapshots dir and dummy snapshots
	os.MkdirAll(dirs.SnapshotsDir, os.ModePerm)
	for _, name := range []string{
		"9_some-snap-1.zip", "1234_not-a-snapshot", "12_other-snap.zip", "3_foo.zip",
	} {
		c.Assert(ioutil.WriteFile(filepath.Join(dirs.SnapshotsDir, name), []byte{}, 0644), check.IsNil)
	}
	setID, err = backend.LastSnapshotSetID()
	c.Assert(err, check.IsNil)
	c.Check(setID, check.Equals, uint64(12))
}

func (s *snapshotSuite) TestLastSnapshotIDErrorOnDirNames(c *check.C) {
	// we need snapshots dir, otherwise LastSnapshotSetID exits early.
	c.Assert(os.MkdirAll(dirs.SnapshotsDir, os.ModePerm), check.IsNil)

	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		return nil, fmt.Errorf("fail")
	})()
	setID, err := backend.LastSnapshotSetID()
	c.Assert(err, check.ErrorMatches, "fail")
	c.Check(setID, check.Equals, uint64(0))
}

func (s *snapshotSuite) TestIsSnapshotFilename(c *check.C) {
	tests := []struct {
		name  string
		valid bool
		setID uint64
	}{
		{"1_foo.zip", true, 1},
		{"14_hello-world_6.4_29.zip", true, 14},
		{"1_.zip", false, 0},
		{"1_foo.zip.bak", false, 0},
		{"foo_1_foo.zip", false, 0},
		{"foo_bar_baz.zip", false, 0},
		{"", false, 0},
		{"1_", false, 0},
	}

	for _, t := range tests {
		ok, setID := backend.IsSnapshotFilename(t.name)
		c.Check(ok, check.Equals, t.valid, check.Commentf("fail: %s", t.name))
		c.Check(setID, check.Equals, t.setID, check.Commentf("fail: %s", t.name))
	}
}

func (s *snapshotSuite) TestIterBailsIfContextDone(c *check.C) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return nil, nil // deal with it
	})()

	err := backend.Iter(ctx, nil)
	c.Check(err, check.Equals, context.Canceled)
	c.Check(triedToOpenDir, check.Equals, false)
}

func (s *snapshotSuite) TestIterBailsIfContextDoneMidway(c *check.C) {
	ctx, cancel := context.WithCancel(context.Background())
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return os.Open(os.DevNull)
	})()
	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		cancel()
		return []string{"hello"}, nil
	})()
	triedToOpenSnapshot := false
	defer backend.MockOpen(func(string, uint64) (*backend.Reader, error) {
		triedToOpenSnapshot = true
		return nil, nil
	})()

	err := backend.Iter(ctx, nil)
	c.Check(err, check.Equals, context.Canceled)
	c.Check(triedToOpenDir, check.Equals, true)
	// bails as soon as
	c.Check(readNames, check.Equals, 1)
	c.Check(triedToOpenSnapshot, check.Equals, false)
}

func (s *snapshotSuite) TestIterReturnsOkIfSnapshotsDirNonexistent(c *check.C) {
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return nil, os.ErrNotExist
	})()

	err := backend.Iter(context.Background(), nil)
	c.Check(err, check.IsNil)
	c.Check(triedToOpenDir, check.Equals, true)
}

func (s *snapshotSuite) TestIterBailsIfSnapshotsDirFails(c *check.C) {
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return nil, os.ErrInvalid
	})()

	err := backend.Iter(context.Background(), nil)
	c.Check(err, check.ErrorMatches, "cannot open snapshots directory: invalid argument")
	c.Check(triedToOpenDir, check.Equals, true)
}

func (s *snapshotSuite) TestIterWarnsOnOpenErrorIfSnapshotNil(c *check.C) {
	logbuf, restore := logger.MockLogger()
	defer restore()
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return new(os.File), nil
	})()
	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		if readNames > 1 {
			return nil, io.EOF
		}
		return []string{"1_hello.zip"}, nil
	})()
	triedToOpenSnapshot := false
	defer backend.MockOpen(func(string, uint64) (*backend.Reader, error) {
		triedToOpenSnapshot = true
		return nil, os.ErrInvalid
	})()

	calledF := false
	f := func(snapshot *backend.Reader) error {
		calledF = true
		return nil
	}

	err := backend.Iter(context.Background(), f)
	// snapshot open errors are not failures:
	c.Check(err, check.IsNil)
	c.Check(triedToOpenDir, check.Equals, true)
	c.Check(readNames, check.Equals, 2)
	c.Check(triedToOpenSnapshot, check.Equals, true)
	c.Check(logbuf.String(), check.Matches, `(?m).* Cannot open snapshot "1_hello.zip": invalid argument.`)
	c.Check(calledF, check.Equals, false)
}

func (s *snapshotSuite) TestIterCallsFuncIfSnapshotNotNil(c *check.C) {
	logbuf, restore := logger.MockLogger()
	defer restore()
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return new(os.File), nil
	})()
	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		if readNames > 1 {
			return nil, io.EOF
		}
		return []string{"1_hello.zip"}, nil
	})()
	triedToOpenSnapshot := false
	defer backend.MockOpen(func(string, uint64) (*backend.Reader, error) {
		triedToOpenSnapshot = true
		// NOTE non-nil reader, and error, returned
		r := backend.Reader{}
		r.SetID = 1
		r.Broken = "xyzzy"
		return &r, os.ErrInvalid
	})()

	calledF := false
	f := func(snapshot *backend.Reader) error {
		c.Check(snapshot.Broken, check.Equals, "xyzzy")
		calledF = true
		return nil
	}

	err := backend.Iter(context.Background(), f)
	// snapshot open errors are not failures:
	c.Check(err, check.IsNil)
	c.Check(triedToOpenDir, check.Equals, true)
	c.Check(readNames, check.Equals, 2)
	c.Check(triedToOpenSnapshot, check.Equals, true)
	c.Check(logbuf.String(), check.Equals, "")
	c.Check(calledF, check.Equals, true)
}

func (s *snapshotSuite) TestIterReportsCloseError(c *check.C) {
	logbuf, restore := logger.MockLogger()
	defer restore()
	triedToOpenDir := false
	defer backend.MockOsOpen(func(string) (*os.File, error) {
		triedToOpenDir = true
		return new(os.File), nil
	})()
	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		if readNames > 1 {
			return nil, io.EOF
		}
		return []string{"42_hello.zip"}, nil
	})()
	triedToOpenSnapshot := false
	defer backend.MockOpen(func(string, uint64) (*backend.Reader, error) {
		triedToOpenSnapshot = true
		r := backend.Reader{}
		r.SetID = 42
		return &r, nil
	})()

	calledF := false
	f := func(snapshot *backend.Reader) error {
		c.Check(snapshot.SetID, check.Equals, uint64(42))
		calledF = true
		return nil
	}

	err := backend.Iter(context.Background(), f)
	// snapshot close errors _are_ failures (because they're completely unexpected):
	c.Check(err, check.Equals, os.ErrInvalid)
	c.Check(triedToOpenDir, check.Equals, true)
	c.Check(readNames, check.Equals, 1) // never gets to read another one
	c.Check(triedToOpenSnapshot, check.Equals, true)
	c.Check(logbuf.String(), check.Equals, "")
	c.Check(calledF, check.Equals, true)
}

func readerForFilename(fname string, c *check.C) *backend.Reader {
	var snapname string
	var id uint64
	fn := strings.TrimSuffix(filepath.Base(fname), ".zip")
	_, err := fmt.Sscanf(fn, "%d_%s", &id, &snapname)
	c.Assert(err, check.IsNil, check.Commentf(fn))
	f, err := os.Open(os.DevNull)
	c.Assert(err, check.IsNil, check.Commentf(fn))
	return &backend.Reader{
		File: f,
		Snapshot: client.Snapshot{
			SetID: id,
			Snap:  snapname,
		},
	}
}

func (s *snapshotSuite) TestIterIgnoresSnapshotsWithInvalidNames(c *check.C) {
	logbuf, restore := logger.MockLogger()
	defer restore()

	defer backend.MockOsOpen(func(string) (*os.File, error) {
		return new(os.File), nil
	})()
	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		if readNames > 1 {
			return nil, io.EOF
		}
		return []string{
			"_foo.zip",
			"43_bar.zip",
			"foo_bar.zip",
			"bar.",
		}, nil
	})()
	defer backend.MockOpen(func(fname string, setID uint64) (*backend.Reader, error) {
		return readerForFilename(fname, c), nil
	})()

	var calledF int
	f := func(snapshot *backend.Reader) error {
		calledF++
		c.Check(snapshot.SetID, check.Equals, uint64(43))
		return nil
	}

	err := backend.Iter(context.Background(), f)
	c.Check(err, check.IsNil)
	c.Check(logbuf.String(), check.Equals, "")
	c.Check(calledF, check.Equals, 1)
}

func (s *snapshotSuite) TestIterSetIDoverride(c *check.C) {
	if os.Geteuid() == 0 {
		c.Skip("this test cannot run as root (runuser will fail)")
	}
	logger.SimpleSetup()

	epoch := snap.E("42*")
	info := &snap.Info{SideInfo: snap.SideInfo{RealName: "hello-snap", Revision: snap.R(42), SnapID: "hello-id"}, Version: "v1.33", Epoch: epoch}
	cfg := map[string]interface{}{"some-setting": false}

	shw, err := backend.Save(context.TODO(), 12, info, cfg, []string{"snapuser"}, &backend.Flags{})
	c.Assert(err, check.IsNil)
	c.Check(shw.SetID, check.Equals, uint64(12))

	snapshotPath := filepath.Join(dirs.SnapshotsDir, "12_hello-snap_v1.33_42.zip")
	c.Check(backend.Filename(shw), check.Equals, snapshotPath)
	c.Check(hashkeys(shw), check.DeepEquals, []string{"archive.tgz", "user/snapuser.tgz"})

	// rename the snapshot, verify that set id from the filename is used by the reader.
	c.Assert(os.Rename(snapshotPath, filepath.Join(dirs.SnapshotsDir, "33_hello.zip")), check.IsNil)

	var calledF int
	f := func(snapshot *backend.Reader) error {
		calledF++
		c.Check(snapshot.SetID, check.Equals, uint64(uint(33)))
		c.Check(snapshot.Snap, check.Equals, "hello-snap")
		return nil
	}

	c.Assert(backend.Iter(context.Background(), f), check.IsNil)
	c.Check(calledF, check.Equals, 1)
}

func (s *snapshotSuite) TestList(c *check.C) {
	logbuf, restore := logger.MockLogger()
	defer restore()
	defer backend.MockOsOpen(func(string) (*os.File, error) { return new(os.File), nil })()

	readNames := 0
	defer backend.MockDirNames(func(*os.File, int) ([]string, error) {
		readNames++
		if readNames > 4 {
			return nil, io.EOF
		}
		return []string{
			fmt.Sprintf("%d_foo.zip", readNames),
			fmt.Sprintf("%d_bar.zip", readNames),
			fmt.Sprintf("%d_baz.zip", readNames),
		}, nil
	})()
	defer backend.MockOpen(func(fn string, setID uint64) (*backend.Reader, error) {
		var id uint64
		var snapname string
		c.Assert(strings.HasSuffix(fn, ".zip"), check.Equals, true)
		fn = strings.TrimSuffix(filepath.Base(fn), ".zip")
		_, err := fmt.Sscanf(fn, "%d_%s", &id, &snapname)
		c.Assert(err, check.IsNil, check.Commentf(fn))
		f, err := os.Open(os.DevNull)
		c.Assert(err, check.IsNil, check.Commentf(fn))
		return &backend.Reader{
			File: f,
			Snapshot: client.Snapshot{
				SetID:    id,
				Snap:     snapname,
				SnapID:   "id-for-" + snapname,
				Version:  "v1.0-" + snapname,
				Revision: snap.R(int(id)),
			},
		}, nil
	})()

	type tableT struct {
		setID     uint64
		snapnames []string
		numSets   int
		numShots  int
		predicate func(*client.Snapshot) bool
	}
	table := []tableT{
		{0, nil, 4, 12, nil},
		{0, []string{"foo"}, 4, 4, func(snapshot *client.Snapshot) bool { return snapshot.Snap == "foo" }},
		{1, nil, 1, 3, func(snapshot *client.Snapshot) bool { return snapshot.SetID == 1 }},
		{2, []string{"bar"}, 1, 1, func(snapshot *client.Snapshot) bool { return snapshot.Snap == "bar" && snapshot.SetID == 2 }},
		{0, []string{"foo", "bar"}, 4, 8, func(snapshot *client.Snapshot) bool { return snapshot.Snap == "foo" || snapshot.Snap == "bar" }},
	}

	for i, t := range table {
		comm := check.Commentf("%d: %d/%v", i, t.setID, t.snapnames)
		// reset
		readNames = 0
		logbuf.Reset()

		sets, err := backend.List(context.Background(), t.setID, t.snapnames)
		c.Check(err, check.IsNil, comm)
		c.Check(readNames, check.Equals, 5, comm)
		c.Check(logbuf.String(), check.Equals, "", comm)
		c.Check(sets, check.HasLen, t.numSets, comm)
		nShots := 0
		fnTpl := filepath.Join(dirs.SnapshotsDir, "%d_%s_%s_%s.zip")
		for j, ss := range sets {
			for k, snapshot := range ss.Snapshots {
				comm := check.Commentf("%d: %d/%v #%d/%d", i, t.setID, t.snapnames, j, k)
				if t.predicate != nil {
					c.Check(t.predicate(snapshot), check.Equals, true, comm)
				}
				nShots++
				fn := fmt.Sprintf(fnTpl, snapshot.SetID, snapshot.Snap, snapshot.Version, snapshot.Revision)
				c.Check(backend.Filename(snapshot), check.Equals, fn, comm)
				c.Check(snapshot.SnapID, check.Equals, "id-for-"+snapshot.Snap)
			}
		}
		c.Check(nShots, check.Equals, t.numShots)
	}
}

func (s *snapshotSuite) TestAddDirToZipBails(c *check.C) {
	snapshot := &client.Snapshot{SetID: 42, Snap: "a-snap"}
	buf, restore := logger.MockLogger()
	defer restore()
	// note as the zip is nil this would panic if it didn't bail
	c.Check(backend.AddDirToZip(nil, snapshot, nil, "", "an/entry", filepath.Join(s.root, "nonexistent")), check.IsNil)
	// no log for the non-existent case
	c.Check(buf.String(), check.Equals, "")
	buf.Reset()
	c.Check(backend.AddDirToZip(nil, snapshot, nil, "", "an/entry", "/etc/passwd"), check.IsNil)
	c.Check(buf.String(), check.Matches, "(?m).* is not a directory.")
}

func (s *snapshotSuite) TestAddDirToZipTarFails(c *check.C) {
	d := filepath.Join(s.root, "foo")
	c.Assert(os.MkdirAll(filepath.Join(d, "bar"), 0755), check.IsNil)
	c.Assert(os.MkdirAll(filepath.Join(s.root, "common"), 0755), check.IsNil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	c.Assert(backend.AddDirToZip(ctx, nil, z, "", "an/entry", d), check.ErrorMatches, ".* context canceled")
}

func (s *snapshotSuite) TestAddDirToZip(c *check.C) {
	d := filepath.Join(s.root, "foo")
	c.Assert(os.MkdirAll(filepath.Join(d, "bar"), 0755), check.IsNil)
	c.Assert(os.MkdirAll(filepath.Join(s.root, "common"), 0755), check.IsNil)
	c.Assert(ioutil.WriteFile(filepath.Join(d, "bar", "baz"), []byte("hello\n"), 0644), check.IsNil)

	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	snapshot := &client.Snapshot{
		SHA3_384: map[string]string{},
	}
	c.Assert(backend.AddDirToZip(context.Background(), snapshot, z, "", "an/entry", d), check.IsNil)
	z.Close() // write out the central directory

	c.Check(snapshot.SHA3_384, check.HasLen, 1)
	c.Check(snapshot.SHA3_384["an/entry"], check.HasLen, 96)
	c.Check(snapshot.Size > 0, check.Equals, true) // actual size most likely system-dependent
	br := bytes.NewReader(buf.Bytes())
	r, err := zip.NewReader(br, int64(br.Len()))
	c.Assert(err, check.IsNil)
	c.Check(r.File, check.HasLen, 1)
	c.Check(r.File[0].Name, check.Equals, "an/entry")
}

func (s *snapshotSuite) TestHappyRoundtrip(c *check.C) {
	s.testHappyRoundtrip(c, "marker", false)
}

func (s *snapshotSuite) TestHappyRoundtripAutomaticSnapshot(c *check.C) {
	s.testHappyRoundtrip(c, "marker", true)
}

func (s *snapshotSuite) TestHappyRoundtripNoCommon(c *check.C) {
	for _, t := range table(snap.MinimalPlaceInfo("hello-snap", snap.R(42)), filepath.Join(dirs.GlobalRootDir, "home/snapuser")) {
		if _, d := filepath.Split(t.dir); d == "common" {
			c.Assert(os.RemoveAll(t.dir), check.IsNil)
		}
	}
	s.testHappyRoundtrip(c, "marker", false)
}

func (s *snapshotSuite) TestHappyRoundtripNoRev(c *check.C) {
	for _, t := range table(snap.MinimalPlaceInfo("hello-snap", snap.R(42)), filepath.Join(dirs.GlobalRootDir, "home/snapuser")) {
		if _, d := filepath.Split(t.dir); d == "42" {
			c.Assert(os.RemoveAll(t.dir), check.IsNil)
		}
	}
	s.testHappyRoundtrip(c, "../common/marker", false)
}

func (s *snapshotSuite) testHappyRoundtrip(c *check.C, marker string, auto bool) {
	if os.Geteuid() == 0 {
		c.Skip("this test cannot run as root (runuser will fail)")
	}
	logger.SimpleSetup()

	epoch := snap.E("42*")
	info := &snap.Info{SideInfo: snap.SideInfo{RealName: "hello-snap", Revision: snap.R(42), SnapID: "hello-id"}, Version: "v1.33", Epoch: epoch}
	cfg := map[string]interface{}{"some-setting": false}
	shID := uint64(12)

	shw, err := backend.Save(context.TODO(), shID, info, cfg, []string{"snapuser"}, &backend.Flags{Auto: auto})
	c.Assert(err, check.IsNil)
	c.Check(shw.SetID, check.Equals, shID)
	c.Check(shw.Snap, check.Equals, info.InstanceName())
	c.Check(shw.SnapID, check.Equals, info.SnapID)
	c.Check(shw.Version, check.Equals, info.Version)
	c.Check(shw.Epoch, check.DeepEquals, epoch)
	c.Check(shw.Revision, check.Equals, info.Revision)
	c.Check(shw.Conf, check.DeepEquals, cfg)
	c.Check(shw.Auto, check.Equals, auto)
	c.Check(backend.Filename(shw), check.Equals, filepath.Join(dirs.SnapshotsDir, "12_hello-snap_v1.33_42.zip"))
	c.Check(hashkeys(shw), check.DeepEquals, []string{"archive.tgz", "user/snapuser.tgz"})

	shs, err := backend.List(context.TODO(), 0, nil)
	c.Assert(err, check.IsNil)
	c.Assert(shs, check.HasLen, 1)
	c.Assert(shs[0].Snapshots, check.HasLen, 1)

	shr, err := backend.Open(backend.Filename(shw), backend.ExtractFnameSetID)
	c.Assert(err, check.IsNil)
	defer shr.Close()

	for label, sh := range map[string]*client.Snapshot{"open": &shr.Snapshot, "list": shs[0].Snapshots[0]} {
		comm := check.Commentf("%q", label)
		c.Check(sh.SetID, check.Equals, shID, comm)
		c.Check(sh.Snap, check.Equals, info.InstanceName(), comm)
		c.Check(sh.SnapID, check.Equals, info.SnapID, comm)
		c.Check(sh.Version, check.Equals, info.Version, comm)
		c.Check(sh.Epoch, check.DeepEquals, epoch)
		c.Check(sh.Revision, check.Equals, info.Revision, comm)
		c.Check(sh.Conf, check.DeepEquals, cfg, comm)
		c.Check(sh.SHA3_384, check.DeepEquals, shw.SHA3_384, comm)
		c.Check(sh.Auto, check.Equals, auto)
	}
	c.Check(shr.Name(), check.Equals, filepath.Join(dirs.SnapshotsDir, "12_hello-snap_v1.33_42.zip"))
	c.Check(shr.Check(context.TODO(), nil), check.IsNil)

	newroot := c.MkDir()
	c.Assert(os.MkdirAll(filepath.Join(newroot, "home/snapuser"), 0755), check.IsNil)
	dirs.SetRootDir(newroot)

	var diff = func() *exec.Cmd {
		cmd := exec.Command("diff", "-urN", "-x*.zip", s.root, newroot)
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr
		return cmd
	}

	for i := 0; i < 3; i++ {
		comm := check.Commentf("%d", i)
		// sanity check
		c.Check(diff().Run(), check.NotNil, comm)

		// restore leaves things like they were (again and again)
		rs, err := shr.Restore(context.TODO(), snap.R(0), nil, logger.Debugf)
		c.Assert(err, check.IsNil, comm)
		rs.Cleanup()
		c.Check(diff().Run(), check.IsNil, comm)

		// dirty it -> no longer like it was
		c.Check(ioutil.WriteFile(filepath.Join(info.DataDir(), marker), []byte("scribble\n"), 0644), check.IsNil, comm)
	}
}

func (s *snapshotSuite) TestOpenSetIDoverride(c *check.C) {
	if os.Geteuid() == 0 {
		c.Skip("this test cannot run as root (runuser will fail)")
	}
	logger.SimpleSetup()

	epoch := snap.E("42*")
	info := &snap.Info{SideInfo: snap.SideInfo{RealName: "hello-snap", Revision: snap.R(42), SnapID: "hello-id"}, Version: "v1.33", Epoch: epoch}
	cfg := map[string]interface{}{"some-setting": false}

	shw, err := backend.Save(context.TODO(), 12, info, cfg, []string{"snapuser"}, &backend.Flags{})
	c.Assert(err, check.IsNil)
	c.Check(shw.SetID, check.Equals, uint64(12))

	c.Check(backend.Filename(shw), check.Equals, filepath.Join(dirs.SnapshotsDir, "12_hello-snap_v1.33_42.zip"))
	c.Check(hashkeys(shw), check.DeepEquals, []string{"archive.tgz", "user/snapuser.tgz"})

	shr, err := backend.Open(backend.Filename(shw), 99)
	c.Assert(err, check.IsNil)
	defer shr.Close()

	c.Check(shr.SetID, check.Equals, uint64(99))
}

func (s *snapshotSuite) TestRestoreRoundtripDifferentRevision(c *check.C) {
	if os.Geteuid() == 0 {
		c.Skip("this test cannot run as root (runuser will fail)")
	}
	logger.SimpleSetup()

	epoch := snap.E("42*")
	info := &snap.Info{SideInfo: snap.SideInfo{RealName: "hello-snap", Revision: snap.R(42), SnapID: "hello-id"}, Version: "v1.33", Epoch: epoch}
	shID := uint64(12)

	shw, err := backend.Save(context.TODO(), shID, info, nil, []string{"snapuser"}, nil)
	c.Assert(err, check.IsNil)
	c.Check(shw.Revision, check.Equals, info.Revision)

	shr, err := backend.Open(backend.Filename(shw), backend.ExtractFnameSetID)
	c.Assert(err, check.IsNil)
	defer shr.Close()

	c.Check(shr.Revision, check.Equals, info.Revision)
	c.Check(shr.Name(), check.Equals, filepath.Join(dirs.SnapshotsDir, "12_hello-snap_v1.33_42.zip"))

	// move the expected data to its expected place
	for _, dir := range []string{
		filepath.Join(s.root, "home", "snapuser", "snap", "hello-snap"),
		filepath.Join(dirs.SnapDataDir, "hello-snap"),
	} {
		c.Check(os.Rename(filepath.Join(dir, "42"), filepath.Join(dir, "17")), check.IsNil)
	}

	newroot := c.MkDir()
	c.Assert(os.MkdirAll(filepath.Join(newroot, "home", "snapuser"), 0755), check.IsNil)
	dirs.SetRootDir(newroot)

	var diff = func() *exec.Cmd {
		cmd := exec.Command("diff", "-urN", "-x*.zip", s.root, newroot)
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr
		return cmd
	}

	// sanity check
	c.Check(diff().Run(), check.NotNil)

	// restore leaves things like they were, but in the new dir
	rs, err := shr.Restore(context.TODO(), snap.R("17"), nil, logger.Debugf)
	c.Assert(err, check.IsNil)
	rs.Cleanup()
	c.Check(diff().Run(), check.IsNil)
}

func (s *snapshotSuite) TestPickUserWrapperRunuser(c *check.C) {
	n := 0
	defer backend.MockExecLookPath(func(s string) (string, error) {
		n++
		if s != "runuser" {
			c.Fatalf(`expected to get "runuser", got %q`, s)
		}
		return "/sbin/runuser", nil
	})()

	c.Check(backend.PickUserWrapper(), check.Equals, "/sbin/runuser")
	c.Check(n, check.Equals, 1)
}

func (s *snapshotSuite) TestPickUserWrapperSudo(c *check.C) {
	n := 0
	defer backend.MockExecLookPath(func(s string) (string, error) {
		n++
		if n == 1 {
			if s != "runuser" {
				c.Fatalf(`expected to get "runuser" first, got %q`, s)
			}
			return "", errors.New("no such thing")
		}
		if s != "sudo" {
			c.Fatalf(`expected to get "sudo" next, got %q`, s)
		}
		return "/usr/bin/sudo", nil
	})()

	c.Check(backend.PickUserWrapper(), check.Equals, "/usr/bin/sudo")
	c.Check(n, check.Equals, 2)
}

func (s *snapshotSuite) TestPickUserWrapperNothing(c *check.C) {
	n := 0
	defer backend.MockExecLookPath(func(s string) (string, error) {
		n++
		return "", errors.New("no such thing")
	})()

	c.Check(backend.PickUserWrapper(), check.Equals, "")
	c.Check(n, check.Equals, 2)
}

func (s *snapshotSuite) TestMaybeRunuserHappyRunuser(c *check.C) {
	uid := sys.UserID(0)
	defer backend.MockSysGeteuid(func() sys.UserID { return uid })()
	defer backend.SetUserWrapper("/sbin/runuser")()
	logbuf, restore := logger.MockLogger()
	defer restore()

	c.Check(backend.TarAsUser("test", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: "/sbin/runuser",
		Args: []string{"/sbin/runuser", "-u", "test", "--", "tar", "--bar"},
	})
	c.Check(backend.TarAsUser("root", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	uid = 42
	c.Check(backend.TarAsUser("test", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	c.Check(logbuf.String(), check.Equals, "")
}

func (s *snapshotSuite) TestMaybeRunuserHappySudo(c *check.C) {
	uid := sys.UserID(0)
	defer backend.MockSysGeteuid(func() sys.UserID { return uid })()
	defer backend.SetUserWrapper("/usr/bin/sudo")()
	logbuf, restore := logger.MockLogger()
	defer restore()

	cmd := backend.TarAsUser("test", "--bar")
	c.Check(cmd, check.DeepEquals, &exec.Cmd{
		Path: "/usr/bin/sudo",
		Args: []string{"/usr/bin/sudo", "-u", "test", "--", "tar", "--bar"},
	})
	c.Check(backend.TarAsUser("root", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	uid = 42
	c.Check(backend.TarAsUser("test", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	c.Check(logbuf.String(), check.Equals, "")
}

func (s *snapshotSuite) TestMaybeRunuserNoHappy(c *check.C) {
	uid := sys.UserID(0)
	defer backend.MockSysGeteuid(func() sys.UserID { return uid })()
	defer backend.SetUserWrapper("")()
	logbuf, restore := logger.MockLogger()
	defer restore()

	c.Check(backend.TarAsUser("test", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	c.Check(backend.TarAsUser("root", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	uid = 42
	c.Check(backend.TarAsUser("test", "--bar"), check.DeepEquals, &exec.Cmd{
		Path: s.tarPath,
		Args: []string{"tar", "--bar"},
	})
	c.Check(strings.TrimSpace(logbuf.String()), check.Matches, ".* No user wrapper found.*")
}

func (s *snapshotSuite) TestEstimateSnapshotSize(c *check.C) {
	restore := backend.MockUsersForUsernames(func(usernames []string) ([]*user.User, error) {
		return []*user.User{{HomeDir: filepath.Join(s.root, "home/user1")}}, nil
	})
	defer restore()

	var info = &snap.Info{
		SuggestedName: "foo",
		SideInfo: snap.SideInfo{
			Revision: snap.R(7),
		},
	}

	snapData := []string{
		"/var/snap/foo/7/somedatadir",
		"/var/snap/foo/7/otherdata",
		"/var/snap/foo/7",
		"/var/snap/foo/common",
		"/var/snap/foo/common/a",
		"/home/user1/snap/foo/7/somedata",
		"/home/user1/snap/foo/common",
	}
	var data []byte
	var expected int
	for _, d := range snapData {
		data = append(data, 0)
		expected += len(data)
		c.Assert(os.MkdirAll(filepath.Join(s.root, d), 0755), check.IsNil)
		c.Assert(ioutil.WriteFile(filepath.Join(s.root, d, "somfile"), data, 0644), check.IsNil)
	}

	sz, err := backend.EstimateSnapshotSize(info, nil)
	c.Assert(err, check.IsNil)
	c.Check(sz, check.Equals, uint64(expected))
}

func (s *snapshotSuite) TestEstimateSnapshotSizeEmpty(c *check.C) {
	restore := backend.MockUsersForUsernames(func(usernames []string) ([]*user.User, error) {
		return []*user.User{{HomeDir: filepath.Join(s.root, "home/user1")}}, nil
	})
	defer restore()

	var info = &snap.Info{
		SuggestedName: "foo",
		SideInfo: snap.SideInfo{
			Revision: snap.R(7),
		},
	}

	snapData := []string{
		"/var/snap/foo/common",
		"/var/snap/foo/7",
		"/home/user1/snap/foo/7",
		"/home/user1/snap/foo/common",
	}
	for _, d := range snapData {
		c.Assert(os.MkdirAll(filepath.Join(s.root, d), 0755), check.IsNil)
	}

	sz, err := backend.EstimateSnapshotSize(info, nil)
	c.Assert(err, check.IsNil)
	c.Check(sz, check.Equals, uint64(0))
}

func (s *snapshotSuite) TestEstimateSnapshotPassesUsernames(c *check.C) {
	var gotUsernames []string
	restore := backend.MockUsersForUsernames(func(usernames []string) ([]*user.User, error) {
		gotUsernames = usernames
		return nil, nil
	})
	defer restore()

	var info = &snap.Info{
		SuggestedName: "foo",
		SideInfo: snap.SideInfo{
			Revision: snap.R(7),
		},
	}

	_, err := backend.EstimateSnapshotSize(info, []string{"user1", "user2"})
	c.Assert(err, check.IsNil)
	c.Check(gotUsernames, check.DeepEquals, []string{"user1", "user2"})
}

func (s *snapshotSuite) TestEstimateSnapshotSizeNotDataDirs(c *check.C) {
	restore := backend.MockUsersForUsernames(func(usernames []string) ([]*user.User, error) {
		return []*user.User{{HomeDir: filepath.Join(s.root, "home/user1")}}, nil
	})
	defer restore()

	var info = &snap.Info{
		SuggestedName: "foo",
		SideInfo:      snap.SideInfo{Revision: snap.R(7)},
	}

	sz, err := backend.EstimateSnapshotSize(info, nil)
	c.Assert(err, check.IsNil)
	c.Check(sz, check.Equals, uint64(0))
}
func (s *snapshotSuite) TestExportTwice(c *check.C) {
	// use mocking done in snapshotSuite.SetUpTest
	info := &snap.Info{
		SideInfo: snap.SideInfo{
			RealName: "hello-snap",
			Revision: snap.R(42),
			SnapID:   "hello-id",
		},
		Version: "v1.33",
	}
	// create a snapshot
	shID := uint64(12)
	_, err := backend.Save(context.TODO(), shID, info, nil, []string{"snapuser"}, &backend.Flags{})
	c.Check(err, check.IsNil)

	// num_files + export.json + footer
	expectedSize := int64(4*512 + 1024 + 2*512)
	// do on export at the start of the epoch
	restore := backend.MockTimeNow(func() time.Time { return time.Time{} })
	defer restore()
	// export once
	buf := bytes.NewBuffer(nil)
	ctx := context.Background()
	se, err := backend.NewSnapshotExport(ctx, shID)
	c.Check(err, check.IsNil)
	err = se.Init()
	c.Assert(err, check.IsNil)
	c.Check(se.Size(), check.Equals, expectedSize)
	// and we can stream the data
	err = se.StreamTo(buf)
	c.Assert(err, check.IsNil)
	c.Check(buf.Len(), check.Equals, int(expectedSize))

	// and again to ensure size does not change when exported again
	//
	// Note that moving beyond year 2242 will change the tar format
	// used by the go internal tar and that will make the size actually
	// change.
	restore = backend.MockTimeNow(func() time.Time { return time.Date(2242, 1, 1, 12, 0, 0, 0, time.UTC) })
	defer restore()
	se2, err := backend.NewSnapshotExport(ctx, shID)
	c.Check(err, check.IsNil)
	err = se2.Init()
	c.Assert(err, check.IsNil)
	c.Check(se2.Size(), check.Equals, expectedSize)
	// and we can stream the data
	buf.Reset()
	err = se2.StreamTo(buf)
	c.Assert(err, check.IsNil)
	c.Check(buf.Len(), check.Equals, int(expectedSize))
}

func (s *snapshotSuite) TestExportUnhappy(c *check.C) {
	se, err := backend.NewSnapshotExport(context.Background(), 5)
	c.Assert(err, check.ErrorMatches, "no snapshot data found for 5")
	c.Assert(se, check.IsNil)
}
