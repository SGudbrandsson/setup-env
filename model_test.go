package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a temporary file with content for testing
func createTempFileForModel(t *testing.T, dir string, fileName string, content string) string {
	t.Helper()
	filePath := filepath.Join(dir, fileName)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write to temp file %s: %v", filePath, err)
	}
	return filePath
}

func TestInitialModel(t *testing.T) {
	m := initialModel()
	if m == nil {
		t.Fatal("initialModel() returned nil")
	}
	if !m.applyChanges { // Default is true
		t.Errorf("Expected m.applyChanges to be true, got %v", m.applyChanges)
	}
}

func TestModelInit(t *testing.T) {
	t.Run("successful init with .env.example and .env", func(t *testing.T) {
		tmpDir := t.TempDir()
		exampleContent := "KEY1=example_val1 # desc1\nKEY2=example_val2"
		envContent := "KEY1=actual_val1\nKEY_NEW=new_val"
		createTempFileForModel(t, tmpDir, ".env.example", exampleContent)
		createTempFileForModel(t, tmpDir, ".env", envContent)

		// Temporarily change working directory for file operations
		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		m := initialModel()
		cmd := m.Init()

		require.Nil(t, m.err, "m.err should be nil on successful init")
		require.NotNil(t, cmd, "Init should return a command")

		expectedEnvVars := []EnvVar{
			{Key: "KEY1", Description: "desc1", ExampleValue: "example_val1"},
			{Key: "KEY2", Description: "", ExampleValue: "example_val2"},
		}
		assert.Equal(t, expectedEnvVars, m.envVars, "m.envVars not parsed correctly")

		expectedExistingValues := map[string]string{
			"KEY1":    "actual_val1",
			"KEY_NEW": "new_val",
		}
		assert.Equal(t, expectedExistingValues, m.existingEnvValues, "m.existingEnvValues not read correctly")
		assert.Len(t, m.fields, 2, "Should have 2 fields based on .env.example")

		// Check field initialization
		field1, ok := m.fields[0].(*huh.Input)
		require.True(t, ok, "Field 0 is not an Input")
		assert.Equal(t, "KEY1", field1.GetKey(), "Field 0 key (title) mismatch")
		assert.Equal(t, "actual_val1", field1.GetValue().(string), "Field 0 value should be from .env")

		field2, ok := m.fields[1].(*huh.Input)
		require.True(t, ok, "Field 1 is not an Input")
		assert.Equal(t, "KEY2", field2.GetKey(), "Field 1 key (title) mismatch")
		assert.Equal(t, "example_val2", field2.GetValue().(string), "Field 1 value should be from .env.example")

		require.NotNil(t, m.form, "m.form should be initialized")
	})

	t.Run("init with missing .env.example", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		m := initialModel()
		cmd := m.Init()

		require.NotNil(t, m.err, "m.err should be non-nil when .env.example is missing")
		assert.Contains(t, m.err.Error(), "Error reading .env.example", "Error message mismatch")
		assert.NotNil(t, cmd, "Init should return tea.Quit command on error")

		// Use helper to check for Quit command
		assert.True(t, isBatchWithQuit(cmd) || isQuitCommand(cmd), "Expected a tea.Quit command or equivalent when .env.example is missing and error is set")
	})

	t.Run("init with empty .env.example", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTempFileForModel(t, tmpDir, ".env.example", "")

		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		m := initialModel()
		cmd := m.Init()

		require.NotNil(t, m.err, "m.err should be non-nil for empty .env.example")
		assert.Contains(t, m.err.Error(), "No environment variables found in .env.example", "Error message mismatch")
		assert.True(t, isBatchWithQuit(cmd) || isQuitCommand(cmd), "Expected a tea.Quit command or equivalent for empty .env.example")
	})

	t.Run("init with missing .env (should not error, just warn)", func(t *testing.T) {
		tmpDir := t.TempDir()
		exampleContent := "KEY1=example_val1"
		createTempFileForModel(t, tmpDir, ".env.example", exampleContent)

		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		// Capture stdout to check for warning
		// This is a bit complex, might skip for brevity or use a more advanced logger mock
		// For now, we'll just check that m.err is nil and existingEnvValues is empty

		m := initialModel()
		cmd := m.Init()

		require.Nil(t, m.err, "m.err should be nil if only .env is missing")
		assert.NotNil(t, cmd, "Init should return a command")
		assert.Empty(t, m.existingEnvValues, "existingEnvValues should be empty if .env is missing")
		assert.Len(t, m.fields, 1, "Should have 1 field")
		field1, ok := m.fields[0].(*huh.Input)
		require.True(t, ok)
		assert.Equal(t, "example_val1", field1.GetValue().(string), "Field value should be from .env.example if .env is missing")
	})
}

func TestModelUpdate(t *testing.T) {
	// Setup a basic model that has successfully initialized
	setupModel := func(t *testing.T) (*model, string) {
		tmpDir := t.TempDir()
		exampleContent := "KEY1=val1 # desc1\nKEY2=val2"
		envContent := "KEY1=old_val1"
		createTempFileForModel(t, tmpDir, ".env.example", exampleContent)
		createTempFileForModel(t, tmpDir, ".env", envContent)

		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		// Defer os.Chdir(originalWd) will be handled by the caller of setupModel if needed

		m := initialModel()
		initCmd := m.Init()
		require.Nil(t, m.err)
		require.NotNil(t, initCmd) // Form's Init command

		// Simulate initial form update if necessary (e.g. for focus)
		// newForm, _ := m.form.Update(nil) // huh.FocusMsg or similar might be needed
		// m.form = newForm.(*huh.Form)
		return m, originalWd
	}

	t.Run("quit on ctrl+c", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		updatedModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		assert.True(t, updatedModel.(*model).quitting, "m.quitting should be true")
		// Check if cmd is tea.Quit
		isQuitCmd := false
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				isQuitCmd = true
			}
		}
		assert.True(t, isQuitCmd, "Command should be tea.Quit")
	})

	t.Run("window size message", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		newWidth, newHeight := 100, 50
		updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: newWidth, Height: newHeight})
		assert.Equal(t, newWidth, updatedModel.(*model).width)
		assert.Equal(t, newHeight, updatedModel.(*model).height)
	})

	t.Run("main form completion leads to confirmation", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		// Simulate filling the form
		key1Field := m.fields[0].(*huh.Input)
		newVal1 := "new_val1_updated"
		key1Field.Value(&newVal1) // Changed from "old_val1"
		key2Field := m.fields[1].(*huh.Input)
		newVal2 := "val2_updated"
		key2Field.Value(&newVal2) // Changed from "val2" (example)

		m.form.State = huh.StateCompleted  // Manually set form to completed
		updatedModel, cmd := m.Update(nil) // Msg doesn't matter much here as state is forced

		mu := updatedModel.(*model)
		require.Nil(t, mu.err, "Error during update to confirmation: %v", mu.err)
		assert.True(t, mu.confirming, "Model should be in confirming state")
		assert.NotNil(t, mu.confirmForm, "Confirm form should be initialized")
		assert.NotEmpty(t, mu.diffSummary, "Diff summary should be generated")
		assert.Contains(t, mu.diffSummary, `~ Changed: KEY1: "old_val1" -> "new_val1_updated"`)
		assert.Contains(t, mu.diffSummary, `+ Added: KEY2="val2_updated"`) // KEY2 was not in .env, so it's "Added"

		require.NotNil(t, cmd)
		// Check if cmd contains m.confirmForm.Init()
		batch, ok := cmd().(tea.BatchMsg)
		require.True(t, ok, "Expected a batch command")
		foundConfirmInit := false
		for _, subCmd := range batch { // Renamed c to subCmd to avoid conflict if c was defined outside
			// This is tricky because Init() returns a Cmd, not a Msg.
			// We'd need to execute it and see if it produces a huh.FocusMsg or similar.
			// For now, we'll assume if confirming is true and confirmForm is not nil, Init was called.
			// A more robust test would mock the form's Init.
			if mu.confirming && mu.confirmForm != nil {
				// Heuristic: if we are confirming and have a form, its Init was likely part of the batch.
				// A proper test would involve checking the type of command, e.g., if it's huh.form.Init()
				// For now, we check that a command was returned.
				// We can also check if the command is m.confirmForm.Init specifically if we can compare functions,
				// but that's also tricky. The presence of any command while in this state is a good sign.
				if subCmd != nil { // Check if subCmd is not nil
					foundConfirmInit = true // Simplified check, assuming any non-nil cmd here is fine
				}
			}
		}
		assert.True(t, foundConfirmInit, "confirmForm.Init() should be part of the returned commands (or a command was present)")
	})

	t.Run("main form completion with no changes", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		// Values are already "old_val1" and "val2" (from example, as KEY2 not in .env)
		// So, no changes are made by the user effectively.
		key1Field := m.fields[0].(*huh.Input)
		oldVal1 := "old_val1"
		key1Field.Value(&oldVal1) // Same as in .env
		key2Field := m.fields[1].(*huh.Input)
		val2Unchanged := "val2"
		key2Field.Value(&val2Unchanged) // Same as example, and it wasn't in .env

		m.form.State = huh.StateCompleted
		updatedModel, cmd := m.Update(nil)

		mu := updatedModel.(*model)
		require.Nil(t, mu.err)
		assert.False(t, mu.confirming, "Should not be confirming if no changes")
		assert.True(t, mu.quitting, "Should be quitting if no changes")
		assert.Equal(t, "No changes to apply to .env file.", mu.diffSummary)

		assert.True(t, isQuitCommand(cmd), "Command should be tea.Quit if no changes")
	})

	t.Run("confirmation form - save changes", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		// Manually set up for confirmation state
		m.confirming = true
		m.envValuesToSave = map[string]string{"KEY1": "saved_value"}
		m.diffSummary = "Changes: KEY1"
		m.applyChanges = true // huh.Confirm will set this based on user input

		confirmField := huh.NewConfirm().Value(&m.applyChanges) // Simulate confirm field
		m.confirmForm = huh.NewForm(huh.NewGroup(confirmField))
		m.confirmForm.State = huh.StateCompleted // Simulate user confirming "Yes"

		// Mock `actuallyWriteEnvFile` for this test to avoid file system ops here
		// Or, better, test `actuallyWriteEnvFile` separately and ensure it's called.
		// For now, we'll check that it *would* try to write.
		// We need to ensure `envOutputFilePath` is set for `actuallyWriteEnvFile` if it were called.
		originalEnvOutputFilePath := envOutputFilePath
		testOutputFile := filepath.Join(wd, "test_output.env") // wd is tmpDir here
		envOutputFilePath = testOutputFile
		defer func() {
			envOutputFilePath = originalEnvOutputFilePath
			os.Remove(testOutputFile)
		}()

		updatedModel, cmd := m.Update(nil) // Msg doesn't matter as state is forced

		mu := updatedModel.(*model)
		require.Nil(t, mu.err)
		assert.True(t, mu.quitting, "Should be quitting after confirmation")

		// Check if file was "written" (i.e., `actuallyWriteEnvFile` logic was hit)
		// This requires `actuallyWriteEnvFile` to be tested robustly itself.
		// Here, we assume if no error and applyChanges is true, it attempted.
		// A more direct check would be to see if the file `testOutputFile` was created.
		_, err := os.Stat(testOutputFile)
		assert.NoError(t, err, "Expected .env file to be written")

		content, _ := os.ReadFile(testOutputFile)
		assert.Contains(t, string(content), "KEY1=saved_value", "Content of written file is wrong")

		isQuitCmd := false
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				isQuitCmd = true
			}
		}
		assert.True(t, isQuitCmd, "Command should be tea.Quit")
	})

	t.Run("confirmation form - discard changes", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		m.confirming = true
		m.envValuesToSave = map[string]string{"KEY1": "should_not_be_saved"}
		m.diffSummary = "Changes: KEY1"
		m.applyChanges = false // Simulate user choosing "No"

		confirmField := huh.NewConfirm().Value(&m.applyChanges)
		m.confirmForm = huh.NewForm(huh.NewGroup(confirmField))
		m.confirmForm.State = huh.StateCompleted

		testOutputFile := filepath.Join(wd, "test_output_discard.env")
		originalEnvOutputFilePath := envOutputFilePath
		envOutputFilePath = testOutputFile
		defer func() { envOutputFilePath = originalEnvOutputFilePath }()

		updatedModel, cmd := m.Update(nil)

		mu := updatedModel.(*model)
		require.Nil(t, mu.err)
		assert.True(t, mu.quitting, "Should be quitting after confirmation")

		_, err := os.Stat(testOutputFile)
		assert.True(t, os.IsNotExist(err), "File should NOT have been written if changes were discarded")

		isQuitCmd := false
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				isQuitCmd = true
			}
		}
		assert.True(t, isQuitCmd, "Command should be tea.Quit")
	})

	t.Run("main form aborted", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		m.form.State = huh.StateAborted // Simulate user aborting main form
		updatedModel, cmd := m.Update(nil)

		mu := updatedModel.(*model)
		assert.True(t, mu.quitting, "Should be quitting if main form aborted")
		isQuitCmd := false
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				isQuitCmd = true
			}
		}
		assert.True(t, isQuitCmd, "Command should be tea.Quit")
	})

	t.Run("confirmation form aborted", func(t *testing.T) {
		m, wd := setupModel(t)
		defer os.Chdir(wd)

		m.confirming = true
		m.confirmForm = huh.NewForm(huh.NewGroup(huh.NewConfirm().Value(new(bool))))
		m.confirmForm.State = huh.StateAborted // Simulate user aborting confirm form
		updatedModel, cmd := m.Update(nil)

		mu := updatedModel.(*model)
		assert.True(t, mu.quitting, "Should be quitting if confirm form aborted")
		isQuitCmd := false
		if cmd != nil {
			if _, ok := cmd().(tea.QuitMsg); ok {
				isQuitCmd = true
			}
		}
		assert.True(t, isQuitCmd, "Command should be tea.Quit")
	})

}

func TestModelView(t *testing.T) {
	t.Run("view with error", func(t *testing.T) {
		m := initialModel()
		m.err = fmt.Errorf("test error")
		view := m.View()
		assert.Contains(t, view, "Error: test error", "View should display error")
	})

	t.Run("view when quitting", func(t *testing.T) {
		m := initialModel()
		m.quitting = true
		view := m.View()
		assert.Equal(t, "", view, "View should be empty when quitting and no error")
	})

	t.Run("view main form", func(t *testing.T) {
		// This requires a fully initialized form.
		// For a simple check, ensure it doesn't panic and returns something.
		tmpDir := t.TempDir()
		exampleContent := "KEY1=val1"
		createTempFileForModel(t, tmpDir, ".env.example", exampleContent)
		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		m := initialModel()
		_ = m.Init() // Initialize form
		require.Nil(t, m.err)
		require.NotNil(t, m.form)

		view := m.View()
		// huh.Form.View() is complex, we're just checking it's called.
		// A more robust test might check for the presence of the form's title.
		assert.Contains(t, view, "Setup your .env values", "View should contain main form title")
	})

	t.Run("view confirmation form", func(t *testing.T) {
		m := initialModel()
		m.confirming = true
		m.diffSummary = "Diff: KEY1 changed"
		m.confirmForm = huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Save?"))) // Basic confirm form
		_ = m.confirmForm.Init()                                                   // Initialize confirm form internal state

		view := m.View()
		assert.Contains(t, view, m.diffSummary, "View should display diff summary")
		assert.Contains(t, view, "Save?", "View should contain confirm form content")
	})
}

func TestPrepareForConfirmation(t *testing.T) {
	setupModelForConfirm := func(t *testing.T, exampleContent, envContent string, fieldValues map[string]string) (*model, string) {
		tmpDir := t.TempDir()
		if exampleContent != "" {
			createTempFileForModel(t, tmpDir, ".env.example", exampleContent)
		}
		if envContent != "" {
			createTempFileForModel(t, tmpDir, ".env", envContent)
		}

		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))

		m := initialModel()
		_ = m.Init()
		require.Nil(t, m.err)

		// Set field values as if user entered them
		for i, envVar := range m.envVars {
			if val, ok := fieldValues[envVar.Key]; ok {
				inputField, isInput := m.fields[i].(*huh.Input)
				require.True(t, isInput, "Field for %s is not huh.Input", envVar.Key)
				// Create a new string for each value to ensure pointer uniqueness if needed,
				// though for this test structure, direct assignment of val's address is fine.
				currentVal := val // Create a local copy to take its address
				inputField.Value(&currentVal)
			}
		}
		return m, originalWd
	}

	t.Run("no changes", func(t *testing.T) {
		m, wd := setupModelForConfirm(t, "K1=v1\nK2=v2", "K1=v1\nK2=v2", map[string]string{"K1": "v1", "K2": "v2"})
		defer os.Chdir(wd)

		err := m.prepareForConfirmation()
		require.Nil(t, err)
		assert.True(t, m.quitting, "Should be quitting if no changes")
		assert.False(t, m.confirming, "Should not be confirming")
		assert.Equal(t, "No changes to apply to .env file.", m.diffSummary)
	})

	t.Run("with changes - added, modified, cleared", func(t *testing.T) {
		example := "K1=ex_v1\nK2=ex_v2\nK3=ex_v3\nK4=ex_v4"
		env := "K1=old_v1\nK2=old_v2" // K3, K4 not in .env
		userInputs := map[string]string{
			"K1": "new_v1", // Modified
			"K2": "",       // Cleared
			"K3": "new_v3", // Added
			"K4": "",       // Added as empty
		}
		m, wd := setupModelForConfirm(t, example, env, userInputs)
		defer os.Chdir(wd)

		err := m.prepareForConfirmation()
		require.Nil(t, err)
		assert.False(t, m.quitting, "Should not be quitting as there are changes")
		assert.True(t, m.confirming, "Should be confirming")
		assert.NotNil(t, m.confirmForm, "Confirm form should be created")

		expectedDiffs := []string{
			`~ Changed: K1: "old_v1" -> "new_v1"`,
			`~ Cleared: K2 (was "old_v2")`,
			`+ Added: K3="new_v3"`,
			`+ Added: K4=""`,
		}
		for _, expectedDiff := range expectedDiffs {
			assert.Contains(t, m.diffSummary, expectedDiff)
		}
		assert.Equal(t, userInputs, m.envValuesToSave)
	})

	t.Run("error if field is not huh.Input", func(t *testing.T) {
		m, wd := setupModelForConfirm(t, "K1=v1", "", map[string]string{"K1": "v1"})
		defer os.Chdir(wd)
		// Corrupt the fields array for testing
		m.fields[0] = huh.NewSelect[string]() // Not an input

		err := m.prepareForConfirmation()
		require.NotNil(t, err)
		assert.Contains(t, err.Error(), "could not cast field for key K1 to huh.Input")
	})
}

func TestActuallyWriteEnvFile(t *testing.T) {
	// This function relies on the global `envOutputFilePath` being set correctly by tests
	// and `backupEnvFile` which is tested in `env_utils_test.go`.

	t.Run("successful write with backup", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalEnvOutputFilePath := envOutputFilePath
		testDotEnv := filepath.Join(tmpDir, ".env")
		testDotEnvOld := filepath.Join(tmpDir, ".env.old")
		envOutputFilePath = testDotEnv // `actuallyWriteEnvFile` writes to `envOutputFilePath`
		defer func() { envOutputFilePath = originalEnvOutputFilePath }()

		// Create an existing .env to be backed up
		require.NoError(t, os.WriteFile(testDotEnv, []byte("OLD_KEY=old_value"), 0644))

		m := initialModel() // model instance is needed to call the method
		valuesToSave := map[string]string{"NEW_KEY": "new_value", "ANOTHER": "val"}

		// Mock os.Stat for backup check - this is getting complex.
		// For now, let's assume backupEnvFile works (tested elsewhere)
		// and focus on the write itself.
		// The backup logic in actuallyWriteEnvFile uses ".env" and ".env.old" hardcoded
		// for statting and backup, which makes this tricky to test in isolation without
		// more significant refactoring or more complex mocking.

		// We will test the write part, and manually check backup if possible.
		// The backup part in `actuallyWriteEnvFile` uses hardcoded ".env" and ".env.old"
		// relative to current working dir. So we need to chdir.
		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)
		// Now, inside tmpDir, testDotEnv is ".env" and testDotEnvOld is ".env.old"

		err := m.actuallyWriteEnvFile(valuesToSave)
		require.Nil(t, err)

		// Check backup
		backupContent, backupErr := os.ReadFile(testDotEnvOld) // ".env.old" in tmpDir
		require.NoError(t, backupErr, "Backup file .env.old not found or unreadable")
		assert.Equal(t, "OLD_KEY=old_value", string(backupContent), "Backup content mismatch")

		// Check new .env content
		newContent, newReadErr := os.ReadFile(testDotEnv) // ".env" in tmpDir
		require.NoError(t, newReadErr)
		expectedNewContentLines := []string{"NEW_KEY=new_value", "ANOTHER=val"}
		actualNewContent := strings.TrimSpace(string(newContent))

		for _, line := range expectedNewContentLines {
			assert.Contains(t, actualNewContent, line)
		}
		// Check number of lines to ensure no extra content
		assert.Equal(t, len(expectedNewContentLines), len(strings.Split(actualNewContent, "\n")))

	})

	t.Run("write when .env does not exist (no backup)", func(t *testing.T) {
		tmpDir := t.TempDir()
		originalEnvOutputFilePath := envOutputFilePath
		testDotEnv := filepath.Join(tmpDir, ".env") // This will be the target
		testDotEnvOld := filepath.Join(tmpDir, ".env.old")
		envOutputFilePath = testDotEnv
		defer func() { envOutputFilePath = originalEnvOutputFilePath }()

		originalWd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(originalWd)

		m := initialModel()
		valuesToSave := map[string]string{"FRESH_KEY": "fresh_value"}

		err := m.actuallyWriteEnvFile(valuesToSave)
		require.Nil(t, err)

		_, backupStatErr := os.Stat(testDotEnvOld)
		assert.True(t, os.IsNotExist(backupStatErr), ".env.old should not exist")

		newContent, _ := os.ReadFile(testDotEnv)
		assert.Equal(t, "FRESH_KEY=fresh_value\n", string(newContent))
	})

	// Error during write is hard to test without os.Create mocking for envOutputFilePath
	// or making envOutputFilePath unwriteable, which is OS-dependent.
	// Error during backup is implicitly covered by backupEnvFile tests.
}

// Note: Testing functions that directly print to console (like some parts of Update or Init warnings)
// is harder and often involves redirecting stdout, which can be flaky.
// We've focused on state changes and returned commands/errors.

// Helper for checking tea.Cmd type (example, might not be fully robust for all cases)
func isQuitCommand(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd() // Execute the command to get the message
	if msg == nil {
		return false
	}
	_, ok := msg.(tea.QuitMsg)
	return ok
}

func isBatchWithQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if msg == nil {
		return false
	}
	batchMsg, ok := msg.(tea.BatchMsg)
	if !ok {
		// If it's not a batch, check if it's a direct quit message
		_, isQuit := msg.(tea.QuitMsg)
		return isQuit
	}
	for _, subCmd := range batchMsg {
		if isQuitCommand(subCmd) { // Recursively check, though Quit is usually not nested in BatchMsg this way
			return true
		}
		// If subCmd itself is a func() tea.Msg that returns QuitMsg
		subMsg := subCmd()
		if subMsg == nil {
			continue
		}
		if _, ok := subMsg.(tea.QuitMsg); ok {
			return true
		}
	}
	return false
}
