package main

import (
	"testing"
	"time"
)

func TestBoltKey(t *testing.T) {
	for i := 0; i < 200; i++ {
		now := time.Now().Round(time.Millisecond)
		for _, d := range []time.Duration{0, time.Millisecond, time.Microsecond, time.Nanosecond} {
			for add := time.Duration(0); add < 1000; add += 21 {
				newt := now.Add(d*add + add)
				expect, _ := time.Parse(timeKeyFormat, newt.Format(timeKeyFormat))
				b := boltKey(newt)
				got, err := time.Parse(timeKeyFormat, string(b))
				if err != nil {
					t.Fatal(err)
				}
				if got != expect {
					t.Errorf("got not rounded %s %s", got, expect)
				}
			}
		}
	}
}
