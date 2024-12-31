package main

import "testing"

func Test_calculateWordsPerMin(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		expect := 116
		startTimeMilli := 1735257725433
		endTimeMilli := 1735257756436
		wordsTyped := int8(60)
		got := calculateWordsPerMin(int64(startTimeMilli), int64(endTimeMilli), wordsTyped)
		if got != expect {
			t.Errorf("error, expected %d but got %d", expect, got)
		}
	})
}
