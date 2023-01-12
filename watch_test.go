package pslog

import (
	"testing"
	"time"
)

func TestWatch(t *testing.T) {
	w, err := NewWatch()
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Add("./tmp/test2tail.log"); err != nil {
		t.Fatal(err)
	}

	bus := make(chan *WatchFileInfo, 1)
	go w.Watch(bus)

	t.Log(w.WatchList())
	
	time.Sleep(5 * time.Second)
	w.Close()
	time.Sleep(2*time.Second)
}
