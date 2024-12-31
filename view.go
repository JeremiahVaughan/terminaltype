package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if m.data.err != nil {
		return getErrorStyle(m.data.err.Error())
	}

	var content string
	switch m.activeView {
	case activeViewWelcome:
		if m.loading {
			content = getRaceLoadingView(m)
		} else {
			content = ` .----------------.  .----------------.  .----------------.  .----------------.   
| .--------------. || .--------------. || .--------------. || .--------------. |  
| |  _________   | || |  _________   | || |  _______     | || | ____    ____ | |  
| | |  _   _  |  | || | |_   ___  |  | || | |_   __ \    | || ||_   \  /   _|| |  
| | |_/ | | \_|  | || |   | |_  \_|  | || |   | |__) |   | || |  |   \/   |  | |  
| |     | |      | || |   |  _|  _   | || |   |  __ /    | || |  | |\  /| |  | |  
| |    _| |_     | || |  _| |___/ |  | || |  _| |  \ \_  | || | _| |_\/_| |_ | |  
| |   |_____|    | || | |_________|  | || | |____| |___| | || ||_____||_____|| |  
| |              | || |              | || |              | || |              | |  
| '--------------' || '--------------' || '--------------' || '--------------' |  
 '----------------'  '----------------'  '----------------'  '----------------'   



(PRESS ENTER TO START)



 .----------------.  .----------------.  .----------------.  .----------------.   
| .--------------. || .--------------. || .--------------. || .--------------. |  
| |  _________   | || |  ____  ____  | || |   ______     | || |  _________   | |  
| | |  _   _  |  | || | |_  _||_  _| | || |  |_   __ \   | || | |_   ___  |  | |  
| | |_/ | | \_|  | || |   \ \  / /   | || |    | |__) |  | || |   | |_  \_|  | |  
| |     | |      | || |    \ \/ /    | || |    |  ___/   | || |   |  _|  _   | |  
| |    _| |_     | || |    _|  |_    | || |   _| |_      | || |  _| |___/ |  | |  
| |   |_____|    | || |   |______|   | || |  |_____|     | || | |_________|  | |  
| |              | || |              | || |              | || |              | |  
| '--------------' || '--------------' || '--------------' || '--------------' |  
 '----------------'  '----------------'  '----------------'  '----------------' `
		}
	case activeViewRace:
		wordBlock := formatWordBlock(
			m.raceWordsCharSlice,
			m.correctPos,
			m.incorrectPos,
		)
		racerViews := strings.Builder{}
		for i := int8(0); i < m.data.racerCount; i++ {
			racerViews.WriteString("\n\n")
			var playerTitle string
			if m.data.allRacerProgress[i].Fingerprint == m.fingerprint {
				playerTitle = fmt.Sprintf("player %d (you)", i)
			} else {
				playerTitle = fmt.Sprintf("player %d", i)
			}
			racerViews.WriteString(fmt.Sprintf("%s: ", playerTitle))
			racerViews.WriteString(m.racerProgressBars[i].View())
			racerViews.WriteString("\n")
		}
		content = fmt.Sprintf("%s\n%s", wordBlock, racerViews.String())
	case activeViewRaceFinished:
		if m.loading {
			content = getRaceLoadingView(m)
		} else {
			content = fmt.Sprintf("Words Per Min: %d\n\n(PRESS ENTER TO PLAY AGAIN)", m.wordsPerMin)
		}
	}
	return m.renderer.Place(
		m.termWidth,
		m.termHeight,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func getRaceLoadingView(m model) string {
	s := m.spinner.View()
	return fmt.Sprintf("%s waiting for other players %s %s", s, m.raceStartCountDown.View(), s)
}

func getErrorStyle(errMsg string) string {
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Width(80).MarginLeft(4)
	return fmt.Sprintf("\n\n%v", errorStyle.Render(errMsg))
}
