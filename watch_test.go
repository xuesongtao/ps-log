package pslog

import (
	"os"
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	w, err := NewWatch()
	if err != nil {
		t.Fatal(err)
	}

	targetLog := "./tmp/test2tail.log"
	newLog := "./tmp/test2tail1.log"
	if err := w.Add(tmpDir); err != nil {
		t.Fatal(err)
	}

	bus := make(chan *WatchFileInfo, 1)
	go w.Watch(bus)

	os.Rename(targetLog, newLog)
	if err != nil {
		t.Fatal(err)
	}

	defer os.Rename(newLog, targetLog)
	t.Log(w.WatchList())

	// for b := range bus {
	// 	t.Log(b.Path)
	// }

	time.Sleep(10 * time.Second)
	w.Close()
	time.Sleep(2 * time.Second)
}

func TestRenameWatch(t *testing.T) {
	targetLog := "./tmp/2024-09-01.log"
	f, err := os.Open(targetLog)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(f.Name())
	newLog := "./tmp/2024-09-01.1.log"
	os.Rename(targetLog, newLog)
	t.Log(f.Name())
}
