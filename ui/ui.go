package ui

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI holds the components of the terminal UI
type UI struct {
	app         *tview.Application
	topBar      *tview.TextView
	footer      *tview.Box
	textContent *tview.TextView
	userList    *tview.List
	layout      *tview.Flex
	mutex       sync.Mutex // To ensure thread-safe updates to UI
}

// NewUI initializes and returns a new UI instance
func NewUI(inputChan chan string) *UI {
	// Create a new application
	app := tview.NewApplication()

	// Top bar with black text on white background
	topBar := tview.NewTextView().
		SetDynamicColors(true)

	// Set the full text with different styles
	topBar.SetText(" [black]gochat v1.0.0[-]").SetBackgroundColor(tcell.ColorWhite)

	// Scrollable content area
	content := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false) // Turn off wrap so you can control horizontal scrolling
	content.SetBorder(true)

	// Create a list of active users
	userList := tview.NewList().SetHighlightFullLine(true).ShowSecondaryText(false)

	// Populate the user list with some example users
	userList.AddItem(" Active users", "", 0, nil).
		AddItem(" me", "", 0, nil)

	userList.
		SetBorder(true)

	// Create a Flex layout to arrange text content and user list horizontally
	contentFlex := tview.NewFlex().
		AddItem(content, 0, 2, true).   // Main text content area takes most of the space
		AddItem(userList, 30, 1, false) // User list with fixed width of 30 characters

	// Input field at the bottom
	inputField := tview.NewInputField().
		SetLabel(" Shell: ").
		SetFieldBackgroundColor(tcell.ColorBlack).
		SetFieldTextColor(tcell.ColorWhite)

	// Create a Flex layout with three parts: content area and input shell
	contentLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentFlex, 0, 1, false). // Let the content fill the available space
		AddItem(inputField, 3, 1, true)    // Set a height of 3 lines for the input field

	// Create a main layout that adds the user list to the left of the content
	mainLayout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topBar, 1, 1, false).
		AddItem(contentLayout, 0, 1, true) // Rest of the space for the content layout

	// Set focus on the input field
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			inputText := inputField.GetText()
			inputChan <- inputText // Send input to the main program

			content.Write([]byte("me: " + inputText + "\n"))
			content.ScrollToEnd() // Scroll to the latest line when input is entered
			inputField.SetText("")
		}
	})

	// Set up key events to scroll content manually
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := content.GetScrollOffset() // Get current row and column offsets
		switch event.Key() {
		case tcell.KeyUp:
			content.ScrollTo(row-1, col) // Scroll up by one row
		case tcell.KeyDown:
			content.ScrollTo(row+1, col) // Scroll down by one row
		}
		return event
	})

	return &UI{
		app:         app,
		topBar:      topBar,
		textContent: content,
		userList:    userList,
		layout:      mainLayout,
	}
}

// AppendContent appends a new message to the text content area in a thread-safe way
func (ui *UI) AppendContent(message string) {
	ui.mutex.Lock()
	defer ui.mutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		ui.textContent.Write([]byte(message + "\n"))
		ui.textContent.ScrollToEnd() // Scroll to the latest line when input is entered
	})
}

// Add a user to the list of active users in a thread-safe way
func (ui *UI) AddUser(user string) {
	ui.mutex.Lock()
	defer ui.mutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		ui.userList.AddItem(user, "", 0, nil)
	})
}

// Remove a user from the list of active users in a thread-safe way
func (ui *UI) RemoveUser(user string) {
	ui.mutex.Lock()
	defer ui.mutex.Unlock()

	ui.app.QueueUpdateDraw(func() {
		for i := 0; i < ui.userList.GetItemCount(); i++ {
			mainText, _ := ui.userList.GetItemText(i)
			if mainText == user {
				ui.userList.RemoveItem(i)
				break
			}
		}
	})
}

// Run starts the UI application in a separate Goroutine
func (ui *UI) Run() {
	go func() {
		if err := ui.app.SetRoot(ui.layout, true).Run(); err != nil {
			panic(err)
		}
	}()
}
