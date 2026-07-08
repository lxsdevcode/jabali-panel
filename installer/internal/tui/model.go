// Package tui is the Bubble Tea installer front-end (M353 / GH #353). It walks
// the operator through a deploy profile, an optional-module checklist with live
// dependency resolution, and a confirm screen, then hands the selected module
// set back to main, which execs install.sh with JABALI_MODULES=<keys>.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"git.jabali-panel.com/shukivaknin/jabali2/installer/internal/modules"
)

type screen int

const (
	screenWelcome screen = iota
	screenProfile
	screenModules
	screenConfirm
	screenDone
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	onStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	offStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// Model is the installer state machine.
type Model struct {
	screen   screen
	cursor   int
	profiles []modules.Profile
	mods     []modules.Module
	selected map[string]bool // optional-module key -> enabled
	// Result flags read by main after the program exits.
	Confirmed bool
	Aborted   bool
}

// New builds the initial model with the "Web host" profile pre-selected.
func New() Model {
	return Model{
		screen:   screenWelcome,
		profiles: modules.Profiles,
		mods:     modules.Registry,
		selected: map[string]bool{},
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) SelectedKeys() []string {
	var out []string
	for _, mod := range m.mods { // registry order
		if m.selected[mod.Key] {
			out = append(out, mod.Key)
		}
	}
	return out
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		m.Aborted = true
		return m, tea.Quit
	}
	switch m.screen {
	case screenWelcome:
		if key.String() == "enter" {
			m.screen = screenProfile
			m.cursor = 0
		}
	case screenProfile:
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			p := m.profiles[m.cursor]
			m.selected = map[string]bool{}
			for _, k := range p.Modules {
				m.selected[k] = true
			}
			m.selected = modules.Resolve(m.selected, "")
			m.screen = screenModules
			m.cursor = 0
		}
	case screenModules:
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.mods)-1 {
				m.cursor++
			}
		case " ", "space", "x":
			mod := m.mods[m.cursor]
			if m.selected[mod.Key] {
				delete(m.selected, mod.Key)
				m.selected = modules.Resolve(m.selected, "")
			} else {
				m.selected[mod.Key] = true
				m.selected = modules.Resolve(m.selected, mod.Key)
			}
		case "enter":
			m.screen = screenConfirm
		}
	case screenConfirm:
		switch key.String() {
		case "enter", "y":
			m.Confirmed = true
			m.screen = screenDone
			return m, tea.Quit
		case "b", "backspace", "left":
			m.screen = screenModules
		}
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	switch m.screen {
	case screenWelcome:
		b.WriteString(titleStyle.Render("Jabali Panel installer"))
		b.WriteString("\n\nModular install: pick a deploy profile, then fine-tune the\noptional modules. Core services (web, database, panel) are\nalways installed.\n\n")
		b.WriteString(helpStyle.Render("enter: continue   q: quit"))
	case screenProfile:
		b.WriteString(titleStyle.Render("Deploy profile"))
		b.WriteString("\n\n")
		for i, p := range m.profiles {
			cur := "  "
			if i == m.cursor {
				cur = cursorStyle.Render("> ")
			}
			b.WriteString(fmt.Sprintf("%s%s\n    %s\n", cur, p.Title, helpStyle.Render(p.Desc)))
		}
		b.WriteString("\n" + helpStyle.Render("↑/↓: move   enter: select   q: quit"))
	case screenModules:
		b.WriteString(titleStyle.Render("Optional modules"))
		b.WriteString("\n\n")
		for i, mod := range m.mods {
			cur := "  "
			if i == m.cursor {
				cur = cursorStyle.Render("> ")
			}
			box := offStyle.Render("[ ]")
			if m.selected[mod.Key] {
				box = onStyle.Render("[x]")
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n      %s\n", cur, box, mod.Title, helpStyle.Render(mod.Desc)))
		}
		b.WriteString("\n" + helpStyle.Render("↑/↓: move   space: toggle   enter: continue   q: quit"))
		b.WriteString("\n" + helpStyle.Render("(dependencies auto-select; conflicts auto-clear)"))
	case screenConfirm:
		b.WriteString(titleStyle.Render("Confirm"))
		b.WriteString("\n\nCore: web (nginx + PHP-FPM), database (MariaDB), panel + auth\n\nOptional modules to install:\n")
		keys := m.SelectedKeys()
		if len(keys) == 0 {
			b.WriteString(helpStyle.Render("  (none — minimal install)\n"))
		}
		for _, k := range keys {
			b.WriteString(onStyle.Render("  + " + k + "\n"))
		}
		b.WriteString("\n" + helpStyle.Render("JABALI_MODULES=") + onStyle.Render(strings.Join(keys, ",")) + "\n")
		b.WriteString("\n" + helpStyle.Render("enter/y: install   b: back   q: quit"))
	case screenDone:
		b.WriteString(titleStyle.Render("Starting install…"))
	}
	b.WriteString("\n")
	return b.String()
}
