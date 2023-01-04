package pslog

import (
	"bytes"
	"testing"
)

func TestTrie(t *testing.T) {
	row := `[2023-01-04T21:21:56+08:00] [ERRO] 110.184.137.102 200 "POST /hiddendanger/getprincipalconfiglist HTTP/1.1" 198 "http://localhost:8080/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36" "-"`
	// row := `[ERRO] 110.184`
	tree := newTire()
	tree.insert([]byte("1101"))
	tree.insert([]byte("ERRO"))
	t.Log(tree.search([]byte(row)))
}




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
	}
)

func BenchmarkMatchForTire(b *testing.B) {
	row := `[2023-01-04T21:21:56+08:00] [ERRO] 110.184.137.102 200 "POST /hiddendanger/getprincipalconfiglist HTTP/1.1" 198 "http://localhost:8080/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36" "-"`
	tree := newTire()
	for _, tt := range tts {
		tree.insert(tt)
	}
	by := []byte(row)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.search(by)
	}
}

func BenchmarkMatchForContains(b *testing.B) {
	row := `[2023-01-04T21:21:56+08:00] [ERRO] 110.184.137.102 200 "POST /hiddendanger/getprincipalconfiglist HTTP/1.1" 198 "http://localhost:8080/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36" "-"`
	by := []byte(row)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range tts {
			bytes.Contains(by, t)
		}
	}
}
