package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// EnvVar holds a key and its description from the .env.example file
type EnvVar struct {
	Key          string
	Description  string
	ExampleValue string // Value from .env.example
}

type model struct {
	form              *huh.Form
	envVars           []EnvVar
	existingEnvValues map[string]string
	fields            []huh.Field
	width, height     int
	quitting          bool
	err               error

	// Fields for confirmation step
	confirming      bool
	confirmForm     *huh.Form
	envValuesToSave map[string]string
	applyChanges    bool   // To store the result of the confirm form
	diffSummary     string // To store the formatted diff for display
}

func initialModel() *model {
	return &model{
		applyChanges: true, // Default to true, will be set by confirm form
	}
}

func (m *model) Init() tea.Cmd {
	var err error
	m.envVars, err = readEnvVarsFromFile(".env.example")
	if err != nil {
		m.err = fmt.Errorf("Error reading .env.example: %w. Please create one to use as a template.", err)
		return tea.Quit
	}
	if len(m.envVars) == 0 {
		m.err = fmt.Errorf("No environment variables found in .env.example.")
		return tea.Quit
	}

	m.existingEnvValues, err = readExistingEnvFile(".env")
	if err != nil {
		fmt.Printf("Warning: could not read existing .env file to prefill: %v\n", err)
		m.existingEnvValues = make(map[string]string)
	}

	m.fields = make([]huh.Field, 0, len(m.envVars))
	for _, envVar := range m.envVars {
		localKey := envVar.Key
		initialValue, valueExistsInEnv := m.existingEnvValues[localKey]
		if !valueExistsInEnv || initialValue == "" {
			if envVar.ExampleValue != "" {
				initialValue = envVar.ExampleValue
			}
		}
		fieldValuePtr := new(string)
		*fieldValuePtr = initialValue

		inputField := huh.NewInput().
			Title(localKey).
			Value(fieldValuePtr)

		if envVar.Description != "" {
			inputField = inputField.Description(envVar.Description)
		}
		m.fields = append(m.fields, inputField)
	}

	customKeyMap := huh.NewDefaultKeyMap()
	customKeyMap.Quit = key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc/ctrl+c", "quit"))
	customKeyMap.Input.Next = key.NewBinding(key.WithKeys("enter", "tab", "down"), key.WithHelp("enter/tab/↓", "next"))
	customKeyMap.Input.Prev = key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab/↑", "prev"))

	m.form = huh.NewForm(
		huh.NewGroup(m.fields...).
			Title("Setup your .env values"),
	).WithTheme(huh.ThemeCharm()).WithKeyMap(customKeyMap).WithWidth(80)

	return m.form.Init()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		return m, tea.Quit
	}
	if m.quitting {
		return m, tea.Quit
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Optionally, update form widths if they support it
		// m.form = m.form.WithWidth(m.width)
		// if m.confirmForm != nil {
		// 	m.confirmForm = m.confirmForm.WithWidth(m.width)
		// }

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	}

	if m.confirming {
		// Process confirmation form
		newConfirmForm, confirmCmd := m.confirmForm.Update(msg)
		if cf, ok := newConfirmForm.(*huh.Form); ok {
			m.confirmForm = cf
			cmds = append(cmds, confirmCmd)
		}

		if m.confirmForm.State == huh.StateCompleted {
			// Confirmation received, m.applyChanges holds the boolean result
			if m.applyChanges {
				err := m.actuallyWriteEnvFile(m.envValuesToSave)
				if err != nil {
					m.err = err // Store error to display it after TUI exits
				}
			} else {
				fmt.Println("\nChanges discarded by user.") // This will print after TUI exits
			}
			m.quitting = true
			return m, tea.Quit
		}
		if m.confirmForm.State == huh.StateAborted {
			fmt.Println("\nSave operation cancelled by user.") // This will print after TUI exits
			m.quitting = true
			return m, tea.Quit
		}
	} else {
		// Process main form
		newMainForm, mainCmd := m.form.Update(msg)
		if mf, ok := newMainForm.(*huh.Form); ok {
			m.form = mf
			cmds = append(cmds, mainCmd)
		}

		if m.form.State == huh.StateCompleted {
			// Main form completed, now switch to confirmation state
			err := m.prepareForConfirmation() // This will set m.confirming = true and m.diffSummary
			if err != nil {
				m.err = err
				m.quitting = true
				return m, tea.Quit
			}
			if m.quitting { // If prepareForConfirmation decided to quit (no changes)
				return m, tea.Quit
			}
			// Do not quit yet, stay in confirming state.
			// We need to return Init for the confirmForm
			if m.confirming && m.confirmForm != nil {
				cmds = append(cmds, m.confirmForm.Init())
			}
		}
		if m.form.State == huh.StateAborted {
			fmt.Println("\nOperation cancelled by user (main form aborted).") // This will print after TUI exits
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}
	if m.quitting {
		return ""
	}

	if m.confirming && m.confirmForm != nil {
		// Display diff summary above the confirmation form
		return fmt.Sprintf("Proposed changes:\n%s\n\n%s", m.diffSummary, m.confirmForm.View())
	}
	// For the main form
	return m.form.View()
}

// prepareForConfirmation collects values and sets up the confirmation form
func (m *model) prepareForConfirmation() error {
	collectedEnvValues := make(map[string]string)
	for i, envVar := range m.envVars {
		inputField, ok := m.fields[i].(*huh.Input)
		if !ok {
			return fmt.Errorf("error: could not cast field for key %s to huh.Input", envVar.Key)
		}
		val := inputField.GetValue().(string)
		collectedEnvValues[envVar.Key] = val
	}

	var diffLines []string
	changed := false
	for _, envVar := range m.envVars {
		key := envVar.Key
		oldValue, oldExists := m.existingEnvValues[key]
		newValue := collectedEnvValues[key]
		if !oldExists && newValue != "" {
			diffLines = append(diffLines, fmt.Sprintf("+ Added: %s=\"%s\"", key, newValue))
			changed = true
		} else if oldExists && newValue != oldValue {
			if newValue == "" {
				diffLines = append(diffLines, fmt.Sprintf("~ Cleared: %s (was \"%s\")", key, oldValue))
			} else {
				diffLines = append(diffLines, fmt.Sprintf("~ Changed: %s: \"%s\" -> \"%s\"", key, oldValue, newValue))
			}
			changed = true
		} else if !oldExists && newValue == "" {
			diffLines = append(diffLines, fmt.Sprintf("+ Added: %s=\"\"", key))
			changed = true
		}
	}

	if !changed {
		// fmt.Println("\nNo changes to apply to .env file.") // This would be overwritten
		m.diffSummary = "No changes to apply to .env file." // Store for potential display or just quit
		m.quitting = true                                   // No changes, so we can quit directly
		return nil
	}

	// Store values and prepare confirmation form
	m.envValuesToSave = collectedEnvValues
	m.diffSummary = strings.Join(diffLines, "\n") // Store formatted diff

	// m.applyChanges is already true by default, huh.Confirm will set it to false if "No"
	confirmField := huh.NewConfirm().
		Title("Save these changes?").
		Affirmative("Save to .env"). // Text for Yes
		Negative("Discard changes"). // Text for No
		Value(&m.applyChanges)       // Bind to model field

	confirmKeyMap := huh.NewDefaultKeyMap()

	m.confirmForm = huh.NewForm(
		huh.NewGroup(confirmField).Title("Confirmation"),
	).WithTheme(huh.ThemeCharm()).WithKeyMap(confirmKeyMap).WithWidth(60) // Adjust width as needed

	m.confirming = true
	return nil
}

// actuallyWriteEnvFile performs the file writing operations
func (m *model) actuallyWriteEnvFile(envValues map[string]string) error {
	// These fmt.Println calls will appear after the TUI exits
	info, statErr := os.Stat(".env")
	if statErr == nil {
		if !info.IsDir() {
			fmt.Println("\nBacking up existing .env to .env.old...")
			backupErr := backupEnvFile(".env", ".env.old")
			if backupErr != nil {
				fmt.Printf("Warning: Failed to backup .env: %v\n", backupErr)
			} else {
				fmt.Println("Successfully backed up .env to .env.old.")
			}
		} else {
			fmt.Printf("Warning: .env exists but is a directory. Skipping backup.\n")
		}
	} else if !os.IsNotExist(statErr) {
		fmt.Printf("Warning: Error checking .env for backup: %v\n", statErr)
	}

	err := writeEnvFile(envValues)
	if err != nil {
		fmt.Printf("\nError writing .env file: %v\n", err)
		return fmt.Errorf("error writing .env file: %w", err)
	}
	fmt.Println("\n✅ Successfully updated the .env file!")
	return nil
}

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	if fm, ok := finalModel.(*model); ok {
		if fm.err != nil {
			// If no changes were made, diffSummary will contain "No changes..."
			// and fm.err will be nil if prepareForConfirmation quit early.
			// Only print fm.err if it's a real error.
			if fm.diffSummary != "No changes to apply to .env file." || fm.err.Error() != "" { // Check if it's not the "no changes" message
				fmt.Printf("%v\n", fm.err)
			}
			if strings.Contains(fm.err.Error(), ".env.example file not found") || strings.Contains(fm.err.Error(), "No environment variables found") {
				os.Exit(1)
			}
		} else if fm.diffSummary == "No changes to apply to .env file." && !fm.confirming {
			// If quitting because no changes, print the summary.
			// This ensures "No changes to apply..." is visible if that was the reason for quitting.
			fmt.Println("\n" + fm.diffSummary)
		}
	}
}

func readEnvVarsFromFile(filePath string) ([]EnvVar, error) {
	exampleFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer exampleFile.Close()

	var envVars []EnvVar
	scanner := bufio.NewScanner(exampleFile)
	for scanner.Scan() {
		line := scanner.Text()
		lineForParsing := strings.TrimSpace(line)
		if lineForParsing == "" || strings.HasPrefix(lineForParsing, "#") {
			continue
		}
		var key, description, exampleValue string
		keyValuePart := lineForParsing
		commentIdx := strings.Index(lineForParsing, "#")
		if commentIdx != -1 {
			description = strings.TrimSpace(lineForParsing[commentIdx+1:])
			keyValuePart = strings.TrimSpace(lineForParsing[:commentIdx])
		}
		parts := strings.SplitN(keyValuePart, "=", 2)
		if len(parts) > 0 {
			key = strings.TrimSpace(parts[0])
			if len(parts) == 2 {
				valStr := strings.TrimSpace(parts[1])
				if len(valStr) >= 2 && valStr[0] == '"' && valStr[len(valStr)-1] == '"' {
					unquotedValue := valStr[1 : len(valStr)-1]
					unquotedValue = strings.ReplaceAll(unquotedValue, `\\`, `\`)
					unquotedValue = strings.ReplaceAll(unquotedValue, `\"`, `"`)
					exampleValue = unquotedValue
				} else if valStr != "" {
					exampleValue = valStr
				}
			}
			if key != "" {
				envVars = append(envVars, EnvVar{Key: key, Description: description, ExampleValue: exampleValue})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s: %w", filePath, err)
	}
	return envVars, nil
}

func writeEnvFile(values map[string]string) error {
	envFile, err := os.Create(".env")
	if err != nil {
		return err
	}
	defer envFile.Close()

	for key, value := range values {
		needsQuoting := false
		if value == "" {
			needsQuoting = true
		} else {
			if strings.ContainsAny(value, " #=\"$\\`\n\r") {
				needsQuoting = true
			}
		}
		var outputValue string
		if needsQuoting {
			escaped := strings.ReplaceAll(value, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, "\n", `\n`)
			escaped = strings.ReplaceAll(escaped, "\r", `\r`)
			outputValue = `"` + escaped + `"`
		} else {
			outputValue = value
		}
		if _, err := envFile.WriteString(fmt.Sprintf("%s=%s\n", key, outputValue)); err != nil {
			return err
		}
	}
	return nil
}

func readExistingEnvFile(filePath string) (map[string]string, error) {
	values := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, fmt.Errorf("error opening %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := parts[1]
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				unquotedValue := value[1 : len(value)-1]
				unquotedValue = strings.ReplaceAll(unquotedValue, `\\`, `\`)
				unquotedValue = strings.ReplaceAll(unquotedValue, `\"`, `"`)
				value = unquotedValue
			}
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return values, fmt.Errorf("error reading %s during scan: %w", filePath, err)
	}
	return values, nil
}

func backupEnvFile(srcPath, dstPath string) error {
	sourceFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy from %s to %s: %w", srcPath, dstPath, err)
	}
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file %s: %w", dstPath, err)
	}
	return nil
}
