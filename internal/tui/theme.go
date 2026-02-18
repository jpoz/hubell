package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/jpoz/hubell/internal/config"
)

// Theme holds all color slots the app needs.
type Theme struct {
	Name string

	// UI chrome
	Error           lipgloss.Color
	HelpText        lipgloss.Color
	FocusedBorder   lipgloss.Color
	UnfocusedBorder lipgloss.Color

	// Banner animation endpoints (RGB)
	BannerDark   [3]int
	BannerBright [3]int

	// CI / PR status
	StatusSuccess lipgloss.Color
	StatusFailure lipgloss.Color
	StatusPending lipgloss.Color

	// List styling
	Title              lipgloss.Color
	TitleBar           lipgloss.Color
	SelectedForeground lipgloss.Color
	SelectedDesc       lipgloss.Color
	NormalForeground   lipgloss.Color
	NormalDesc         lipgloss.Color

	// Timeline event colors
	TimelineCreated  lipgloss.Color
	TimelineApproved lipgloss.Color
	TimelineMerged   lipgloss.Color

	// General
	Accent lipgloss.Color
	Subtle lipgloss.Color
}

// Built-in themes keyed by lowercase identifier.
var themes = map[string]Theme{
	"default": {
		Name:               "Default",
		Error:              lipgloss.Color("9"),
		HelpText:           lipgloss.Color("241"),
		FocusedBorder:      lipgloss.Color("62"),
		UnfocusedBorder:    lipgloss.Color("241"),
		BannerDark:         [3]int{80, 80, 100},
		BannerBright:       [3]int{95, 135, 175},
		StatusSuccess:      lipgloss.Color("42"),
		StatusFailure:      lipgloss.Color("196"),
		StatusPending:      lipgloss.Color("214"),
		Title:              lipgloss.Color("62"),
		TitleBar:           lipgloss.Color("236"),
		SelectedForeground: lipgloss.Color("62"),
		SelectedDesc:       lipgloss.Color("246"),
		NormalForeground:   lipgloss.Color("255"),
		NormalDesc:         lipgloss.Color("241"),
		TimelineCreated:    lipgloss.Color("33"),
		TimelineApproved:   lipgloss.Color("42"),
		TimelineMerged:     lipgloss.Color("135"),
		Accent:             lipgloss.Color("62"),
		Subtle:             lipgloss.Color("241"),
	},
	"nord": {
		Name:               "Nord",
		Error:              lipgloss.Color("#BF616A"),
		HelpText:           lipgloss.Color("#4C566A"),
		FocusedBorder:      lipgloss.Color("#88C0D0"),
		UnfocusedBorder:    lipgloss.Color("#4C566A"),
		BannerDark:         [3]int{59, 66, 82},
		BannerBright:       [3]int{136, 192, 208},
		StatusSuccess:      lipgloss.Color("#A3BE8C"),
		StatusFailure:      lipgloss.Color("#BF616A"),
		StatusPending:      lipgloss.Color("#EBCB8B"),
		Title:              lipgloss.Color("#88C0D0"),
		TitleBar:           lipgloss.Color("#3B4252"),
		SelectedForeground: lipgloss.Color("#88C0D0"),
		SelectedDesc:       lipgloss.Color("#D8DEE9"),
		NormalForeground:   lipgloss.Color("#ECEFF4"),
		NormalDesc:         lipgloss.Color("#4C566A"),
		TimelineCreated:    lipgloss.Color("#81A1C1"),
		TimelineApproved:   lipgloss.Color("#A3BE8C"),
		TimelineMerged:     lipgloss.Color("#B48EAD"),
		Accent:             lipgloss.Color("#88C0D0"),
		Subtle:             lipgloss.Color("#4C566A"),
	},
	"dracula": {
		Name:               "Dracula",
		Error:              lipgloss.Color("#FF5555"),
		HelpText:           lipgloss.Color("#6272A4"),
		FocusedBorder:      lipgloss.Color("#BD93F9"),
		UnfocusedBorder:    lipgloss.Color("#6272A4"),
		BannerDark:         [3]int{68, 71, 90},
		BannerBright:       [3]int{189, 147, 249},
		StatusSuccess:      lipgloss.Color("#50FA7B"),
		StatusFailure:      lipgloss.Color("#FF5555"),
		StatusPending:      lipgloss.Color("#F1FA8C"),
		Title:              lipgloss.Color("#BD93F9"),
		TitleBar:           lipgloss.Color("#44475A"),
		SelectedForeground: lipgloss.Color("#BD93F9"),
		SelectedDesc:       lipgloss.Color("#F8F8F2"),
		NormalForeground:   lipgloss.Color("#F8F8F2"),
		NormalDesc:         lipgloss.Color("#6272A4"),
		TimelineCreated:    lipgloss.Color("#8BE9FD"),
		TimelineApproved:   lipgloss.Color("#50FA7B"),
		TimelineMerged:     lipgloss.Color("#BD93F9"),
		Accent:             lipgloss.Color("#BD93F9"),
		Subtle:             lipgloss.Color("#6272A4"),
	},
	"catppuccin": {
		Name:               "Catppuccin Mocha",
		Error:              lipgloss.Color("#F38BA8"),
		HelpText:           lipgloss.Color("#585B70"),
		FocusedBorder:      lipgloss.Color("#CBA6F7"),
		UnfocusedBorder:    lipgloss.Color("#585B70"),
		BannerDark:         [3]int{49, 50, 68},
		BannerBright:       [3]int{203, 166, 247},
		StatusSuccess:      lipgloss.Color("#A6E3A1"),
		StatusFailure:      lipgloss.Color("#F38BA8"),
		StatusPending:      lipgloss.Color("#F9E2AF"),
		Title:              lipgloss.Color("#CBA6F7"),
		TitleBar:           lipgloss.Color("#313244"),
		SelectedForeground: lipgloss.Color("#CBA6F7"),
		SelectedDesc:       lipgloss.Color("#CDD6F4"),
		NormalForeground:   lipgloss.Color("#CDD6F4"),
		NormalDesc:         lipgloss.Color("#585B70"),
		TimelineCreated:    lipgloss.Color("#89B4FA"),
		TimelineApproved:   lipgloss.Color("#A6E3A1"),
		TimelineMerged:     lipgloss.Color("#CBA6F7"),
		Accent:             lipgloss.Color("#CBA6F7"),
		Subtle:             lipgloss.Color("#585B70"),
	},
	"solarized": {
		Name:               "Solarized Dark",
		Error:              lipgloss.Color("#DC322F"),
		HelpText:           lipgloss.Color("#586E75"),
		FocusedBorder:      lipgloss.Color("#268BD2"),
		UnfocusedBorder:    lipgloss.Color("#586E75"),
		BannerDark:         [3]int{0, 43, 54},
		BannerBright:       [3]int{38, 139, 210},
		StatusSuccess:      lipgloss.Color("#859900"),
		StatusFailure:      lipgloss.Color("#DC322F"),
		StatusPending:      lipgloss.Color("#B58900"),
		Title:              lipgloss.Color("#268BD2"),
		TitleBar:           lipgloss.Color("#073642"),
		SelectedForeground: lipgloss.Color("#268BD2"),
		SelectedDesc:       lipgloss.Color("#93A1A1"),
		NormalForeground:   lipgloss.Color("#FDF6E3"),
		NormalDesc:         lipgloss.Color("#586E75"),
		TimelineCreated:    lipgloss.Color("#268BD2"),
		TimelineApproved:   lipgloss.Color("#859900"),
		TimelineMerged:     lipgloss.Color("#6C71C4"),
		Accent:             lipgloss.Color("#268BD2"),
		Subtle:             lipgloss.Color("#586E75"),
	},
	"gruvbox": {
		Name:               "Gruvbox",
		Error:              lipgloss.Color("#FB4934"),
		HelpText:           lipgloss.Color("#665C54"),
		FocusedBorder:      lipgloss.Color("#FE8019"),
		UnfocusedBorder:    lipgloss.Color("#665C54"),
		BannerDark:         [3]int{60, 56, 54},
		BannerBright:       [3]int{254, 128, 25},
		StatusSuccess:      lipgloss.Color("#B8BB26"),
		StatusFailure:      lipgloss.Color("#FB4934"),
		StatusPending:      lipgloss.Color("#FABD2F"),
		Title:              lipgloss.Color("#FE8019"),
		TitleBar:           lipgloss.Color("#3C3836"),
		SelectedForeground: lipgloss.Color("#FE8019"),
		SelectedDesc:       lipgloss.Color("#EBDBB2"),
		NormalForeground:   lipgloss.Color("#EBDBB2"),
		NormalDesc:         lipgloss.Color("#665C54"),
		TimelineCreated:    lipgloss.Color("#83A598"),
		TimelineApproved:   lipgloss.Color("#B8BB26"),
		TimelineMerged:     lipgloss.Color("#D3869B"),
		Accent:             lipgloss.Color("#FE8019"),
		Subtle:             lipgloss.Color("#665C54"),
	},
	"tokyonight": {
		Name:               "Tokyo Night",
		Error:              lipgloss.Color("#F7768E"),
		HelpText:           lipgloss.Color("#565F89"),
		FocusedBorder:      lipgloss.Color("#7AA2F7"),
		UnfocusedBorder:    lipgloss.Color("#565F89"),
		BannerDark:         [3]int{26, 27, 38},
		BannerBright:       [3]int{122, 162, 247},
		StatusSuccess:      lipgloss.Color("#9ECE6A"),
		StatusFailure:      lipgloss.Color("#F7768E"),
		StatusPending:      lipgloss.Color("#E0AF68"),
		Title:              lipgloss.Color("#7AA2F7"),
		TitleBar:           lipgloss.Color("#1A1B26"),
		SelectedForeground: lipgloss.Color("#7AA2F7"),
		SelectedDesc:       lipgloss.Color("#C0CAF5"),
		NormalForeground:   lipgloss.Color("#C0CAF5"),
		NormalDesc:         lipgloss.Color("#565F89"),
		TimelineCreated:    lipgloss.Color("#7AA2F7"),
		TimelineApproved:   lipgloss.Color("#9ECE6A"),
		TimelineMerged:     lipgloss.Color("#BB9AF7"),
		Accent:             lipgloss.Color("#7AA2F7"),
		Subtle:             lipgloss.Color("#565F89"),
	},
	"rosepine": {
		Name:               "Rose Pine",
		Error:              lipgloss.Color("#EB6F92"),
		HelpText:           lipgloss.Color("#6E6A86"),
		FocusedBorder:      lipgloss.Color("#C4A7E7"),
		UnfocusedBorder:    lipgloss.Color("#6E6A86"),
		BannerDark:         [3]int{35, 33, 54},
		BannerBright:       [3]int{196, 167, 231},
		StatusSuccess:      lipgloss.Color("#31748F"),
		StatusFailure:      lipgloss.Color("#EB6F92"),
		StatusPending:      lipgloss.Color("#F6C177"),
		Title:              lipgloss.Color("#C4A7E7"),
		TitleBar:           lipgloss.Color("#1F1D2E"),
		SelectedForeground: lipgloss.Color("#C4A7E7"),
		SelectedDesc:       lipgloss.Color("#E0DEF4"),
		NormalForeground:   lipgloss.Color("#E0DEF4"),
		NormalDesc:         lipgloss.Color("#6E6A86"),
		TimelineCreated:    lipgloss.Color("#9CCFD8"),
		TimelineApproved:   lipgloss.Color("#31748F"),
		TimelineMerged:     lipgloss.Color("#C4A7E7"),
		Accent:             lipgloss.Color("#C4A7E7"),
		Subtle:             lipgloss.Color("#6E6A86"),
	},
}

// themeOrder defines the display order in the selector.
var themeOrder = []string{
	"default",
	"nord",
	"dracula",
	"catppuccin",
	"solarized",
	"gruvbox",
	"tokyonight",
	"rosepine",
}

// GetTheme returns the theme for the given key, falling back to default.
func GetTheme(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["default"]
}

// newThemedDelegate creates a list delegate styled with the given theme.
func newThemedDelegate(t Theme) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(t.SelectedForeground).
		BorderLeftForeground(t.Accent)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Foreground(t.SelectedDesc).
		BorderLeftForeground(t.Accent)
	d.Styles.NormalTitle = d.Styles.NormalTitle.
		Foreground(t.NormalForeground)
	d.Styles.NormalDesc = d.Styles.NormalDesc.
		Foreground(t.NormalDesc)
	return d
}

// applyListTheme sets the title style on a list model.
func applyListTheme(l *list.Model, t Theme) {
	l.Styles.Title = l.Styles.Title.
		Foreground(t.Title).
		Background(t.TitleBar)
}

// ThemeItem implements list.Item for the theme picker.
type ThemeItem struct {
	key  string
	name string
}

func (i ThemeItem) FilterValue() string { return i.name }
func (i ThemeItem) Title() string       { return i.name }
func (i ThemeItem) Description() string { return "" }

// buildThemeList creates the theme selector list model.
func buildThemeList() list.Model {
	items := make([]list.Item, len(themeOrder))
	for i, key := range themeOrder {
		items[i] = ThemeItem{key: key, name: themes[key].Name}
	}

	d := list.NewDefaultDelegate()
	d.ShowDescription = false

	l := list.New(items, d, 30, len(themeOrder)*3+4)
	l.Title = "Select Theme"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	return l
}

// applyTheme switches the active theme and persists it.
func (m *Model) applyTheme(name string) {
	m.theme = GetTheme(name)

	// Re-theme notification list
	nd := newThemedDelegate(m.theme)
	m.list.SetDelegate(nd)
	applyListTheme(&m.list, m.theme)

	// Re-theme PR list
	pd := newPRDelegate(m.theme)
	m.prList.SetDelegate(pd)
	applyListTheme(&m.prList, m.theme)

	// Re-theme timeline list
	td := newTimelineDelegate(m.theme)
	m.timelineList.SetDelegate(td)
	applyListTheme(&m.timelineList, m.theme)

	// Rebuild theme list so it picks up new styling
	m.themeList = buildThemeList()

	_ = config.SaveTheme(name)
}
