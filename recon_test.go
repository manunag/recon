package main

import (
	"reflect"
	"strings"
	"testing"
)

func load(t *testing.T, s string) map[string]Record {
	t.Helper()
	m, err := Load(strings.NewReader(s), "id")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCompareCategories(t *testing.T) {
	src := load(t, `{"id":1,"email":"a@x.com","tier":"gold"}
{"id":2,"email":"b@x.com","tier":"gold"}
{"id":3,"email":"c@x.com","tier":"silver"}
{"id":4,"email":"d@x.com","tier":"gold"}`)
	dst := load(t, `{"id":1,"email":"a@x.com","tier":"gold"}
{"id":2,"email":"B@x.com","tier":"gold"}
{"id":3,"email":"c@x.com","tier":"gold"}
{"id":9,"email":"z@x.com","tier":"gold"}`)

	rep := Compare(src, dst, Options{Key: "id", Samples: 5})

	want := map[Category]int{
		"field:email":   1, // id 2
		"field:tier":    1, // id 3
		MissingInTarget: 1, // id 4
		ExtraInTarget:   1, // id 9
	}
	got := map[Category]int{}
	for c, b := range rep.Categories {
		got[c] = b.Count
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories = %v, want %v", got, want)
	}
	if rep.Matched != 1 {
		t.Fatalf("matched = %d, want 1", rep.Matched)
	}
}

func TestIgnoreAndSampleLimit(t *testing.T) {
	src := load(t, `{"id":1,"ts":1,"v":"a"}
{"id":2,"ts":1,"v":"a"}
{"id":3,"ts":1,"v":"a"}`)
	dst := load(t, `{"id":1,"ts":2,"v":"b"}
{"id":2,"ts":2,"v":"b"}
{"id":3,"ts":2,"v":"b"}`)

	rep := Compare(src, dst, Options{Key: "id", Ignore: map[string]bool{"ts": true}, Samples: 2})

	if _, ok := rep.Categories["field:ts"]; ok {
		t.Fatal("ignored field reported")
	}
	b := rep.Categories["field:v"]
	if b == nil || b.Count != 3 || len(b.Samples) != 2 {
		t.Fatalf("bucket = %+v, want count 3 with 2 samples", b)
	}
}

func TestLoadErrors(t *testing.T) {
	for name, in := range map[string]string{
		"bad json":      `{"id":1`,
		"missing key":   `{"name":"x"}`,
		"duplicate key": "{\"id\":1}\n{\"id\":1}",
	} {
		if _, err := Load(strings.NewReader(in), "id"); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMissingFieldIsDifferent(t *testing.T) {
	src := load(t, `{"id":1,"email":"a@x.com"}`)
	dst := load(t, `{"id":1}`)
	rep := Compare(src, dst, Options{Key: "id", Samples: 1})
	if rep.Categories["field:email"] == nil {
		t.Fatal("a field absent in the target should count as a difference")
	}
}
