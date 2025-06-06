package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

func ensureEnoughGeneratedText(ctx context.Context) error {
	for {
		numberOfGeneratedSentences, err := fetchNumberOfGeneratedSentences()
		if err != nil {
			return fmt.Errorf("error, when fetchNumberOfGeneratedSentences() for ensureEnoughGeneratedText(). Error: %v", err)
		}
		highestTypeTestCompletionCount, err := fetchHighestTypingTestCompletionCount()
		if err != nil {
			return fmt.Errorf("error, when fetchHighestTypingTestCompletionCount() for ensureEnoughGeneratedText(). Error: %v", err)
		}
		enough := isEnoughTextGenerated(
			sentencesPerTypingTest,
			numberOfGeneratedSentences,
			highestTypeTestCompletionCount,
		)
		if !enough {
			randomText, err := gatherRandomText(ctx)
			if err != nil {
				return fmt.Errorf("error, when gatherRandomText() for ensureEnoughGeneratedText(). Error: %v", err)
			}
			randomText = filterOutWeirdText(randomText)
			err = persistGeneratedSentences(randomText)
			if err != nil {
				return fmt.Errorf("error, when persistGeneratedSentences() for ensureEnoughGeneratedText(). Error: %v", err)
			}
		}
		time.Sleep(2 * time.Minute)
	}
}

func gatherRandomText(ctx context.Context) (string, error) {
	chatRequest := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: "Generate a completely random series of unrelated but coherent sentences.",
		},
	}
	req := openai.ChatCompletionRequest{
		Model:     openai.GPT4oMini,
		MaxTokens: 4096,
		Messages:  chatRequest,
		Stream:    true,
	}

	stream, err := chatClient.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error, when creating chat completion stream for submitChatMessage(). Error: %v", err)
	}
	defer stream.Close()

	var result strings.Builder
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return result.String(), nil
		}
		if err != nil {
			return "", fmt.Errorf("error, when streaming. Error: %v", err)
		}
		if len(response.Choices) == 0 {
			return "", fmt.Errorf("error, not enough choices returned in chat stream for gatherRandomText()")
		}
		result.WriteString(response.Choices[0].Delta.Content)
	}
}

func persistGeneratedSentences(text string) error {
	sentences := strings.Split(text, ".")
	sqlStatement, args := generateSqlForSentences(sentences)
	_, err := theClients.Database.Conn.Exec(sqlStatement, args...)
	if err != nil {
		return fmt.Errorf("error, when executing sql statement for persistGeneratedSentences(). Error: %v", err)
	}
	return nil
}

func generateSqlForSentences(sentences []string) (string, []any) {
	var inserts []string
	var args []any
	var builder strings.Builder
	for _, s := range sentences {
		s = strings.ReplaceAll(s, "\n", "")
		s = strings.TrimSpace(s)
		if len(s) < 5 { // junk sentence
			continue
		}
		builder.WriteString(s)
		args = append(args, builder.String())
		builder.Reset()
		inserts = append(inserts, "(?)")
	}
	return fmt.Sprintf("INSERT INTO sentence (text) VALUES %s", strings.Join(inserts, ",")), args
}

func isEnoughTextGenerated(
	sentencesPerTypingTest,
	numberOfGeneratedSentences,
	highestTypeTestCompletionCount int,
) bool {
	floor := 10
	ceiling := 1000
	possibleTypeTestCombinations := numberOfGeneratedSentences / sentencesPerTypingTest
	if possibleTypeTestCombinations < floor {
		return false
	}
	if possibleTypeTestCombinations > ceiling {
		return true
	}
	if possibleTypeTestCombinations > highestTypeTestCompletionCount {
		return true
	}
	return false
}

func fetchHighestTypingTestCompletionCount() (int, error) {
	var result int
	err := theClients.Database.Conn.QueryRow(
		`SELECT typing_test_completion_count
FROM person_who_types
ORDER BY typing_test_completion_count DESC
LIMIT 1`,
	).Scan(
		&result,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			return 0, fmt.Errorf("error, when attempting to execute sql statement: %v", err)
		}
	}
	return result, nil
}

func fetchNumberOfGeneratedSentences() (int, error) {
	var result int
	err := theClients.Database.Conn.QueryRow(
		`SELECT id
FROM sentence
ORDER BY id DESC
LIMIT 1`,
	).Scan(
		&result,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		} else {
			return 0, fmt.Errorf("error, when attempting to execute sql statement: %v", err)
		}
	}
	return result, nil
}

var makeTextNotWeirdMap = map[string]string{
	`”`: `"`,
	`“`: `"`,
	`’`: `'`,
	"À": "A",
	"Á": "A",
	"Â": "A",
	"Ã": "A",
	"Ä": "A",
	"Å": "A",
	"Ç": "C",
	"È": "E",
	"É": "E",
	"Ê": "E",
	"Ë": "E",
	"Ì": "I",
	"Í": "I",
	"Î": "I",
	"Ï": "I",
	"Ñ": "N",
	"Ò": "O",
	"Ó": "O",
	"Ô": "O",
	"Õ": "O",
	"Ö": "O",
	"Ø": "O",
	"Ù": "U",
	"Ú": "U",
	"Û": "U",
	"Ü": "U",
	"Ý": "Y",
	"à": "a",
	"á": "a",
	"â": "a",
	"ã": "a",
	"ä": "a",
	"å": "a",
	"ç": "c",
	"è": "e",
	"é": "e",
	"ê": "e",
	"ë": "e",
	"ì": "i",
	"í": "i",
	"î": "i",
	"ï": "i",
	"ñ": "n",
	"ò": "o",
	"ó": "o",
	"ô": "o",
	"õ": "o",
	"ö": "o",
	"ø": "o",
	"ù": "u",
	"ú": "u",
	"û": "u",
	"ü": "u",
	"ý": "y",
	"ÿ": "y",
}

func filterOutWeirdText(text string) string {
	parts := strings.Split(text, "")
	for i, s := range parts {
		if convert, ok := makeTextNotWeirdMap[s]; ok {
			parts[i] = convert
		}
	}
	return strings.Join(parts, "")
}
