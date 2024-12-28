package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type activeView string

const (
	activeViewWelcome      activeView = "w"
	activeViewRace         activeView = "r"
	activeViewRaceFinished activeView = "rs"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// ignore key presses if loading
		if !m.loading {
			// reset any errors or validation messages on key press if not loading
			m.data.err = nil
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEnter:
				if m.activeView == activeViewWelcome {
					m.loading = true
					md := m.data
					go func() {
						// start race, for now but todo try to join one first if one is available
						md.raceWords, md.wordCount, md.err = fetchRaceWords()
						if md.err != nil {
							md.err = fmt.Errorf("error, when fetchRaceWords() for Update(). Error: %v", md.err)
						}
						loadingFinished <- md
					}()
					return m, m.spinner.Tick
				}
			case tea.KeyCtrlW:
				// todo punctuation needs to stagger ctrl W, like it does in vim
				// todo consider making commas, periods, and spaces at the end of the word not part of the word itself so they don't cause the adjecent word to also become incorrect
				if m.activeView == activeViewRace {
					i := m.incorrectPos
					j := 0
					for i > 0 && (m.data.raceWords[i-1] != ' ' || j == 0) {
						i--
						j++
					}
					if m.correctPos > i {
						m.correctPos = i
					}
					m.incorrectPos = i
				}
			case tea.KeyCtrlH, tea.KeyBackspace:
				if m.activeView == activeViewRace {
					if m.incorrectPos > m.correctPos {
						if m.incorrectPos > 0 {
							m.incorrectPos--
						}
					} else {
						if m.correctPos > 0 {
							m.correctPos--
						}
						if m.incorrectPos > 0 {
							m.incorrectPos--
						}
					}
				}
			default:
				if m.activeView == activeViewRace {
					if m.incorrectPos > m.correctPos {
						if m.incorrectPos < len(m.raceWordsCharSlice) {
							m.incorrectPos++
						}
					} else if m.correctPos < len(m.raceWordsCharSlice) && m.incorrectPos < len(m.raceWordsCharSlice) {
						keyMsg := msg.String()
						m = evaluateTypedKeyMatch(m, keyMsg)
					}
				}
			}
		}
	case spinner.TickMsg:
		select {
		case md := <-loadingFinished:
			m.resetSpinner()
			m.loading = false
			m.data = md
			switch m.activeView {
			case activeViewWelcome:
				m.activeView = activeViewRace
				m.raceWordsCharSlice = strings.Split(m.data.raceWords, "")
				m.raceStartTime = time.Now().UnixMilli()
			}
		default:
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
	default:
		// one use case so far for unexpected messages is holding down shift and pressing space bar.
		// I want this just to represent a space bar push so I am handling it as such.
		m = evaluateTypedKeyMatch(m, " ")
	}
	return m, cmd
	// #####
	// need to have
	// #####
	// handle welcome screen where the user can press enter to start a race
	// handle typed keys
	// need to display two sets of text, typed text and untyped text. Can add a cursor to if possible.
	// should use char count to decide how far the user has typed score wise and also to see if they are completed
	// if incorrect keys are typed, the user will be required to delete the whole word before they can proceed
	// for incorrect keys just show the correct key but turn it red and make as many red keys as needed with the exception of reaching the end of the text to display how much they need to delete

	// #####
	// nice to have
	// #####
	// should eject inactive users
	// Need to have a timer going so we can display words per min at the end of the race.
	// should display as many other players in the race as possible. Maybe even use a viewport for the text area (3 lines high) so there is more room for players and I don't have to worry about screen space should there be too much text on the screen at one time.
	// can use progress bars to represent other players. The current player should always been green so they don't lose track who they are but other players can be random colors other than green.
	// There should be a time limit on the race but long enough to let even very slow typers to finish. This prevents never ending sessions.
	// race should end for individuals once they have completed, which means they don't have to wait for other racers to finish before they can start another race
}

func fetchRaceWords() (string, int, error) {
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
	return text, wordCount, nil
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
	wordsTyped int) int {
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

func evaluateTypedKeyMatch(m model, keyMsg string) model {
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
		}
	} else {
		i := m.incorrectPos
		for i > 0 && m.data.raceWords[i-1] != ' ' {
			i--
		}
		m.correctPos = i
		m.incorrectPos++
	}
	return m
}
