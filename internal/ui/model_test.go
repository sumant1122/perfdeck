package ui

import (
	"testing"

	"github.com/sumant1122/perfdeck/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelNavigation(t *testing.T) {
	// Setup a model with 3 tabs
	m := NewModel()
	m.tabs = []config.Tab{
		{Title: "Tab 1", Cmd: []string{"echo", "1"}},
		{Title: "Tab 2", Cmd: []string{"echo", "2"}},
		{Title: "Tab 3", Cmd: []string{"echo", "3"}},
	}
	m.active = 0

	// Test moving right
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}} // vim binding 'l'
	newM, _ := m.Update(msg)
	updatedM, ok := newM.(Model)
	if !ok {
		t.Fatal("Expected Model type")
	}

	if updatedM.active != 1 {
		t.Errorf("Expected active tab 1, got %d", updatedM.active)
	}

	// Test moving right again
	newM, _ = updatedM.Update(msg)
	updatedM, ok = newM.(Model)
	if !ok {
		t.Fatal("Expected Model type")
	}

	if updatedM.active != 2 {
		t.Errorf("Expected active tab 2, got %d", updatedM.active)
	}

	// Test wrapping around
	newM, _ = updatedM.Update(msg)
	updatedM, ok = newM.(Model)
	if !ok {
		t.Fatal("Expected Model type")
	}

	if updatedM.active != 0 {
		t.Errorf("Expected wrap around to 0, got %d", updatedM.active)
	}

	// Test moving left (wrapping back)
	leftMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	newM, _ = updatedM.Update(leftMsg)
	updatedM, ok = newM.(Model)
	if !ok {
		t.Fatal("Expected Model type")
	}

	if updatedM.active != 2 {
		t.Errorf("Expected wrap back to 2, got %d", updatedM.active)
	}
}

func TestThemeToggle(t *testing.T) {
	m := NewModel()
	initialTheme := m.themeIndex

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	newM, _ := m.Update(msg)
	updatedM, ok := newM.(Model)
	if !ok {
		t.Fatal("Expected Model type")
	}

	if updatedM.themeIndex == initialTheme {
		t.Error("Theme index should change after pressing 't'")
	}
}

func TestQuit(t *testing.T) {
	m := NewModel()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("Expected quit command")
	}
	// We can't easily check if cmd is actually Quit without internal access,
	// but getting a command back is a good sign here.
}
