package main

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type activeView string

const (
	activeViewWelcome activeView = "w"
	activeViewRace    activeView = "r"
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
						time.Sleep(50 * time.Second)
						loadingFinished <- md
					}()
					return m, m.spinner.Tick
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
			}
		default:
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
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
