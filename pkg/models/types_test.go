package models

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	cases := []struct {
		d  time.Duration
		ex string
	}{
		{5 * time.Second, "5s"},
		{2 * time.Minute, "2m"},
		{1 * time.Hour, "1h"},
		{3 * 24 * time.Hour, "3d"},
		{14 * 24 * time.Hour, "2w"},
	}
	for _, c := range cases {
		if got := FormatAge(c.d); got != c.ex {
			t.Fatalf("expected %s got %s", c.ex, got)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	created := time.Now().Add(-time.Hour)
	age := HumanizeAge(created)
	if age == "" || age[len(age)-1] != 'h' {
		t.Fatalf("unexpected age: %s", age)
	}
}

func TestMatchesFilter(t *testing.T) {
	svc := Service{Namespace: "PUBLIC", Name: "hello", Status: "running", ComputePool: "x"}
	if !svc.MatchesFilter("run") {
		t.Fatalf("should match status")
	}
	if svc.MatchesFilter("nomatch") {
		t.Fatalf("should not match")
	}
}
