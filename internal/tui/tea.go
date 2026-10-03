package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jonathanngiroux-star/oneprobe/internal/store"
)

// teaModel adapts Model to bubbletea. All logic lives in Model; this
// is the terminal plumbing only.
type teaModel struct {
	Model
}

// Run starts the TUI against the shared store. Blocks until quit.
func Run(s *store.Store) error {
	m := teaModel{New(s)}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init starts the refresh ticker.
func (t teaModel) Init() tea.Cmd {
	return tick(t.tickEvery)
}

func tick(every time.Duration) tea.Cmd {
	return tea.Tick(every, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
}

// Update translates bubbletea messages into Model events.
func (t teaModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return t, nil

	case tea.KeyMsg:
		normalized := normalizeKey(msg)
		next := t.Model.update(keyMsg(normalized))
		t.Model = next
		if t.quitting {
			return t, tea.Quit
		}
		return t, tick(t.tickEvery)

	case refreshMsg:
		next := t.Model.update(refreshMsg{})
		t.Model = next
		return t, tick(t.tickEvery)
	}
	return t, nil
}

// normalizeKey maps tea.KeyMsg strings onto the Model's keyMsg space:
// named keys keep their tea name; printable runes go as themselves.
func normalizeKey(msg tea.KeyMsg) string {
	switch msg.String() {
	case "tab", "shift+tab", "enter", "backspace", "ctrl+c", "q":
		return msg.String()
	}
	s := msg.String()
	// A KeyRunes message can carry MULTIPLE runes when input arrives
	// fast (terminal coalescing, paste). Feed them all through as one
	// string — the Model appends it to the input whole.
	if len(s) > 0 && msg.Type == tea.KeyRunes {
		return s
	}
	return ""
}

// View renders the current screen.
func (t teaModel) View() string {
	return t.Model.View()
}

var _ = strings.TrimSpace
