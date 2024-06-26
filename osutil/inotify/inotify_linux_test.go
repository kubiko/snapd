//go:build linux

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inotify_test

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snapcore/snapd/osutil/inotify"
)

func TestInotifyEvents(t *testing.T) {
	// Create an inotify watcher instance and initialize it
	watcher, err := inotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher failed: %s", err)
	}

	dir, err := os.MkdirTemp("", "inotify")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	defer os.RemoveAll(dir)

	// Add a watch for "_test"
	err = watcher.Watch(dir)
	if err != nil {
		t.Fatalf("Watch failed: %s", err)
	}

	// Receive errors on the error channel on a separate goroutine
	go func() {
		for err := range watcher.Error {
			t.Errorf("error received: %s", err)
		}
	}()

	testFile := dir + "/TestInotifyEvents.testfile"

	// Receive events on the event channel on a separate goroutine
	eventstream := watcher.Event
	var eventsReceived int32
	done := make(chan bool)
	go func() {
		for event := range eventstream {
			// Only count relevant events
			if event.Name == testFile {
				atomic.AddInt32(&eventsReceived, 1)
				t.Logf("event received: %s", event)
			} else {
				t.Logf("unexpected event received: %s", event)
			}
		}
		done <- true
	}()

	// Create a file
	// This should add at least one event to the inotify event queue
	_, err = os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Fatalf("creating test file: %s", err)
	}

	// We expect this event to be received almost immediately, but let's wait 1 s to be sure
	time.Sleep(1 * time.Second)
	if atomic.AddInt32(&eventsReceived, 0) == 0 {
		t.Fatal("inotify event hasn't been received after 1 second")
	}

	// Try closing the inotify instance
	t.Log("calling Close()")
	watcher.Close()
	t.Log("waiting for the event channel to become closed...")
	select {
	case <-done:
		t.Log("event channel closed")
	case <-time.After(1 * time.Second):
		t.Fatal("event stream was not closed after 1 second")
	}
}

func TestInotifyClose(t *testing.T) {
	watcher, _ := inotify.NewWatcher()
	watcher.Close()

	done := make(chan bool)
	go func() {
		watcher.Close()
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("double Close() test failed: second Close() call didn't return")
	}

	err := watcher.Watch(os.TempDir())
	if err == nil {
		t.Fatal("expected error on Watch() after Close(), got nil")
	}
}

func TestLockOnEvent(t *testing.T) {
	watcher, err := inotify.NewWatcher()

	if err != nil {
		t.Fatalf("NewWatcher failed: %s", err)
	}

	dir, err := os.MkdirTemp("", "inotify")
	if err != nil {
		t.Fatalf("TempDir failed: %s", err)
	}
	defer os.RemoveAll(dir)

	// Add a watch for "_test"
	err = watcher.Watch(dir)
	if err != nil {
		t.Fatalf("Watch failed: %s", err)
	}
	os.Mkdir(dir+"/TestInotifyEvents.testfile", 0)
	// wait one second to ensure that the event is created
	time.Sleep(1 * time.Second)
	// now close the watcher. It must close everything and unlock from the event
	watcher.Close()
	// wait another second to ensure that the thread is executed and everything is processed
	time.Sleep(1 * time.Second)
	// this must fail because the queue is closed
	data, ok := <-watcher.Event
	if (data != nil) || (ok != false) {
		t.Fatalf("Watcher event queue isn't closed")
	}
}
