package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/nats-io/nats.go"
)

func fetchRaceWords() (string, int8, error) {
	totalSentences, err := fetchNumberOfGeneratedSentences()
	if err != nil {
		return "", 0, fmt.Errorf("error, when fetchNumberOfGeneratedSentences() for fetchRaceWords(). Error: %v", err)
	}
	if totalSentences <= sentencesPerTypingTest {
		return "", 0, fmt.Errorf("error, more sentences need to generate, please wait.")
	}
	randomSentences := make([]any, sentencesPerTypingTest)
	i := 0
	for {
		randomSentence := rand.Intn(totalSentences) + 1
		duplicateFound := false
		for _, r := range randomSentences {
			if r == randomSentence {
				duplicateFound = true
				break
			}
		}
		if duplicateFound {
			continue
		}
		randomSentences[i] = randomSentence
		i++
		if i >= sentencesPerTypingTest {
			break
		}
	}

	placeholders := make([]string, sentencesPerTypingTest)
	for i := 0; i < sentencesPerTypingTest; i++ {
		placeholders[i] = "?"
	}

	theQuery := fmt.Sprintf(
		`SELECT text
	FROM sentence
	WHERE id IN (%s)`,
		strings.Join(placeholders, ","),
	)
	rows, err := database.Query(
		theQuery,
		randomSentences...,
	)

	defer func(rows *sql.Rows) {
		if rows != nil {
			closeRowsError := rows.Close()
			if closeRowsError != nil {
				// no choice but to log the error since defer doesn't let us return errors
				// defer is needed though because it ensures a cleanup attempt is made even if we should return early due to an error
				log.Printf("error, when attempting to close database rows: %v", closeRowsError)
			}
		}
	}(rows)
	if err != nil {
		return "", 0, fmt.Errorf("error, when attempting to retrieve records. Error: %v", err)
	}

	queryResults := make([]string, sentencesPerTypingTest)
	i = 0
	for rows.Next() {
		var theQueryResult string
		err = rows.Scan(
			&theQueryResult,
		)
		if err != nil {
			return "", 0, fmt.Errorf("error, when scanning database rows. Error: %v", err)
		}
		queryResults[i] = theQueryResult
		i++
	}
	err = rows.Err()
	if err != nil {
		return "", 0, fmt.Errorf("error, when iterating through database rows. Error: %v", err)
	}
	builder := strings.Builder{}
	builder.WriteString(strings.Join(queryResults, ". "))
	builder.WriteRune('.')
	text := builder.String()
	wordCount := len(strings.Split(text, " ")) // todo consider saving the word count in the DB to speed up game start times
	return text, int8(wordCount), nil
}

func formatWordBlock(
	raceWordsCharSlice []string,
	correctPos int,
	incorrectPos int,
) string {
	unitSeperator := "\u200B" // this zero width space char doesn't appear to conflict or get counted in word wrap length functions
	raceWordsCharSlice = insert(raceWordsCharSlice, correctPos, unitSeperator)
	if correctPos != incorrectPos {
		raceWordsCharSlice = insert(raceWordsCharSlice, incorrectPos+1, unitSeperator)
	}
	str := strings.Join(raceWordsCharSlice, "")
	str = textBaseStyle.Render(str)
	str = applyTextColors(
		str,
		unitSeperator,
	)
	return textBaseStyle.Render(str)
}

func applyTextColors(text string, unitSeparator string) string {
	parts := strings.Split(text, unitSeparator)
	b := strings.Builder{}

	for i, p := range parts {
		switch i {
		case 0:
			b.WriteString(renderAndTrim(p, false, correctStyle.Render))
		case 1:
			if len(parts) == 3 {
				p = strings.ReplaceAll(p, " ", "_")
				b.WriteString(renderAndTrim(p, false, incorrectStyle.Render))
			} else {
				b.WriteString(renderAndTrim(p, true, regularStyle.Render))
			}

		case 2:
			b.WriteString(renderAndTrim(p, true, regularStyle.Render))
		}
	}

	return b.String()
}

func renderAndTrim(text string, cursor bool, style func(...string) string) string {
	cutset := " \t\n\r"
	var b strings.Builder
	if cursor {
		cursorPart := text[:1]
		rest := text[1:]
		b.WriteString(cursorStyle.Render(cursorPart))
		b.WriteString(style(rest))
	} else {
		b.WriteString(style(text))
	}
	return strings.TrimRight(b.String(), cutset)
}

func insert(slice []string, index int, value string) []string {
	if index < 0 || index > len(slice) {
		panic("index out of range")
	}
	newSlice := make([]string, len(slice)+1)
	copy(newSlice, slice[:index])
	newSlice[index] = value
	copy(newSlice[index+1:], slice[index:])
	return newSlice
}

func calculateWordsPerMin(startTimeMillis int64, endTimeMillis int64,
	wordsTyped int8) int {
	// Calculate the time difference in milliseconds
	timeDifferenceMillis := endTimeMillis - startTimeMillis

	// Convert milliseconds to minutes
	timeDifferenceMinutes := float64(timeDifferenceMillis) / 60000.0

	// Calculate words per minute
	if timeDifferenceMinutes <= 0 {
		return 0 // Prevent division by zero or negative time
	}

	wordsPerMin := float64(wordsTyped) / timeDifferenceMinutes
	return int(wordsPerMin + 0.5) // Round to the nearest whole number
}

func evaluateTypedKeyMatch(m model, cmd tea.Cmd, keyMsg string) (model, tea.Cmd) {
	if keyMsg == m.raceWordsCharSlice[m.correctPos] {
		m.correctPos++
		m.incorrectPos = m.correctPos // stay in sync
		if m.correctPos >= len(m.raceWordsCharSlice) {
			m.wordsPerMin = calculateWordsPerMin(
				m.raceStartTime,
				time.Now().UnixMilli(),
				m.data.wordCount,
			)
			m.activeView = activeViewRaceFinished
			cmd1 := m.raceTicker.Stop()
			cmd2 := m.raceTicker.Reset()
			cmd = tea.Batch(cmd, cmd1, cmd2)
			go func() {
				err := incrementRaceCompletionCount(m.fingerprint)
				if err != nil {
					HandleUnexpectedError(nil, fmt.Errorf("error, when incrementRaceCompletionCount() for evaluateTypedKeyMatch(). Error: %v", err))
					// can continue if this error happens because its not the end of the world, but still needs to be reported
				}
			}()
		}
	} else {
		i := m.incorrectPos
		for i > 0 && m.data.raceWords[i-1] != ' ' {
			i--
		}
		m.correctPos = i
		m.incorrectPos++
	}
	return m, cmd
}

func incrementRaceCompletionCount(userFingerprint string) error {
	count, err := fetchCurrentRaceCompletionCount(userFingerprint)
	if err != nil {
		return fmt.Errorf("error, when fetchCurrentRaceCompletionCount() for incrementRaceCompletionCount(). Error: %v", err)
	}
	count++
	if count == 1 {
		_, err = database.Exec(
			`INSERT INTO person_who_types (ssh_finger_print, typing_test_completion_count)
VALUES (?, ?)`,
			userFingerprint,
			count,
		)
		if err != nil {
			return fmt.Errorf("error, during insert for incrementRaceCompletionCount(). Error: %v", err)
		}
	} else {
		_, err = database.Exec(
			`UPDATE person_who_types 
SET typing_test_completion_count = ?
WHERE ssh_finger_print = ?`,
			count,
			userFingerprint,
		)
		if err != nil {
			return fmt.Errorf("error, during update for incrementRaceCompletionCount(). Error: %v", err)
		}
	}
	return nil
}

func fetchCurrentRaceCompletionCount(userFingerprint string) (int, error) {
	var result int
	err := database.QueryRow(
		`SELECT typing_test_completion_count
FROM person_who_types
WHERE ssh_finger_print = ?`,
		userFingerprint,
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

type RaceRegistration struct {
	RaceWords       string         `json:"raceWords"`
	WordCount       int8           `json:"wordCount"`
	RaceId          string         `json:"raceId"`
	AllRaceProgress []RaceProgress `json:"allRaceProgress"`
	RacerCount      int8           `json:"racerCount"`
}

type RaceProgress struct {
	RacerId            int8    `json:"racerId"`
	Fingerprint        string  `json:"fingerprint"`
	PercentageComplete float32 `json:"percentageComplete"`
}

// todo the below encodings are being made in JSON only for convience I want to get away from
// encoding everything in JSON in favor of more effecient approaches. But for now this will have to do.
func encodeRaceProgress(rp RaceProgress) ([]byte, error) {
	// buf := new(bytes.Buffer)
	// err := binary.Write(buf, binary.LittleEndian, rp)
	// if err != nil {
	// 	return nil, fmt.Errorf("error, when writing bytes for encodeRaceProgress(). Error: %v", err)
	// }
	// return buf.Bytes(), nil
	bytes, err := json.Marshal(rp)
	if err != nil {
		return nil, fmt.Errorf("error, when encoding json for encodeRaceProgress(). Error: %v", err)
	}
	return bytes, nil
}

func encodeAllRaceProgress(rp []RaceProgress) ([]byte, error) {
	// buf := new(bytes.Buffer)
	// err := binary.Write(buf, binary.LittleEndian, rp)
	// if err != nil {
	// }
	// return buf.Bytes(), nil

	bytes, err := json.Marshal(rp)
	if err != nil {
		return nil, fmt.Errorf("error, when writing bytes for encodeAllRaceProgress(). Error: %v", err)
	}
	return bytes, nil
}

func encodeRaceRegistration(r RaceRegistration) ([]byte, error) {
	// buf := new(bytes.Buffer)
	// err := binary.Write(buf, binary.LittleEndian, r)
	// if err != nil {
	// 	return nil, fmt.Errorf("error, when writing bytes for encodeRaceRegistration(). Error: %v", err)
	// }
	bytes, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("error, when writing bytes for encodeRaceRegistration(). Error: %v", err)
	}
	return bytes, nil
	// return buf.Bytes(), nil
}

func decodeRaceProgress(data []byte) (RaceProgress, error) {
	var rp RaceProgress
	// reader := bytes.NewReader(data)
	// if err := binary.Read(reader, binary.LittleEndian, &rp); err != nil {
	// 	return RaceProgress{}, fmt.Errorf("error, when reading bytes for decodeRaceProgress(). Error: %v", err)
	// }
	var err error
	err = json.Unmarshal(data, &rp)
	if err != nil {
		return RaceProgress{}, fmt.Errorf("error, when reading bytes for decodeRaceProgress(). Error: %v", err)
	}
	return rp, nil
}

func decodeAllRaceProgress(data []byte) ([]RaceProgress, error) {
	var rp []RaceProgress
	// reader := bytes.NewReader(data)
	// if err := binary.Read(reader, binary.LittleEndian, &rp); err != nil {
	// 	return nil, fmt.Errorf("error, when reading bytes for decodeAllRaceProgress(). Error: %v", err)
	// }
	var err error
	err = json.Unmarshal(data, &rp)
	if err != nil {
		return nil, fmt.Errorf("error, when reading bytes for decodeAllRaceProgress(). Error: %v", err)
	}
	return rp, nil
}

func decodeRaceRegistration(data []byte) (RaceRegistration, error) {
	var r RaceRegistration
	// reader := bytes.NewReader(data)
	// if err := binary.Read(reader, binary.LittleEndian, &r); err != nil {
	// 	return RaceRegistration{}, fmt.Errorf("error, when reading bytes for decodeRaceRegistration(). Error: %v", err)
	// }
	var err error
	err = json.Unmarshal(data, &r)
	if err != nil {
		return RaceRegistration{}, fmt.Errorf("error, when reading bytes for decodeRaceRegistration(). Error: %v", err)
	}
	return r, nil
}

func monitorRaceProgression(
	raceCtx context.Context,
	raceNatsConnection *nats.Conn,
	raceId string,
	allRacerProgressChan chan *nats.Msg,
) {
	sub, err := raceNatsConnection.ChanSubscribe(raceId, allRacerProgressChan)
	if err != nil {
		err = fmt.Errorf("error, when nats.Conn.ChanSubscribe() for monitorRaceProgression(). Error: %v", err)
		HandleUnexpectedError(nil, err)
		return
	}
	<-raceCtx.Done()
	err = sub.Unsubscribe()
	if err != nil {
		err = fmt.Errorf("error, when unsubscribing for monitorRaceProgression(). Error: %v", err)
		HandleUnexpectedError(nil, err)
		return
	}
	return
}

func processRacerProgressMsgs(messages chan *nats.Msg, progress []RaceProgress) ([]RaceProgress, error) {
	for {
		select {
		case natsMsg, ok := <-messages:
			if !ok {
				// Channel is closed, exit the loop
				return progress, nil
			}
			p, err := decodeRaceProgress(natsMsg.Data)
			if err != nil {
				return nil, fmt.Errorf("error, when decodeRaceProgress() for processRacerProgressMsgs(). Error: %v", err)
			}
			progress[p.RacerId] = p
		default:
			return progress, nil
		}
	}
}
