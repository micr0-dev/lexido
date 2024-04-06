package tea

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/micr0-dev/lexido/pkg/commands"
	"github.com/micr0-dev/lexido/pkg/format"
)

const maxWidth = 200

type model struct {
	spinner                spinner.Model
	commands               *[]string
	response               string
	choices                []string
	selected               []bool
	cursor                 int
	width                  int
	height                 int
	displayedContentLength int
	commandless            bool
	isDone                 bool
	hasSudo                bool
	isLocal                bool
}

type (
	AppendResponseMsg string
	GenerationDoneMsg struct{}
)

func InitialModel(commmands *[]string, local bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{
		spinner:                s,
		commands:               commmands,
		response:               "",
		choices:                make([]string, 0),
		selected:               make([]bool, 0),
		cursor:                 0,
		width:                  0,
		height:                 0,
		displayedContentLength: 0,
		commandless:            true,
		isDone:                 false,
		hasSudo:                false,
		isLocal:                local,
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

func (m model) Init() tea.Cmd {
	return tea.Batch(tickCmd(100*time.Millisecond), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.displayedContentLength >= len(m.response) && len(m.response) > 0 && m.commandless && m.isDone {
		return m.Close(false)
	}

	switch msg := msg.(type) {
	case AppendResponseMsg:
		m.response += string(msg)
		m.choices = commands.ParseCommands(m.response)
		m.selected = make([]bool, len(m.choices)+1)
		m.commandless = m.choices == nil || len(m.choices) == 0
		m.hasSudo = commands.ContainsSudo(m.choices)
	case GenerationDoneMsg:
		m.isDone = true
	case tickMsg:
		totalResponseLength := len(m.response)
		// Logic to increment displayedContentLength
		chunkSize := rand.Intn(7) + 2 // Random chunk size between 1 and 5
		m.displayedContentLength += chunkSize

		// Ensure we don't exceed the total content length
		if m.displayedContentLength > totalResponseLength {
			m.displayedContentLength = totalResponseLength
		}

		// Adjust the timing based on the proportion of the content displayed
		sleepMs := int(math.Max(float64(m.displayedContentLength)/float64(totalResponseLength)*30, 1))
		interval := time.Duration(sleepMs) * time.Millisecond

		return m, tickCmd(interval)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			return m.Close(false)
		}
		if m.commandless {
			return m, nil
		}

		switch msg.String() {
		case "enter":
			if m.cursor != len(m.choices) {
				m.selected[m.cursor] = !m.selected[m.cursor]
			} else {
				return m.Close(true)
			}
		case "j", "down":
			if m.cursor < len(m.choices) {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		}
	case tea.WindowSizeMsg:
		// Optionally store the new dimensions
		m.width = msg.Width
		m.height = msg.Height
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) Close(exec bool) (tea.Model, tea.Cmd) {
	// Collect the selected commands
	if exec {
		for i, selected := range m.selected {
			if selected {
				*m.commands = append(*m.commands, m.choices[i])
			}
		}
		fmt.Print("\n")
	}
	fmt.Print("\n")
	return m, tea.Quit
}

func (m model) View() string {
	var s strings.Builder

	s.WriteString("\033[0m")

	if m.response == "" {
		if m.isLocal {
			s.WriteString(fmt.Sprintf("%sInitializing...", m.spinner.View()))
		} else {
			s.WriteString(fmt.Sprintf("%sConnecting...", m.spinner.View()))
		}
		return s.String()
	}

	displayContent := format.TrimWhitespace(m.response)
	if len(displayContent) > m.displayedContentLength {
		displayContent = displayContent[:m.displayedContentLength]
	}

	wrappedResponse := format.WrapText(commands.HighlightCommands(displayContent), min(m.width, maxWidth))
	s.WriteString(wrappedResponse)

	if m.commandless {
		return s.String()
	}

	s.WriteString("\n—————————————————————\n")

	s.WriteString("Command List:\n\n")
	for i, todo := range m.choices {
		var selected, color string

		if m.selected[i] {
			selected = "x"
			color = "\033[32m"
		} else {
			selected = " "
			color = "\033[0m"
		}
		if m.cursor == i {
			s.WriteString(fmt.Sprintf("> "+color+"["+selected+"] %s\n", todo))
		} else {
			s.WriteString(fmt.Sprintf("  "+color+"["+selected+"] %s\n", todo))
		}
		s.WriteString("\033[0m")
	}

	if m.cursor == len(m.choices) {
		s.WriteString(">   \033[32m[RUN]\033[0m\n")
	} else {
		s.WriteString("    [RUN]\n")
	}

	if m.hasSudo {
		s.WriteString(format.WrapText("\n\033[31mWarning: This response contains sudo commands. Please thoroughly review the commands before running them.\033[0m\n", min(m.width, maxWidth)))
	}

	s.WriteString(format.WrapText("\nPlease select the tasks to run. q to quit. up/down to select", min(m.width, maxWidth)))

	return s.String()
}
