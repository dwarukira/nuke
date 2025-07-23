package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	flag "github.com/spf13/pflag"
)

type folder struct {
	Path     string
	Size     string
	Selected bool
}

type model struct {
	cursor     int
	folders    []folder
	filtered   []folder
	quitting   bool
	deleted    []string
	dryRun     bool
	loading    bool
	searchMode bool
	searchTerm string
	scanPath   string
}

func main() {
	var root string
	var dryRun bool
	var showVersion bool

	flag.StringVarP(&root, "path", "p", ".", "Root directory to start from")
	flag.BoolVarP(&dryRun, "dry-run", "d", false, "Preview deletions without removing anything")
	flag.BoolVar(&showVersion, "version", false, "Print version info")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `
Usage:
  nuke [--path <dir>] [--dry-run]

Options:
  -p, --path      Directory to scan for node_modules (default ".")
  -d, --dry-run   Show what would be deleted, don't actually delete
      --version   Show version information
  -h, --help      Show this help message

Interactive Controls:
  ↑/↓ or j/k      Move
  Space           Select/deselect
  Enter           Confirm deletion
  /               Search
  esc             Exit search
  q               Quit
`)
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("nuke version: %s\ncommit: %s\ndate: %s\n", version, commit, date)
		os.Exit(0)
	}
	flag.Parse()

	if !isTerminal() {
		fmt.Println("This app must be run in a terminal with interactive input.")
		os.Exit(1)
	}

	m := model{
		loading:  true,
		dryRun:   dryRun,
		scanPath: root,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func isTerminal() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		folders := findNodeModules(m.scanPath)
		return folders
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case []folder:
		m.folders = msg
		m.filtered = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if m.searchMode {
			switch msg.Type {
			case tea.KeyEsc:
				m.searchMode = false
				m.searchTerm = ""
				m.filtered = m.folders
				return m, nil
			case tea.KeyEnter:
				m.searchMode = false
				return m, nil
			case tea.KeyBackspace:
				if len(m.searchTerm) > 0 {
					m.searchTerm = m.searchTerm[:len(m.searchTerm)-1]
					m.filtered = filterFolders(m.folders, m.searchTerm)
				}
			default:
				m.searchTerm += msg.String()
				m.filtered = filterFolders(m.folders, m.searchTerm)
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case " ":
			if len(m.filtered) > 0 {
				idx := m.findFolderIndex(m.filtered[m.cursor].Path)
				m.folders[idx].Selected = !m.folders[idx].Selected
				m.filtered[m.cursor].Selected = m.folders[idx].Selected
			}
		case "enter":
			var deleted []string
			for _, f := range m.folders {
				if f.Selected {
					if !m.dryRun {
						_ = os.RemoveAll(f.Path)
					}
					deleted = append(deleted, f.Path)
				}
			}
			m.deleted = deleted
			m.quitting = true
			return m, tea.Quit
		case "/":
			m.searchMode = true
			m.searchTerm = ""
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "\033[H\033[2J" // clear terminal and move cursor to top

	if m.loading {
		return s + headerStyle.Render("Scanning for node_modules folders...\n")
	}

	if m.quitting {
		if len(m.deleted) == 0 {
			return s + quitStyle.Render("Aborted. No folders deleted.\n")
		}
		title := titleStyle.Render("Deleted the following folders:")
		if m.dryRun {
			title = titleStyle.Render("Dry run — these would be deleted:")
		}
		body := ""
		for _, d := range m.deleted {
			body += itemStyle.Render("  - " + d + "\n")
		}
		return s + lipgloss.JoinVertical(lipgloss.Left, title, "\n", body)
	}

	header := "Select node_modules folders to delete (↑↓ to move, space to select, enter to delete, q to quit, / to search):\n"
	if m.dryRun {
		header = "DRY RUN MODE — nothing will be deleted!\n\n" + header
	}
	s += headerStyle.Render(header) + "\n"

	if m.searchMode {
		s += inputStyle.Render("/" + m.searchTerm + "\n\n")
	}

	if len(m.filtered) == 0 {
		s += itemStyle.Render("No matching folders.\n")
		return s
	}

	for i, f := range m.filtered {
		cursor := " "
		if m.cursor == i {
			cursor = cursorStyle.Render(">")
		}
		checked := " "
		if f.Selected {
			checked = checkedStyle.Render("x")
		}
		nameCol := lipgloss.NewStyle().Width(15).Render("node_modules")
		sizeCol := lipgloss.NewStyle().Width(8).Render(f.Size)
		pathCol := dimStyle.Render(f.Path)
		line := fmt.Sprintf("%s [%s] %s %s %s\n", cursor, checked, nameCol, sizeCol, pathCol)
		s += line
	}

	return s + "\n"
}

func (m model) findFolderIndex(path string) int {
	for i, f := range m.folders {
		if f.Path == path {
			return i
		}
	}
	return -1
}

func findNodeModules(root string) []folder {
	var results []folder
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == "node_modules" {
			size := getDirSize(path)
			results = append(results, folder{Path: path, Size: size})
			return filepath.SkipDir
		}
		return nil
	})
	return results
}

func getDirSize(path string) string {
	out, err := exec.Command("du", "-sh", path).Output()
	if err != nil {
		return "?"
	}
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return fields[0]
	}
	return "?"
}

func filterFolders(folders []folder, term string) []folder {
	var filtered []folder
	term = strings.ToLower(term)
	for _, f := range folders {
		if strings.Contains(strings.ToLower(f.Path), term) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// Styling
var (
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	itemStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202"))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	dimStyle     = lipgloss.NewStyle().Faint(true)
	quitStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	inputStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
