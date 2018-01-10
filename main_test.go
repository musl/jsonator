package main

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	pattern := regexp.MustCompile(`\d+\.\d+\.\d+`)
	if !pattern.MatchString(Version) {
		t.Errorf("Expected %#v to match \"%s\".", Version, pattern.String())
	}
}

func random_word() string {
	a := int('a')
	d := int('z') - a
	l := 3 + rand.Intn(11)

	str := make([]byte, l)

	for i := 0; i < l; i++ {
		b := byte(a + rand.Intn(d+1))
		if rand.Intn(10) > 5 {
			b ^= 1 << 5
		}
		str[i] = b
	}

	return string(str)
}

func random_paragraph() string {
	w := 10 + rand.Intn(91)
	p := make([]string, w)

	for i := 0; i < w; i++ {
		p[i] = random_word()
	}

	return strings.Join(p, " ")
}

func BenchmarkRead(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	count := 200
	store := NewStore()

	for store.Count() < count {
		k := random_word()
		d := make(map[string]string, count)

		for len(d) < count {
			w := random_word()
			p := random_paragraph()
			d[w] = p
		}

		store.Set(k, d)
	}

	keys := store.Keys()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = store.Get(keys[i%len(keys)])
	}
}

func BenchmarkInsert(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	store := NewStore()

	count := 1000

	words := make([]string, count)
	for i := 0; i < len(words); i++ {
		words[i] = random_word()
	}

	paragraphs := make([]string, count)
	for i := 0; i < len(paragraphs); i++ {
		paragraphs[i] = random_paragraph()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Set(words[i%len(words)], paragraphs[i%len(paragraphs)])
	}
}
