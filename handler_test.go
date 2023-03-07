package pslog

import (
	"sync"
	"testing"
)

func TestTmp(t *testing.T) {
	a := map[string]int{"1": 1}
	b := make(map[string]int)
	for k, v := range a {
		b[k] = v
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		b["2"] = 2
	}()
	go func() {
		defer wg.Done()
		b["3"] = 3
	}()
	wg.Wait()
	t.Log(a, b)
}
