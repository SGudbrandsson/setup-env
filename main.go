package main

import (
	"bufio"
	"fmt" // Added io
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

// EnvVar holds a key and its description from the .env.example file
type EnvVar struct {
	Key          string
	Description  string
	ExampleValue string // Value from .env.example
}

func main() {
	// --- 1. Read variable names from .env.example ---
	exampleFile, err := os.Open(".env.example")
	if err != nil {
		fmt.Println("Error: .env.example file not found in the current directory.")
		fmt.Println("Please create one to use as a template.")
		os.Exit(1)
	}
	defer exampleFile.Close()

	var envVars []EnvVar // Changed from envKeys []string
	scanner := bufio.NewScanner(exampleFile)
	for scanner.Scan() {
		line := scanner.Text()

		lineForParsing := strings.TrimSpace(line)
		// Skip empty lines or lines that are purely comments
		if lineForParsing == "" || strings.HasPrefix(lineForParsing, "#") {
			continue
		}

		var key, description, exampleValue string // Added exampleValue
		keyValuePart := lineForParsing

		commentIdx := strings.Index(lineForParsing, "#")
		if commentIdx != -1 {
			description = strings.TrimSpace(lineForParsing[commentIdx+1:])
			keyValuePart = strings.TrimSpace(lineForParsing[:commentIdx])
		}

		// Now, parse the key and example value from keyValuePart
		parts := strings.SplitN(keyValuePart, "=", 2)
		if len(parts) > 0 {
			key = strings.TrimSpace(parts[0])
			// Extract and unquote the example value if present
			if len(parts) == 2 {
				valStr := strings.TrimSpace(parts[1])
				if len(valStr) >= 2 && valStr[0] == '"' && valStr[len(valStr)-1] == '"' {
					unquotedValue := valStr[1 : len(valStr)-1]
					// Basic unescaping for .env.example values
					// Order of replacement matters: backslashes first.
					unquotedValue = strings.ReplaceAll(unquotedValue, `\\`, `\`)
					unquotedValue = strings.ReplaceAll(unquotedValue, `\"`, `"`)
					// Add other common unescapes if necessary (e.g., \n, \r)
					// unquotedValue = strings.ReplaceAll(unquotedValue, `\n`, "\n")
					// unquotedValue = strings.ReplaceAll(unquotedValue, `\r`, "\r")
					exampleValue = unquotedValue
				} else if valStr != "" { // Only assign if not empty and not just quotes
					exampleValue = valStr
				}
			}

			if key != "" {
				envVars = append(envVars, EnvVar{Key: key, Description: description, ExampleValue: exampleValue})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading .env.example: %v\n", err)
		os.Exit(1)
	}

	if len(envVars) == 0 { // Changed from envKeys
		fmt.Println("No environment variables found in .env.example.")
		os.Exit(0)
	}

	// --- 1.5. Read existing .env file (if any) to prefill values ---
	existingEnvValues, err := readExistingEnvFile(".env") // err is declared here
	if err != nil {
		// Log error but continue; the form will just not be prefilled.
		// Ensure existingEnvValues is initialized to avoid nil map panic on lookup.
		fmt.Printf("Warning: could not read existing .env file to prefill: %v\n", err)
		existingEnvValues = make(map[string]string)
	}

	// --- 2. Create huh form fields ---
	envValues := make(map[string]string) // This map will store final values from the form
	var fields []huh.Field

	for _, envVar := range envVars { // Changed from envKeys
		localKey := envVar.Key // Use envVar.Key

		// Get the existing value for this key from .env, default to empty string if not found.
		initialValue, valueExistsInEnv := existingEnvValues[localKey]

		// If the key is not in .env, or its value is empty,
		// try to use the ExampleValue from .env.example.
		if !valueExistsInEnv || initialValue == "" {
			if envVar.ExampleValue != "" {
				initialValue = envVar.ExampleValue
			}
		}

		// huh.Input().Value() expects a pointer to a string.
		// We need to create a new string variable for each field to hold its value.
		fieldValuePtr := new(string)
		*fieldValuePtr = initialValue // Set the initial value for the form field.

		inputField := huh.NewInput().
			Title(localKey).
			Value(fieldValuePtr) // Pass the pointer to the dedicated string.

		if envVar.Description != "" {
			inputField = inputField.Description(envVar.Description)
		}

		fields = append(fields, inputField)
		// Values will be retrieved after the form runs to get the *final* (potentially edited) values.
	}

	// --- Create a custom keymap to include Escape for quitting ---
	customKeyMap := huh.NewDefaultKeyMap() // Corrected
	customKeyMap.Quit = key.NewBinding(
		key.WithKeys("esc", "ctrl+c"),
		key.WithHelp("esc/ctrl+c", "quit"),
	)

	// Add Up/Down arrow keys for Input field navigation
	// Default keys for Input.Next are "enter", "tab"
	// Default keys for Input.Prev are "shift+tab"
	customKeyMap.Input.Next = key.NewBinding(
		key.WithKeys("enter", "tab", "down"), // Added "down"
		key.WithHelp("enter/tab/↓", "next"),
	)
	customKeyMap.Input.Prev = key.NewBinding(
		key.WithKeys("shift+tab", "up"), // Added "up"
		key.WithHelp("shift+tab/↑", "prev"),
	)

	form := huh.NewForm(
		huh.NewGroup(fields...).
			Title("Setup your .env values"), // Changed title
	).WithTheme(huh.ThemeCharm()).WithKeyMap(customKeyMap) // Optional: using a theme

	// --- 3. Run the form ---
	err = form.Run()
	if err != nil {
		if err == huh.ErrUserAborted {
			fmt.Println("\nOperation cancelled by user.")
			os.Exit(0)
		}
		fmt.Printf("Error running form: %v\n", err)
		os.Exit(1)
	}

	// --- 4. Collect values from the form ---
	// The values are now in the pointers we passed to NewInput().Value()
	// We need to associate them back with the keys.
	for i, envVar := range envVars { // Changed from envKeys
		inputField, ok := fields[i].(*huh.Input)
		if !ok {
			// Use envVar.Key for the error message
			fmt.Printf("Error: could not cast field for key %s to huh.Input\n", envVar.Key)
			continue // Or handle error more gracefully
		}
		val := inputField.GetValue().(string)
		envValues[envVar.Key] = val // Use envVar.Key
	}

	// --- 4.5. Prepare and confirm changes ---
	var diffLines []string
	changed := false

	// Iterate over envVars to ensure order matches form and .env.example
	for _, envVar := range envVars { // Changed from envKeys
		key := envVar.Key // Get the key from envVar
		oldValue, oldExists := existingEnvValues[key]
		newValue := envValues[key] // Populated from form field pointers

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
		} else if !oldExists && newValue == "" { // New variable, explicitly set to empty
			diffLines = append(diffLines, fmt.Sprintf("+ Added: %s=\"\"", key))
			changed = true
		}
		// If oldExists && newValue == oldValue, it's unchanged, so no diff line.
	}

	if !changed {
		fmt.Println("\nNo changes to apply to .env file.")
	} else {
		fmt.Println("\nProposed changes for .env:")
		for _, line := range diffLines {
			fmt.Println(line)
		}
		fmt.Println() // Extra line for spacing

		confirmApply := true
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Save these changes?").
					Affirmative("Save to .env").
					Negative("Discard changes").
					Value(&confirmApply),
			).Title("Confirmation"),
		).WithTheme(huh.ThemeCharm()).WithKeyMap(customKeyMap) // Use the same theme for the confirmation dialog

		errConfirm := confirmForm.Run()
		if errConfirm != nil {
			if errConfirm == huh.ErrUserAborted {
				fmt.Println("\nSave operation cancelled by user.")
				os.Exit(0)
			}
			fmt.Printf("Error running confirmation form: %v\n", errConfirm)
			os.Exit(1)
		}

		if confirmApply {
			// --- Attempt to backup .env before writing ---
			// Check if .env exists and is a file.
			info, statErr := os.Stat(".env")
			if statErr == nil {
				if !info.IsDir() { // Ensure it's a file, not a directory
					fmt.Println("\nBacking up existing .env to .env.old...")
					backupErr := backupEnvFile(".env", ".env.old")
					if backupErr != nil {
						fmt.Printf("Warning: Failed to backup .env: %v\n", backupErr)
						// Potentially ask user if they want to proceed without backup
						// For now, we proceed with a warning.
					} else {
						fmt.Println("Successfully backed up .env to .env.old.")
					}
				} else {
					fmt.Printf("Warning: .env exists but is a directory. Skipping backup.\n")
				}
			} else if !os.IsNotExist(statErr) {
				// An error other than "file does not exist" occurred.
				fmt.Printf("Warning: Error checking .env for backup (will attempt to write anyway): %v\n", statErr)
			}
			// If os.IsNotExist(statErr) is true, .env doesn't exist, so no backup is needed.

			// --- 5. Write the .env file ---
			err = writeEnvFile(envValues) // err is from the main function's scope
			if err != nil {
				fmt.Printf("Error writing .env file: %v\n", err)
				// os.Exit(1) // Optionally exit on write error
			} else {
				fmt.Println("\n✅ Successfully updated the .env file!")
			}
		} else {
			fmt.Println("\nChanges discarded by user.")
			// os.Exit(0) // Optionally exit if discarded
		}
	}
}

// writeEnvFile writes the collected environment variables to a .env file
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
			// Characters that necessitate quoting.
			// Includes common special characters and whitespace.
			if strings.ContainsAny(value, " #=\"$\\`\n\r") {
				needsQuoting = true
			}
		}

		var outputValue string
		if needsQuoting {
			// Perform escaping for characters within quotes.
			// Order of replacement matters: backslashes first.
			escaped := strings.ReplaceAll(value, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, "\n", `\n`) // Replace literal newline char with \n string
			escaped = strings.ReplaceAll(escaped, "\r", `\r`) // Replace literal CR char with \r string
			// Note: $ is not escaped here as its expansion behavior varies among .env parsers
			// and often it's treated literally within double quotes if not part of a ${VAR} syntax.
			outputValue = `"` + escaped + `"`
		} else {
			// Value is simple enough to not require quotes or internal escaping.
			outputValue = value
		}

		if _, err := envFile.WriteString(fmt.Sprintf("%s=%s\n", key, outputValue)); err != nil {
			return err
		}
	}
	return nil
}

// readExistingEnvFile reads an existing .env file and returns its key-value pairs.
// It returns an empty map if the file doesn't exist or an error occurs during reading.
// Values are unquoted and basic escape sequences (\", \\) are processed.
func readExistingEnvFile(filePath string) (map[string]string, error) {
	values := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil // File doesn't exist, return empty map, no error
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

			// Unquote if the value is double-quoted
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				unquotedValue := value[1 : len(value)-1]
				// Unescape common sequences. Order can be important for complex cases.
				unquotedValue = strings.ReplaceAll(unquotedValue, `\\`, `\`) // \\ -> \
				unquotedValue = strings.ReplaceAll(unquotedValue, `\"`, `"`) // \" -> "
				// Add other common unescapes if necessary (e.g., \n, \t, \r)
				// unquotedValue = strings.ReplaceAll(unquotedValue, `\n`, "\n")
				// unquotedValue = strings.ReplaceAll(unquotedValue, `\r`, "\r")
				value = unquotedValue
			}
			// Note: This simple parser doesn't handle single-quoted values or more complex .env syntax.
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		// Return successfully parsed values up to the point of error, along with the error itself.
		return values, fmt.Errorf("error reading %s during scan: %w", filePath, err)
	}
	return values, nil
}

// backupEnvFile copies the source file to the destination file.
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

	// Ensure the destination file is synced to disk.
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file %s: %w", dstPath, err)
	}

	return nil
}
