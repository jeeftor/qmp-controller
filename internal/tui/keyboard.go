package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// CommonKeyMap defines common keyboard shortcuts used across TUI models
type CommonKeyMap struct {
	Quit           key.Binding
	Help           key.Binding
	Refresh        key.Binding
	Up             key.Binding
	Down           key.Binding
	Left           key.Binding
	Right          key.Binding
	Enter          key.Binding
	Space          key.Binding
	Tab            key.Binding
	ShiftTab       key.Binding
	PageUp         key.Binding
	PageDown       key.Binding
	Home           key.Binding
	End            key.Binding
	Clear          key.Binding
	TogglePause    key.Binding
	Screenshot     key.Binding
	ToggleView     key.Binding
}

// DefaultKeyMap returns the default key mappings
func DefaultKeyMap() CommonKeyMap {
	return CommonKeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c", "q", "esc"),
			key.WithHelp("ctrl+c/q/esc", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("h", "?"),
			key.WithHelp("h/?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r", "refresh"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "previous"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("pgdown", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		Clear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear"),
		),
		TogglePause: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "pause/resume"),
		),
		Screenshot: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "screenshot"),
		),
		ToggleView: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "toggle view"),
		),
	}
}

// KeyHandler provides common keyboard event handling
type KeyHandler struct {
	keyMap CommonKeyMap
}

// NewKeyHandler creates a new keyboard handler with default key mappings
func NewKeyHandler() *KeyHandler {
	return &KeyHandler{
		keyMap: DefaultKeyMap(),
	}
}

// NewKeyHandlerWithMap creates a new keyboard handler with custom key mappings
func NewKeyHandlerWithMap(keyMap CommonKeyMap) *KeyHandler {
	return &KeyHandler{
		keyMap: keyMap,
	}
}

// HandleCommonKeys handles common keyboard shortcuts and returns the appropriate action
func (kh *KeyHandler) HandleCommonKeys(msg tea.KeyMsg) (KeyAction, bool) {
	switch {
	case key.Matches(msg, kh.keyMap.Quit):
		return KeyActionQuit, true
	case key.Matches(msg, kh.keyMap.Help):
		return KeyActionHelp, true
	case key.Matches(msg, kh.keyMap.Refresh):
		return KeyActionRefresh, true
	case key.Matches(msg, kh.keyMap.Up):
		return KeyActionUp, true
	case key.Matches(msg, kh.keyMap.Down):
		return KeyActionDown, true
	case key.Matches(msg, kh.keyMap.Left):
		return KeyActionLeft, true
	case key.Matches(msg, kh.keyMap.Right):
		return KeyActionRight, true
	case key.Matches(msg, kh.keyMap.Enter):
		return KeyActionEnter, true
	case key.Matches(msg, kh.keyMap.Space):
		return KeyActionSpace, true
	case key.Matches(msg, kh.keyMap.Tab):
		return KeyActionTab, true
	case key.Matches(msg, kh.keyMap.ShiftTab):
		return KeyActionShiftTab, true
	case key.Matches(msg, kh.keyMap.PageUp):
		return KeyActionPageUp, true
	case key.Matches(msg, kh.keyMap.PageDown):
		return KeyActionPageDown, true
	case key.Matches(msg, kh.keyMap.Home):
		return KeyActionHome, true
	case key.Matches(msg, kh.keyMap.End):
		return KeyActionEnd, true
	case key.Matches(msg, kh.keyMap.Clear):
		return KeyActionClear, true
	case key.Matches(msg, kh.keyMap.TogglePause):
		return KeyActionTogglePause, true
	case key.Matches(msg, kh.keyMap.Screenshot):
		return KeyActionScreenshot, true
	case key.Matches(msg, kh.keyMap.ToggleView):
		return KeyActionToggleView, true
	}

	return KeyActionNone, false
}

// KeyAction represents common keyboard actions
type KeyAction int

const (
	KeyActionNone KeyAction = iota
	KeyActionQuit
	KeyActionHelp
	KeyActionRefresh
	KeyActionUp
	KeyActionDown
	KeyActionLeft
	KeyActionRight
	KeyActionEnter
	KeyActionSpace
	KeyActionTab
	KeyActionShiftTab
	KeyActionPageUp
	KeyActionPageDown
	KeyActionHome
	KeyActionEnd
	KeyActionClear
	KeyActionTogglePause
	KeyActionScreenshot
	KeyActionToggleView
)

// String returns a string representation of the key action
func (ka KeyAction) String() string {
	switch ka {
	case KeyActionQuit:
		return "quit"
	case KeyActionHelp:
		return "help"
	case KeyActionRefresh:
		return "refresh"
	case KeyActionUp:
		return "up"
	case KeyActionDown:
		return "down"
	case KeyActionLeft:
		return "left"
	case KeyActionRight:
		return "right"
	case KeyActionEnter:
		return "enter"
	case KeyActionSpace:
		return "space"
	case KeyActionTab:
		return "tab"
	case KeyActionShiftTab:
		return "shift+tab"
	case KeyActionPageUp:
		return "page_up"
	case KeyActionPageDown:
		return "page_down"
	case KeyActionHome:
		return "home"
	case KeyActionEnd:
		return "end"
	case KeyActionClear:
		return "clear"
	case KeyActionTogglePause:
		return "toggle_pause"
	case KeyActionScreenshot:
		return "screenshot"
	case KeyActionToggleView:
		return "toggle_view"
	default:
		return "none"
	}
}

// GetKeyMap returns the current key mapping
func (kh *KeyHandler) GetKeyMap() CommonKeyMap {
	return kh.keyMap
}

// UpdateKeyMap updates the key mapping
func (kh *KeyHandler) UpdateKeyMap(keyMap CommonKeyMap) {
	kh.keyMap = keyMap
}

// GetHelpText returns help text for all mapped keys
func (kh *KeyHandler) GetHelpText() map[string]string {
	return map[string]string{
		kh.keyMap.Quit.Help().Key:         kh.keyMap.Quit.Help().Desc,
		kh.keyMap.Help.Help().Key:         kh.keyMap.Help.Help().Desc,
		kh.keyMap.Refresh.Help().Key:      kh.keyMap.Refresh.Help().Desc,
		kh.keyMap.Up.Help().Key:           kh.keyMap.Up.Help().Desc,
		kh.keyMap.Down.Help().Key:         kh.keyMap.Down.Help().Desc,
		kh.keyMap.Left.Help().Key:         kh.keyMap.Left.Help().Desc,
		kh.keyMap.Right.Help().Key:        kh.keyMap.Right.Help().Desc,
		kh.keyMap.Enter.Help().Key:        kh.keyMap.Enter.Help().Desc,
		kh.keyMap.Space.Help().Key:        kh.keyMap.Space.Help().Desc,
		kh.keyMap.Tab.Help().Key:          kh.keyMap.Tab.Help().Desc,
		kh.keyMap.ShiftTab.Help().Key:     kh.keyMap.ShiftTab.Help().Desc,
		kh.keyMap.PageUp.Help().Key:       kh.keyMap.PageUp.Help().Desc,
		kh.keyMap.PageDown.Help().Key:     kh.keyMap.PageDown.Help().Desc,
		kh.keyMap.Home.Help().Key:         kh.keyMap.Home.Help().Desc,
		kh.keyMap.End.Help().Key:          kh.keyMap.End.Help().Desc,
		kh.keyMap.Clear.Help().Key:        kh.keyMap.Clear.Help().Desc,
		kh.keyMap.TogglePause.Help().Key:  kh.keyMap.TogglePause.Help().Desc,
		kh.keyMap.Screenshot.Help().Key:   kh.keyMap.Screenshot.Help().Desc,
		kh.keyMap.ToggleView.Help().Key:   kh.keyMap.ToggleView.Help().Desc,
	}
}

// ViewManager handles switching between different views in a TUI
type ViewManager struct {
	currentView int
	viewNames   []string
	views       map[int]string
}

// NewViewManager creates a new view manager
func NewViewManager(viewNames []string) *ViewManager {
	views := make(map[int]string)
	for i, name := range viewNames {
		views[i] = name
	}

	return &ViewManager{
		currentView: 0,
		viewNames:   viewNames,
		views:       views,
	}
}

// GetCurrentView returns the current view index
func (vm *ViewManager) GetCurrentView() int {
	return vm.currentView
}

// GetCurrentViewName returns the current view name
func (vm *ViewManager) GetCurrentViewName() string {
	if name, exists := vm.views[vm.currentView]; exists {
		return name
	}
	return ""
}

// NextView switches to the next view
func (vm *ViewManager) NextView() {
	vm.currentView = (vm.currentView + 1) % len(vm.viewNames)
}

// PreviousView switches to the previous view
func (vm *ViewManager) PreviousView() {
	vm.currentView--
	if vm.currentView < 0 {
		vm.currentView = len(vm.viewNames) - 1
	}
}

// SetView sets the current view by index
func (vm *ViewManager) SetView(index int) {
	if index >= 0 && index < len(vm.viewNames) {
		vm.currentView = index
	}
}

// SetViewByName sets the current view by name
func (vm *ViewManager) SetViewByName(name string) {
	for i, viewName := range vm.viewNames {
		if viewName == name {
			vm.currentView = i
			return
		}
	}
}

// GetViewNames returns all view names
func (vm *ViewManager) GetViewNames() []string {
	return vm.viewNames
}
