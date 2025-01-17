package tui

import (
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
)

const (
	listView uint = iota
	titleView
	bodyView
	timeView
	projectSelectView
	projectCategoiesView
	summaryNoteToday
)

type model struct {
	state         uint
	store         *Store
	notes         []Note
	currNote      Note
	listIndex     int
	textArea      textarea.Model
	textInput     textinput.Model
	textInputTime textinput.Model
	isEditing     bool

	spinner   spinner.Model
	isLoading bool

	projectCursor int
	projects      []Project
	currProject   Project

	categoriesCursor int
	categories       []Category
	currCategory     Category

	//filteredNotes []Note    // Notes filtered by the current date
	currentDate time.Time // Tracks the displayed date

	summaryNoteViewport viewport.Model
}

// Custom message for loading notes
type notesLoadedMsg struct {
	notes []Note
}

type saveCompleteMsg struct {
	notes []Note
}

type deleteCompleteMsg struct {
	notes []Note
}

func NewModel(store *Store) model {
	today := time.Now().Truncate(24 * time.Hour)

	//notes, err := store.GetNotes()
	notes, err := store.GetNotesByDate(today)

	if err != nil {
		log.Fatalf("unable to get notes: %v", err)
	}

	projects, err := store.GetProjects()
	if err != nil {
		log.Fatalf("unable to get projects: %v", err)
	}

	const witdth = 78
	vp := viewport.New(witdth, 20)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(2)

		/*
		   renderer, err := glamour.NewTermRenderer(
		       glamour.WithAutoStyle(),
		       glamour.WithWordWrap(witdth),
		   )

		   if err != nil {
		       log.Fatal("unable to render glamour viewport", err)
		   }

		   str, err := renderer.Render("aaa") // set str empty
		   if err != nil {
		       log.Fatal("unable to render content empty", err)
		   }

		   vp.SetContent(str)
		*/

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return model{
		state:               listView,
		store:               store,
		notes:               notes,
		textArea:            textarea.New(),
		textInput:           textinput.New(),
		spinner:             spin,
		isLoading:           false,
		textInputTime:       textinput.New(),
		projects:            projects,
		currentDate:         today,
		summaryNoteViewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.textArea, cmd = m.textArea.Update(msg)
	cmds = append(cmds, cmd)

	m.textInputTime, cmd = m.textInputTime.Update(msg)
	cmds = append(cmds, cmd)

	m.summaryNoteViewport, cmd = m.summaryNoteViewport.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.isLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case notesLoadedMsg:
		// Update notes after loading completes
		m.notes = msg.notes
		m.isLoading = false

	case saveCompleteMsg:
		// Handle save completion
		m.notes = msg.notes
		m.isLoading = false
		m.isEditing = false
		m.currNote = Note{}
		// reset currProject
		// or m.currProject = Project{}
		if len(m.projects) > 0 {
			m.projectCursor = 0
			m.currProject = m.projects[m.projectCursor]
		}
		if len(m.categories) > 0 {
			m.categoriesCursor = 0
			m.currCategory = m.categories[m.categoriesCursor]
		}
		m.state = listView

	case deleteCompleteMsg:
		m.notes = msg.notes
		m.isLoading = false

		if m.listIndex >= len(m.notes) && len(m.notes) > 0 {
			m.listIndex = len(m.notes) - 1 // adjust to the last note if need
		}

	case tea.KeyMsg:
		key := msg.String() //up, down, etc ...
		switch m.state {
		case listView:
			if m.isLoading {
				// Don't allow interaction with the list when loading
				return m, tea.Batch(cmds...)
			}
			switch key {
			case "q":
				return m, tea.Quit
			case "n":
				m.textInput.SetValue("")
				m.textInput.Focus()
				m.currNote = Note{}
				m.state = titleView
			case "up", "k":
				if m.listIndex > 0 {
					m.listIndex--
				}
			case "down", "j":
				if m.listIndex < len(m.notes)-1 {
					m.listIndex++
				}
			case "enter":
				m.currNote = m.notes[m.listIndex]
				m.textArea.SetValue(m.currNote.Body)
				m.textInputTime.SetValue(m.currNote.TotalTime)
				m.textArea.Focus()
				m.textArea.CursorEnd()

				m.isEditing = m.currNote.Id != "" // Set if editing

				m.state = bodyView

			case "r":
				m.isLoading = true
				//return m, m.spinner.Tick
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						// Simulate a delay (e.g., fetching notes)
						time.Sleep(1 * time.Second)
						newNotes, err := m.store.GetNotesByDate(m.currentDate)
						if err != nil {
							// Handle error (for simplicity, quit)
							return tea.Quit
						}
						return notesLoadedMsg{notes: newNotes}
					},
				)
			case "d": // Delete the seletced note
				if len(m.notes) > 0 && m.listIndex < len(m.notes) {
					m.isLoading = true
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							err := m.store.DeleteNote(m.notes[m.listIndex].Id)
							if err != nil {
								// Handle error
								return tea.Quit()
							}
							updatedNotes, err := m.store.GetNotesByDate(m.currentDate)
							if err != nil {
								return tea.Quit()
							}
							time.Sleep(300 * time.Millisecond)
							return deleteCompleteMsg{notes: updatedNotes}
						},
					)
				}
			case "ctrl+n":
				if m.currentDate.AddDate(0, 0, 1).After(time.Now()) {
					break // Prevent advancing beyond the current date
				}
				m.currentDate = m.currentDate.AddDate(0, 0, 1)
				//m.filteredNotes = filterNotesByDate(m.notes, m.currentDate)
				notes, err := m.store.GetNotesByDate(m.currentDate)
				if err != nil {
					// handle error ...
				}
				m.notes = notes
			case "ctrl+p":
				m.currentDate = m.currentDate.AddDate(0, 0, -1)
				notes, err := m.store.GetNotesByDate(m.currentDate)
				if err != nil {
					// handle error ...
				}
				m.notes = notes
			case "ctrl+g":
				today := time.Now().Truncate(24 * time.Hour)
				m.currentDate = today
				notes, err := m.store.GetNotesByDate(m.currentDate)
				if err != nil {
					// handle error ...
				}
				m.notes = notes
			case "ctrl+s":

				renderer, err := glamour.NewTermRenderer(
					glamour.WithAutoStyle(),
					glamour.WithWordWrap(78),
				)

				if err != nil {
					log.Fatal("unable to render glamour viewport", err)
				}

				notes, _ := m.store.GetNotesByDate(m.currentDate)
				content := generateNoteSummaryContent(notes)
				str, err := renderer.Render(content) // set str empty
				if err != nil {
					log.Fatal("unable to render content empty", err)
				}

				m.summaryNoteViewport.SetContent(str)
				m.state = summaryNoteToday
			}
		case summaryNoteToday:
			switch key {
			case "esc":
				m.state = listView
			}
		case titleView:
			switch key {
			case "enter":
				title := m.textInput.Value()
				if title != "" {
					m.currNote.Title = title
					m.textArea.SetValue("")
					m.textArea.Focus()
					m.textArea.CursorEnd()

					m.state = bodyView
				}
			case "esc":
				m.state = listView
			}

		case projectSelectView:
			switch key {
			case "q":
				return m, tea.Quit
			case "esc":
				m.textInputTime.Focus()
				m.textInputTime.CursorEnd()

				m.state = timeView
			case "down", "j":
				m.projectCursor++
				if m.projectCursor >= len(m.projects) {
					m.projectCursor = 0
				}
			case "up", "k":
				m.projectCursor--
				if m.projectCursor < 0 {
					m.projectCursor = len(m.projects) - 1
				}

			case "enter":
				m.currProject = m.projects[m.projectCursor]

				categories, err := m.store.GetCategoriesByProject(m.currProject.Id)
				if err != nil {
					// handle err ...
				}

				m.categories = categories

				// categories > 1 will redirect if not must will force to save
				// or set unknown or error
				if len(m.categories) > 0 {
					if m.isEditing {
						for i, category := range m.categories {
							if category.Name == m.currNote.Category.Name { // Adjust comparison if necessary
								m.categoriesCursor = i
								break
							}
						}
					}

					//m.projectCursor = 2
					m.currCategory = m.categories[m.categoriesCursor]
					m.state = projectCategoiesView
				}

				/*
								case "ctrl+s":
									body := m.textArea.Value()
									m.currNote.Body = body
									totalTime := m.textInputTime.Value()
									m.currNote.TotalTime = totalTime

									// force set currProject by cursor
									m.currProject = m.projects[m.projectCursor]

					                m.currCategory = m.categories[m.categoriesCursor]

									// Start loading spinner
									m.isLoading = true

									return m, tea.Batch(
										m.spinner.Tick,
										func() tea.Msg {
											err := m.store.SaveNoteWithProject(m.currNote, m.currProject.Id, m.currCategory.Id)
											if err != nil {
												// Handle save error (simplified for example)
												return tea.Quit
											}
											newNotes, err := m.store.GetNotes()
											if err != nil {
												// Handle fetch error (simplified for example)
												return tea.Quit
											}

											// Simulate load operation with a delay
											time.Sleep(400 * time.Millisecond) // Simulated delay
											return saveCompleteMsg{notes: newNotes}
										},
									)
				*/
			}

		case projectCategoiesView:
			switch key {
			case "q":
				return m, tea.Quit
			case "esc":
				m.state = projectSelectView
			case "enter":
				// for enter case
			case "down", "j":
				m.categoriesCursor++
				if m.categoriesCursor >= len(m.categories) {
					m.categoriesCursor = 0
				}
			case "up", "k":
				m.categoriesCursor--
				if m.categoriesCursor < 0 {
					m.categoriesCursor = len(m.categories) - 1
				}
			case "ctrl+s":
				body := m.textArea.Value()
				m.currNote.Body = body
				totalTime := m.textInputTime.Value()
				m.currNote.TotalTime = totalTime

				// force set currProject by cursor
				m.currProject = m.projects[m.projectCursor]

				m.currCategory = m.categories[m.categoriesCursor]

				// Start loading spinner
				m.isLoading = true

				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						err := m.store.SaveNoteWithProject(m.currNote, m.currProject.Id, m.currCategory.Id, m.currentDate)
						if err != nil {
							// Handle save error (simplified for example)
							return tea.Quit
						}
						newNotes, err := m.store.GetNotesByDate(m.currentDate)
						if err != nil {
							// Handle fetch error (simplified for example)
							return tea.Quit
						}

						// Simulate load operation with a delay
						time.Sleep(400 * time.Millisecond) // Simulated delay
						return saveCompleteMsg{notes: newNotes}
					},
				)
			}

		case timeView:
			switch key {
			case "q":
				return m, tea.Quit
			case "esc":
				// Make text area focus again
				m.textInputTime.Blur() // Blur time input before switching back
				m.textArea.Focus()
				m.textArea.CursorEnd()
				m.state = bodyView

			case "enter":
				// Blur textInputTime when transitioning out of timeView
				m.textInputTime.Blur()

				// set cursor when edit have project data
				// Find the index of the current project in m.projects
				if m.isEditing {
					for i, project := range m.projects {
						if project.Name == m.currNote.Project.Name { // Adjust comparison if necessary
							m.projectCursor = i
							break
						}
					}
				}

				//m.projectCursor = 2
				m.currProject = m.projects[m.projectCursor]

				m.state = projectSelectView
				// set current project if enter

				/*
					case "ctrl+s":
						body := m.textArea.Value()
						m.currNote.Body = body
						totalTime := m.textInputTime.Value()
						m.currNote.TotalTime = totalTime

						// Start loading spinner
						m.isLoading = true

						return m, tea.Batch(
							m.spinner.Tick,
							func() tea.Msg {
								err := m.store.SaveNote(m.currNote)
								if err != nil {
									// Handle save error (simplified for example)
									return tea.Quit
								}
								newNotes, err := m.store.GetNotes()
								if err != nil {
									// Handle fetch error (simplified for example)
									return tea.Quit
								}

								// Simulate load operation with a delay
								time.Sleep(400 * time.Millisecond) // Simulated delay
								return saveCompleteMsg{notes: newNotes}
							},
						)
				*/
			}
		case bodyView:
			switch key {
			case "tab":
				if m.isEditing {
					m.textInputTime.Focus()
					m.textInputTime.CursorEnd()
				} else {
					m.textInputTime.SetValue("")
					m.textInputTime.Focus()
					m.textInputTime.CursorEnd()
				}
				// Blur textArea when transitioning out of bodyView
				m.textArea.Blur()
				m.state = timeView
				/*
					case "ctrl+s":
						body := m.textArea.Value()
						m.currNote.Body = body

						// Start loading spinner
						m.isLoading = true

						return m, tea.Batch(
							m.spinner.Tick,
							func() tea.Msg {
								err := m.store.SaveNote(m.currNote)
								if err != nil {
									// Handle save error (simplified for example)
									return tea.Quit
								}
								newNotes, err := m.store.GetNotes()
								if err != nil {
									// Handle fetch error (simplified for example)
									return tea.Quit
								}

								// Simulate load operation with a delay
								time.Sleep(400 * time.Millisecond) // Simulated delay
								return saveCompleteMsg{notes: newNotes}
							},
						)

				*/
			case "esc":
				m.isEditing = false // Reset editing case
				m.textArea.Blur()   // Ensure focus is cleared
				m.state = listView
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func filterNotesByDate(notes []Note, date time.Time) []Note {
	filtered := []Note{}
	for _, note := range notes {
		if note.CreatedAt.Truncate(24 * time.Hour).Equal(date.Truncate(24 * time.Hour)) {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func generateNoteSummaryContent(notes []Note) string {
	content := "# Notes for Today\n\n"
	content += "| Title           | Time       | Detail                        |\n"
	content += "| --------------- | ---------- | --------------------------- |\n"

	// Check if there are no notes
	if len(notes) == 0 {
		content += "# No notes for today. 뇨진스🐰\n" // Add placeholder row if no notes
		return content
	}

	// Loop through the notes and generate the content
	for _, note := range notes {
		// Check if project name is empty and handle accordingly
		projectName := getFirstString(note.Project.Name)
		if projectName == "" {
			projectName = "No Project" // Placeholder if no project name
		}

		// Check if body is empty and handle accordingly
		body := note.Body
		if body == "" {
			body = "No details provided" // Placeholder if no body content
		}

		// Format the content with note data
		content += fmt.Sprintf("| %s | %s | [%s] - %s  |\n",
			note.Title,
			note.TotalTime,
			projectName,
			body,
		)
	}

	return content
}

func getFirstString(str string) string {
	if len(str) > 0 {
		return string(str[0]) // Return the first character as a string
	}
	return "" // Return empty string if input is empty
}
