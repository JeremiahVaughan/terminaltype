package main

import "testing"

func Test_generateSqlForSentences(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		input := []string{" Hello sir", "Me too "}
		expected := "INSERT INTO sentence (text) VALUES (?),(?)"
		got, gotArgs := generateSqlForSentences(input)
		if len(gotArgs) != len(input) {
			t.Errorf("error, expected %d args but got %d args", len(input), len(gotArgs))
		}
		if got != expected {
			t.Errorf("error, expected '%s' but got '%s' args", expected, got)
		}
	})
}

func Test_isEnoughTextGenerated(t *testing.T) {
	sentencesPerTypingTest := 5

	t.Run("below ceiling", func(t *testing.T) {
		expected := false
		numberOfGeneratedSentences := 1
		highestTypeTestCompletionCount := 0
		got := isEnoughTextGenerated(
			sentencesPerTypingTest,
			numberOfGeneratedSentences,
			highestTypeTestCompletionCount,
		)
		if got != expected {
			t.Errorf("error, expected %t but got %t", expected, got)
		}
	})

	t.Run("above ceiling", func(t *testing.T) {
		expected := true
		numberOfGeneratedSentences := 1001
		highestTypeTestCompletionCount := 0
		got := isEnoughTextGenerated(
			sentencesPerTypingTest,
			numberOfGeneratedSentences,
			highestTypeTestCompletionCount,
		)
		if got != expected {
			t.Errorf("error, expected %t but got %t", expected, got)
		}
	})

	t.Run("not enough text per completion count", func(t *testing.T) {
		expected := false
		numberOfGeneratedSentences := 20
		highestTypeTestCompletionCount := 3
		got := isEnoughTextGenerated(
			sentencesPerTypingTest,
			numberOfGeneratedSentences,
			highestTypeTestCompletionCount,
		)
		if got != expected {
			t.Errorf("error, expected %t but got %t", expected, got)
		}
	})

	t.Run("enough text per completion count", func(t *testing.T) {
		expected := true
		numberOfGeneratedSentences := 50
		highestTypeTestCompletionCount := 3
		got := isEnoughTextGenerated(
			sentencesPerTypingTest,
			numberOfGeneratedSentences,
			highestTypeTestCompletionCount,
		)
		if got != expected {
			t.Errorf("error, expected %t but got %t", expected, got)
		}
	})

}
