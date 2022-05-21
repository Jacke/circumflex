package list

import (
	"clx/bheader"
	"clx/bubble/ranking"
	"clx/constants/category"
	"clx/constants/style"
	"clx/core"
	"clx/history"
	"clx/hn"
	"clx/hn/services/hybrid"
	"clx/hn/services/mock"
	"clx/item"
	"clx/screen"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	numberOfCategories = 4
)

// Item is an item that appears in the list.
//type Item interface{}

// ItemDelegate encapsulates the general functionality for all list items. The
// benefit to separating this logic from the item itself is that you can change
// the functionality of items without changing the actual items themselves.
//
// Note that if the delegate also implements help.KeyMap delegate-related
// help items will be added to the help view.
type ItemDelegate interface {
	// Render renders the item's view.
	Render(w io.Writer, m Model, index int, item *item.Item)

	// Height is the height of the list item.
	Height() int

	// Spacing is the size of the horizontal gap between list items in cells.
	Spacing() int

	// Update is the update loop for items. All messages in the list's update
	// loop will pass through here except when the user is setting a filter.
	// Use this method to perform item-level updates appropriate to this
	// delegate.
	Update(msg tea.Msg, m *Model) tea.Cmd
}

type statusMessageTimeoutMsg struct{}
type fetchingFinished struct{}

// Model contains the state of this component.
type Model struct {
	showTitle     bool
	showStatusBar bool
	disableInput  bool

	Title  string
	Styles Styles

	// Key mappings for navigating the list.
	KeyMap KeyMap

	disableQuitKeybindings bool

	// Additional key mappings for the short and full help views. This allows
	// you to add additional key mappings to the help menu without
	// re-implementing the help component. Of course, you can also disable the
	// list's help component and implement a new one if you need more
	// flexibility.
	AdditionalShortHelpKeys func() []key.Binding
	AdditionalFullHelpKeys  func() []key.Binding

	spinner     spinner.Model
	showSpinner bool
	width       int
	height      int
	Paginator   paginator.Model
	cursor      int
	onStartup   bool
	onStartup2  bool

	StatusMessageLifetime time.Duration

	statusMessage      string
	statusMessageTimer *time.Timer

	category int
	items    [][]*item.Item

	delegate ItemDelegate
	history  history.History
	config   *core.Config
	service  hn.Service
}

func (m *Model) FetchFrontPageStories() tea.Cmd {
	return func() tea.Msg {
		stories := m.service.FetchStories(0, 0)

		m.items[category.FrontPage] = stories
		return fetchingFinished{}
	}
}

func New(delegate ItemDelegate, config *core.Config, width, height int) Model {
	styles := DefaultStyles()

	sp := spinner.New()
	sp.Spinner = getSpinner()
	sp.Style = styles.Spinner

	p := paginator.New()
	p.Type = paginator.Dots
	p.ActiveDot = styles.ActivePaginationDot.String()
	p.InactiveDot = styles.InactivePaginationDot.String()

	items := make([][]*item.Item, numberOfCategories)

	m := Model{
		showTitle:             true,
		showStatusBar:         true,
		KeyMap:                DefaultKeyMap(),
		Styles:                styles,
		Title:                 "List",
		StatusMessageLifetime: time.Second,

		width:        width,
		height:       height,
		delegate:     delegate,
		history:      getHistory(config.DebugMode, config.MarkAsRead),
		items:        items,
		Paginator:    p,
		spinner:      sp,
		onStartup:    true,
		disableInput: true,
		config:       config,
		service:      getService(config.DebugMode),
	}

	m.service.Init(30)

	m.updatePagination()
	return m
}

func getHistory(debugMode bool, markAsRead bool) history.History {
	if debugMode {
		return history.NewMockHistory()
	}

	if markAsRead {
		return history.NewPersistentHistory()
	}

	return history.NewNonPersistentHistory()
}

func getService(debugMode bool) hn.Service {
	if debugMode {
		return mock.MockService{}
	}

	return &hybrid.Service{}
}

// NewModel returns a new model with sensible defaults.
//
// Deprecated. Use New instead.
var NewModel = New

// SetShowTitle shows or hides the title bar.
func (m *Model) SetShowTitle(v bool) {
	m.showTitle = v
	m.updatePagination()
}

// ShowTitle returns whether or not the title bar is set to be rendered.
func (m Model) ShowTitle() bool {
	return m.showTitle
}

// SetShowStatusBar shows or hides the view that displays metadata about the
// list, such as item counts.
func (m *Model) SetShowStatusBar(v bool) {
	m.showStatusBar = v
	m.updatePagination()
}

// ShowStatusBar returns whether or not the status bar is set to be rendered.
func (m Model) ShowStatusBar() bool {
	return m.showStatusBar
}

// Items returns the items in the list.
func (m Model) Items() []*item.Item {
	return m.items[m.category]
}

// Set the items available in the list. This returns a command.
func (m *Model) SetItems(i []*item.Item) tea.Cmd {
	var cmd tea.Cmd
	m.items[m.category] = i

	m.updatePagination()
	return cmd
}

// Select selects the given index of the list and goes to its respective page.
func (m *Model) Select(index int) {
	m.Paginator.Page = index / m.Paginator.PerPage
	m.cursor = index % m.Paginator.PerPage
}

// ResetSelected resets the selected item to the first item in the first page of the list.
func (m *Model) ResetSelected() {
	m.Select(0)
}

// Set the item delegate.
func (m *Model) SetDelegate(d ItemDelegate) {
	m.delegate = d
	m.updatePagination()
}

// VisibleItems returns the total items available to be shown.
func (m Model) VisibleItems() []*item.Item {
	return m.items[m.category]
}

// SelectedItems returns the current selected item in the list.
func (m Model) SelectedItem() *item.Item {
	i := m.Index()

	items := m.VisibleItems()
	if i < 0 || len(items) == 0 || len(items) <= i {
		//return nil
		return &item.Item{}
	}

	return items[i]
}

// Index returns the index of the currently selected item as it appears in the
// entire slice of items.
func (m Model) Index() int {
	return m.Paginator.Page*m.Paginator.PerPage + m.cursor
}

// Cursor returns the index of the cursor on the current page.
func (m Model) Cursor() int {
	return m.cursor
}

// CursorUp moves the cursor up. This can also move the state to the previous
// page.
func (m *Model) CursorUp() {
	m.cursor--

	// If we're at the top, stop
	if m.cursor < 0 {
		m.cursor = 0
		return
	}

	return
}

// CursorDown moves the cursor down. This can also advance the state to the
// next page.
func (m *Model) CursorDown() {
	itemsOnPage := m.Paginator.ItemsOnPage(len(m.VisibleItems()))

	m.cursor++

	// If we're at the end, stop
	if m.cursor < itemsOnPage {
		return
	}

	m.cursor = itemsOnPage - 1
}

func (m Model) PrevPage() {
	m.Paginator.PrevPage()
}

func (m Model) NextPage() {
	m.Paginator.NextPage()
}

func (m *Model) NextCategory() {
	isAtLastCategory := m.category == numberOfCategories-1
	if isAtLastCategory {
		m.selectCategory(category.FrontPage)

		return
	}

	m.selectCategory(m.category + 1)
}

func (m *Model) PreviousCategory() {
	isAtFirstCategory := m.category == category.FrontPage
	if isAtFirstCategory {
		m.selectCategory(category.Show)

		return
	}

	m.selectCategory(m.category - 1)
}

func (m *Model) selectCategory(category int) {
	m.category = category
	categoryIsEmpty := len(m.items[category]) == 0

	if !categoryIsEmpty {
		m.Paginator.Page = 0
		m.updatePagination()

		return
	}

	service := new(mock.MockService)
	stories := service.FetchStories(0, m.category)

	// Randomize list to make debugging easier
	rand.Shuffle(len(stories), func(i, j int) { stories[i], stories[j] = stories[j], stories[i] })

	m.items[category] = stories

	m.Paginator.Page = 0
	m.updatePagination()

	return
}

// Width returns the current width setting.
func (m Model) Width() int {
	return m.width
}

// Height returns the current height setting.
func (m Model) Height() int {
	return m.height
}

// SetSpinner allows to set the spinner style.
func (m *Model) SetSpinner(spinner spinner.Spinner) {
	m.spinner.Spinner = spinner
}

// Toggle the spinner. Note that this also returns a command.
func (m *Model) ToggleSpinner() tea.Cmd {
	if !m.showSpinner {
		return m.StartSpinner()
	}
	m.StopSpinner()
	return nil
}

// StartSpinner starts the spinner. Note that this returns a command.
func (m *Model) StartSpinner() tea.Cmd {
	m.showSpinner = true
	return m.spinner.Tick
}

// StopSpinner stops the spinner.
func (m *Model) StopSpinner() {
	m.showSpinner = false
}

// Helper for disabling the keybindings used for quitting, incase you want to
// handle this elsewhere in your application.
func (m *Model) DisableQuitKeybindings() {
	m.disableQuitKeybindings = true
	m.KeyMap.Quit.SetEnabled(false)
	m.KeyMap.ForceQuit.SetEnabled(false)
}

// NewStatusMessage sets a new status message, which will show for a limited
// amount of time. Note that this also returns a command.
func (m *Model) NewStatusMessage(s string) tea.Cmd {
	m.statusMessage = s
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}

	m.statusMessageTimer = time.NewTimer(m.StatusMessageLifetime)

	// Wait for timeout
	return func() tea.Msg {
		<-m.statusMessageTimer.C
		return statusMessageTimeoutMsg{}
	}
}

func (m *Model) NewStatusMessageWithDuration(s string, d time.Duration) tea.Cmd {
	m.statusMessage = s
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}

	m.statusMessageTimer = time.NewTimer(d)

	// Wait for timeout
	return func() tea.Msg {
		<-m.statusMessageTimer.C
		return statusMessageTimeoutMsg{}
	}
}

// SetSize sets the width and height of this component.
func (m *Model) SetSize(width, height int) {
	m.setSize(width, height)
}

// SetWidth sets the width of this component.
func (m *Model) SetWidth(v int) {
	m.setSize(v, m.height)
}

// SetHeight sets the height of this component.
func (m *Model) SetHeight(v int) {
	m.setSize(m.width, v)
}

func (m *Model) setSize(width, height int) {
	m.width = width
	m.height = height
	m.updatePagination()
}

// Update pagination according to the amount of items for the current state.
func (m *Model) updatePagination() {
	index := m.Index()
	availHeight := m.height

	if m.showTitle {
		availHeight -= lipgloss.Height(m.titleView())
	}
	if m.showStatusBar {
		availHeight -= lipgloss.Height(m.statusView())
	}

	m.Paginator.PerPage = max(1, availHeight/(m.delegate.Height()+m.delegate.Spacing()))

	if pages := len(m.VisibleItems()); pages < 1 {
		m.Paginator.SetTotalPages(1)
	} else {
		m.Paginator.SetTotalPages(pages)
	}

	// Restore index
	m.Paginator.Page = index / m.Paginator.PerPage
	m.cursor = index % m.Paginator.PerPage

	// Make sure the page stays in bounds
	if m.Paginator.Page >= m.Paginator.TotalPages-1 {
		m.Paginator.Page = max(0, m.Paginator.TotalPages-1)
	}
}

func (m *Model) hideStatusMessage() {
	m.statusMessage = ""
	if m.statusMessageTimer != nil {
		m.statusMessageTimer.Stop()
	}
}

// Update is the Bubble Tea update loop.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.KeyMap.ForceQuit) {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		newSpinnerModel, cmd := m.spinner.Update(msg)
		m.spinner = newSpinnerModel
		if m.showSpinner {
			cmds = append(cmds, cmd)
		}

	case fetchingFinished:
		m.StopSpinner()
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.setSize(screen.GetTerminalWidth()-h, screen.GetTerminalHeight()-v)
		m.disableInput = false

		return m, nil

	case statusMessageTimeoutMsg:
		m.hideStatusMessage()
	}

	cmds = append(cmds, m.handleBrowsing(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) handleBrowsing(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	numItems := len(m.VisibleItems())

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Quit):
			return tea.Quit

		case key.Matches(msg, m.KeyMap.CursorUp):
			m.CursorUp()

		case key.Matches(msg, m.KeyMap.CursorDown):
			m.CursorDown()

		case key.Matches(msg, m.KeyMap.PrevPage):
			m.Paginator.PrevPage()

		case key.Matches(msg, m.KeyMap.NextPage):
			m.Paginator.NextPage()

		case key.Matches(msg, m.KeyMap.NextCategory):
			m.NextCategory()

		case key.Matches(msg, m.KeyMap.PreviousCategory):
			m.PreviousCategory()

		case key.Matches(msg, m.KeyMap.GoToStart):
			m.Paginator.Page = 0
			m.cursor = 0

		case key.Matches(msg, m.KeyMap.GoToEnd):
			m.Paginator.Page = m.Paginator.TotalPages - 1
			m.cursor = m.Paginator.ItemsOnPage(numItems) - 1
		}
	}

	cmd := m.delegate.Update(msg, m)
	cmds = append(cmds, cmd)

	// Keep the index in bounds when paginating
	itemsOnPage := m.Paginator.ItemsOnPage(len(m.VisibleItems()))
	if m.cursor > itemsOnPage-1 {
		m.cursor = max(0, itemsOnPage-1)
	}

	return tea.Batch(cmds...)
}

// View renders the component.
func (m Model) View() string {
	var (
		sections    []string
		availHeight = m.height
	)

	if m.showTitle {
		v := m.titleView()
		sections = append(sections, v)
		availHeight -= lipgloss.Height(v)
	}

	if m.showStatusBar {
		v := m.statusView()
		availHeight -= lipgloss.Height(v)
	}

	content := lipgloss.NewStyle().Height(availHeight).Render(m.populatedView())
	rankings := ranking.GetRankings(false, m.Paginator.PerPage, len(m.items[m.category]), m.cursor,
		m.Paginator.Page, m.Paginator.TotalPages)

	rankingsAndContent := lipgloss.JoinHorizontal(lipgloss.Top, rankings, content)
	sections = append(sections, rankingsAndContent)

	if m.showStatusBar {
		v := m.statusAndPaginationView()
		sections = append(sections, v)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) titleView() string {
	return bheader.GetHeader(m.category, m.width) + "\n"
}

func (m Model) statusAndPaginationView() string {
	centerContent := ""

	if m.showSpinner {
		centerContent = m.spinnerView()
	} else {
		centerContent = m.statusMessage
	}

	left := lipgloss.NewStyle().Inline(true).
		Background(lipgloss.AdaptiveColor{Light: style.HeaderBackgroundLight, Dark: style.HeaderBackgroundDark}).
		Width(5).MaxWidth(5).Render("")

	center := lipgloss.NewStyle().Inline(true).
		Background(lipgloss.AdaptiveColor{Light: style.HeaderBackgroundLight, Dark: style.HeaderBackgroundDark}).
		Width(m.width - 5 - 5).Align(lipgloss.Center).Render(centerContent)

	right := lipgloss.NewStyle().Inline(true).
		Background(lipgloss.AdaptiveColor{Light: style.LogoBackgroundLight, Dark: style.LogoBackgroundDark}).
		Width(5).Align(lipgloss.Center).Render(m.Paginator.View())

	return m.Styles.StatusBar.Render(left) + m.Styles.StatusBar.Render(center) + m.Styles.StatusBar.Render(right)
}

func (m Model) statusView() string {
	var status string

	visibleItems := len(m.VisibleItems())

	plural := ""
	if visibleItems != 1 {
		plural = "s"
	}

	if len(m.items) == 0 {
		status = m.Styles.StatusEmpty.Render("")
	} else {
		status += fmt.Sprintf("%d item%s", visibleItems, plural)
	}

	return m.Styles.StatusBar.Render(status)
}

func (m Model) OnStartup() bool {
	return m.onStartup
}

func (m *Model) IsInputDisabled() bool {
	return m.disableInput
}

func (m *Model) SetDisabledInput(value bool) {
	m.disableInput = value
}

func (m *Model) SetOnStartup(value bool) {
	m.onStartup = value
}

func (m Model) populatedView() string {
	items := m.VisibleItems()

	var b strings.Builder

	// Empty states
	if len(items) == 0 {
		return m.Styles.NoItems.Render("")
	}

	if len(items) > 0 {
		start, end := m.Paginator.GetSliceBounds(len(items))
		docs := items[start:end]

		for i, item := range docs {
			m.delegate.Render(&b, m, i+start, item)
			if i != len(docs)-1 {
				fmt.Fprint(&b, strings.Repeat("\n", m.delegate.Spacing()+1))
			}
		}
	}

	// If there aren't enough items to fill up this page (always the last page)
	// then we need to add some newlines to fill up the space where items would
	// have been.
	itemsOnPage := m.Paginator.ItemsOnPage(len(items))
	if itemsOnPage < m.Paginator.PerPage {
		n := (m.Paginator.PerPage - itemsOnPage) * (m.delegate.Height() + m.delegate.Spacing())
		if len(items) == 0 {
			n -= m.delegate.Height() - 1
		}
		fmt.Fprint(&b, strings.Repeat("\n", n))
	}

	return b.String()
}

func (m Model) spinnerView() string {
	return m.spinner.View()
}

func max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func getSpinner() spinner.Spinner {
	fg := lipgloss.AdaptiveColor{Light: style.UnselectedItemLight, Dark: style.UnselectedPageDark}
	bg := lipgloss.AdaptiveColor{Light: style.HeaderBackgroundLight, Dark: style.HeaderBackgroundDark}
	normal := lipgloss.NewStyle().Foreground(fg).Background(bg)
	color := normal.Copy()

	magenta := lipgloss.Color(style.MagentaDark)
	yellow := lipgloss.Color(style.YellowDark)
	blue := lipgloss.Color(style.BlueDark)

	return spinner.Spinner{
		Frames: []string{
			normal.Render("fetching"),
			normal.Render("fetching"),
			normal.Render("fetching"),
			normal.Render("fetching"),
			normal.Render("fetching"),
			normal.Render("fetching"),
			color.Foreground(blue).Render("f") + lipgloss.NewStyle().Foreground(fg).Background(bg).Render("etching"),
			color.Foreground(yellow).Render("f") + color.Foreground(blue).Render("e") + normal.Render("tching"),
			color.Foreground(magenta).Render("f") + color.Foreground(yellow).Render("e") + color.Foreground(blue).Render("t") + normal.Render("ching"),
			normal.Render("f") + color.Foreground(magenta).Render("e") + color.Foreground(yellow).Render("t") + color.Foreground(blue).Render("c") + normal.Render("hing"),
			normal.Render("fe") + color.Foreground(magenta).Render("t") + color.Foreground(yellow).Render("c") + color.Foreground(blue).Render("h") + normal.Render("ing"),
			normal.Render("fet") + color.Foreground(magenta).Render("c") + color.Foreground(yellow).Render("h") + color.Foreground(blue).Render("i") + normal.Render("ng"),
			normal.Render("fetc") + color.Foreground(magenta).Render("h") + color.Foreground(yellow).Render("i") + color.Foreground(blue).Render("n") + normal.Render("g"),
			normal.Render("fetch") + color.Foreground(magenta).Render("i") + color.Foreground(yellow).Render("n") + color.Foreground(blue).Render("g"),
			normal.Render("fetchi") + color.Foreground(magenta).Render("n") + color.Foreground(yellow).Render("g"),
			normal.Render("fetchin") + color.Foreground(magenta).Render("g"),
			normal.Render("fetching"),
		},
		FPS: 150 * time.Millisecond,
	}
}
