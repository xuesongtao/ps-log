package pslog

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"
)

var (
	tts = [][]byte{
		[]byte("1101"),
		[]byte("1101 a"),
		[]byte("a 1101"),
		[]byte("1101 a b"),
		[]byte("a b 1101"),
		[]byte("ERRO"),
		[]byte("ERRO a"),
		[]byte("a ERRO"),
		[]byte("ERRO a b"),
		[]byte("b a ERRO"),
		[]byte("110.184.137.102"),
		[]byte("测试"),
	}
)

func contains(by []byte) bool {
	for _, t := range tts {
		if ok := bytes.Contains(by, t); ok {
			return ok
		}
	}
	return false
}

func TestTireMem(t *testing.T) {
	tree := newTire()
	for i, tt := range tts {
		printMemStats(fmt.Sprintf("第%d次分配", i))
		tree.Insert(tt)
	}
	// var arr [255]*node
	// for _, v := range arr {
	// 	tv := reflect.ValueOf(&v)
	// 	t.Log(tv.Type().Size())
	// }
	printMemStats("")
}

func TestTrie(t *testing.T) {
	row := `{"level":"warning","msg":"q","time":"2024-02-02 17:57:10.399"}`
	contain := []byte("warning")
	// row := `[ERRO] 110.184`
	tree := newTire()
	if !tree.Null() {
		t.Error("null is no ok")
	}
	for _, tt := range tts {
		tree.Insert(tt)
	}
	if tree.Null() {
		t.Error("null is no ok")
	}

	bb := []byte(row)
	ok := contains(contain)
	if tree.Search(bb) != ok {
		t.Errorf("handle is failed, it should is [%v]", ok)
	}
}

func printMemStats(mag string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("%v：memory = %vKB, total = %vKB GC Times = %v\n", mag, m.Alloc/1024, m.TotalAlloc/1024, m.NumGC)
}

func BenchmarkMatchForTire(b *testing.B) {
	row := `[2023-01-04T21:21:56+08:00] [ERRO] 110.184.137.102 200 "POST /hiddendanger/getprincipalconfiglist HTTP/1.1" 198 "http://localhost:8080/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36" "-"`
	tree := newTire()
	for _, tt := range tts {
		tree.Insert(tt)
	}
	by := []byte(row)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(by)
	}
	// b.ResetTimer()
	printMemStats("test")
}

func BenchmarkMatchForContains(b *testing.B) {
	row := `[2023-01-04T21:21:56+08:00] [ERRO] 110.184.137.102 200 "POST /hiddendanger/getprincipalconfiglist HTTP/1.1" 198 "http://localhost:8080/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36" "-"`
	by := []byte(row)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contains(by)
	}
	printMemStats("test")
}
