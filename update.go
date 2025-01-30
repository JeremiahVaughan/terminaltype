package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/timer"
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
	case tea.MouseMsg:
		// Ignore all mouse messages, this is a typing game
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
		// ignore key presses if loading
		if !m.loading {
			// reset any errors or validation messages on key press if not loading
			m.data.err = nil
			switch msg.Type {
			case tea.KeyEnter:
				if m.activeView == activeViewWelcome || m.activeView == activeViewRaceFinished {
					m.loading = true
					md := m.data
					if m.natsConnection == nil {
						m.natsConnection, m.data.err = connectToNats()
						if m.data.err != nil {
							m.data.err = fmt.Errorf("error, when connectToNats() for Update(). Error: %v", m.data.err)
							HandleUnexpectedError(nil, m.data.err)
							return m, cmd
						}
					}
					// todo figure out the correct way to determine this channels buffer size
					m.allRacerProgressChan = make(chan *nats.Msg, 30)
					m.raceCtx, m.raceCancel = context.WithCancel(m.ctx)
					var sub *nats.Subscription
					sub, m.data.err = m.natsConnection.SubscribeSync(m.fingerprint)
					if m.data.err != nil {
						m.data.err = fmt.Errorf("error, when subscribing to registration queue for Update(). Error: %v", m.data.err)
						HandleUnexpectedError(nil, m.data.err)
						return m, cmd
					}
					sendMsg := nats.Msg{
						Subject: raceRegistrationRequestQueueId,
						Data:    []byte(m.fingerprint),
					}
					m.data.err = m.natsConnection.PublishMsg(&sendMsg)
					if m.data.err != nil {
						m.data.err = fmt.Errorf("error, when publishing registation request message for Update(). Error: %v", m.data.err)
						HandleUnexpectedError(nil, m.data.err)
						return m, cmd
					}
					// waits for twice as long as the race timeout and then assumes failure
					raceStartTimeout := time.Duration(raceStartTimeoutInSeconds) * 2 * time.Second
					var subMsg *nats.Msg
					subMsg, m.data.err = sub.NextMsg(raceStartTimeout)
					if m.data.err != nil {
						m.data.err = fmt.Errorf("error, when recieving race registration start time for Update(). Error: %v", m.data.err)
						HandleUnexpectedError(nil, m.data.err)
						return m, cmd
					}
					var theResponse RegResponse
					m.data.err = json.Unmarshal(subMsg.Data, &theResponse)
					if m.data.err != nil {
						m.data.err = fmt.Errorf("error, when decoding RegResponse for Update(). Error: %v", m.data.err)
						HandleUnexpectedError(nil, m.data.err)
						return m, cmd
					}
					timeRemainingTillStart := theResponse.RaceStartTime - time.Now().Unix()
					m.raceStartCountDown = timer.NewWithInterval(time.Duration(timeRemainingTillStart)*time.Second, time.Second)
					startTimeCmd := m.raceStartCountDown.Init()
					cmd = tea.Batch(cmd, startTimeCmd)
					m.data.raceId = theResponse.RaceId
					go func() {
						defer sub.Unsubscribe()
						subMsg, md.err = sub.NextMsg(raceStartTimeout)
						if md.err != nil {
							md.err = fmt.Errorf("error, when retrieving registration response message for Update(). Error: %v", md.err)
							HandleUnexpectedError(nil, md.err)
							m.loadingFinished <- md
							return
						}
						var reg RaceRegistration
						reg, md.err = decodeRaceRegistration(subMsg.Data)
						if md.err != nil {
							md.err = fmt.Errorf("error, when decoding registration response message for Update(). Error: %v", md.err)
							HandleUnexpectedError(nil, md.err)
							m.loadingFinished <- md
							return
						}

						md.raceId = reg.RaceId
						md.raceWords = reg.RaceWords
						md.wordCount = reg.WordCount
						md.allRacerProgress = reg.AllRaceProgress
						md.racerCount = reg.RacerCount
						go monitorRaceProgression(
							m.raceCtx,
							m.natsConnection,
							md.raceId,
							m.allRacerProgressChan,
							m.raceCancel,
						)

						m.loadingFinished <- md
					}()
					cmd = tea.Batch(cmd, m.spinner.Tick)
					return m, cmd
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
						var keyTypedCmd tea.Cmd
						m, keyTypedCmd = evaluateTypedKeyMatch(m, cmd, keyMsg)
						cmd = tea.Batch(cmd, keyTypedCmd)
						return m, cmd
					}
				}
			}
		}
	case stopwatch.StartStopMsg, stopwatch.ResetMsg:
		raceTicker, rtCmd := m.raceTicker.Update(msg)
		m.raceTicker = &raceTicker
		cmd = tea.Batch(cmd, rtCmd)
		return m, cmd
	case stopwatch.TickMsg:
		var p float32
		if m.correctPos == 0 {
			p = 0
		} else {
			p = float32(m.correctPos) / float32(len(m.raceWordsCharSlice))
		}
		rp := RaceProgress{
			Fingerprint:        m.fingerprint,
			RacerId:            m.racerId,
			PercentageComplete: p,
		}
		var raceCompletionPercentage []byte
		raceCompletionPercentage, m.data.err = encodeRaceProgress(rp)
		if m.data.err != nil {
			m.data.err = fmt.Errorf("error, when encodeRaceProgress() for Update(). Error: %v", m.data.err)
			HandleUnexpectedError(nil, m.data.err)
			return m, cmd
		}
		m.data.err = m.natsConnection.Publish(m.data.raceId, raceCompletionPercentage)
		if m.data.err != nil {
			m.data.err = fmt.Errorf("error, when attempting to publish the message for Update(). Error: %v", m.data.err)
			HandleUnexpectedError(nil, m.data.err)
			return m, cmd
		}
		m.data.allRacerProgress, m.data.err = processRacerProgressMsgs(m.allRacerProgressChan, m.data.allRacerProgress)
		if m.data.err != nil {
			m.data.err = fmt.Errorf("error, when processRacerProgressMsgs for Update(). Error: %v", m.data.err)
			HandleUnexpectedError(nil, m.data.err)
			return m, cmd
		}
		for i, p := range m.data.allRacerProgress {
			pc := m.racerProgressBars[i].SetPercent(float64(p.PercentageComplete))
			cmd = tea.Batch(cmd, pc)
		}
		raceTicker, rtCmd := m.raceTicker.Update(msg)
		m.raceTicker = &raceTicker
		cmd = tea.Batch(cmd, rtCmd)
		return m, cmd
	case timer.TimeoutMsg:
	case timer.StartStopMsg:
		m.raceStartCountDown, cmd = m.raceStartCountDown.Update(msg)
		return m, cmd
	case timer.TickMsg:
		m.raceStartCountDown, cmd = m.raceStartCountDown.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		for i, b := range m.racerProgressBars {
			progressModel, pCmd := b.Update(msg)
			b = progressModel.(progress.Model)
			m.racerProgressBars[i] = b
			cmd = tea.Batch(cmd, pCmd)
		}
		return m, cmd
	case spinner.TickMsg:
		select {
		case md := <-m.loadingFinished:
			m.resetSpinner()
			m.loading = false
			m.data = md
			if m.data.err != nil {
				return m, cmd
			}
			switch m.activeView {
			case activeViewWelcome, activeViewRaceFinished:
				m.activeView = activeViewRace
				m.raceWordsCharSlice = strings.Split(m.data.raceWords, "")
				m.raceStartTime = time.Now().UnixMilli()
				m.correctPos = 0
				m.incorrectPos = 0
				var swCmd tea.Cmd
				if m.raceTicker == nil {
					newWatch := stopwatch.New()
					m.raceTicker = &newWatch
					swCmd = m.raceTicker.Init()
				} else {
					swCmd = m.raceTicker.Start()
				}
				cmd = tea.Batch(cmd, swCmd)
				m.racerProgressBars = make([]progress.Model, maxPlayersPerRace)
				for i := int8(0); i < m.data.racerCount; i++ {
					m.racerProgressBars[i] = progress.New(progress.WithSolidFill(playerColors[i]))
					if m.fingerprint == m.data.allRacerProgress[i].Fingerprint {
						m.racerId = i
					}
				}
			}
			return m, cmd
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
		var keyTypedCmd tea.Cmd
		m, keyTypedCmd = evaluateTypedKeyMatch(m, cmd, " ")
		cmd = tea.Batch(cmd, keyTypedCmd)
		return m, cmd
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
