package main

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	var content string
	switch m.activeView {
	case activeViewWelcome:
		if m.loading {
			s := m.spinner.View()
			content = fmt.Sprintf("%s starting race %s", s, s)
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



PRESS ENTER TO START



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
		content = formatWordBlock(
			m.raceWordsCharSlice,
			m.correctPos,
			m.incorrectPos,
		)
	case activeViewRaceFinished:
		content = fmt.Sprintf("Words Per Min: %d", m.wordsPerMin)
	}
	return m.renderer.Place(
		m.termWidth,
		m.termHeight,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
