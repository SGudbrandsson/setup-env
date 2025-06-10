package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadEnvVarsFromFile(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedVars   []EnvVar
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid file with various entries",
			fileContent: `
KEY1=value1 # Description for key1
KEY2="quoted value2" # Description for key2
KEY3= # Empty value with description
KEY_NO_DESC=nodefault
KEY_ONLY
# This is a comment
KEY_WITH_ESCAPED_QUOTE="value with \"escaped\" quote" # desc
KEY_WITH_BACKSLASH="value with \\ backslash" # desc
`,
			expectedVars: []EnvVar{
				{Key: "KEY1", Description: "Description for key1", ExampleValue: "value1"},
				{Key: "KEY2", Description: "Description for key2", ExampleValue: "quoted value2"},
				{Key: "KEY3", Description: "Empty value with description", ExampleValue: ""},
				{Key: "KEY_NO_DESC", Description: "", ExampleValue: "nodefault"},
				{Key: "KEY_ONLY", Description: "", ExampleValue: ""},
				{Key: "KEY_WITH_ESCAPED_QUOTE", Description: "desc", ExampleValue: "value with \"escaped\" quote"},
				{Key: "KEY_WITH_BACKSLASH", Description: "desc", ExampleValue: "value with \\ backslash"},
			},
			expectError: false,
		},
		{
			name:         "empty file",
			fileContent:  "",
			expectedVars: []EnvVar{},
			expectError:  false,
		},
		{
			name: "file with only comments and blank lines",
			fileContent: `
# Comment 1
# Comment 2

`,
			expectedVars: []EnvVar{},
			expectError:  false,
		},
		{
			name:           "non-existent file",
			fileContent:    "", // Content doesn't matter, path will be invalid
			expectedVars:   nil,
			expectError:    true,
			expectedErrMsg: "no such file or directory", // OS-dependent part
		},
		{
			name: "key with no value and no equals",
			fileContent: `
KEY_NO_VALUE_NO_EQUALS # description
KEY_WITH_EQUALS=
`,
			expectedVars: []EnvVar{
				{Key: "KEY_NO_VALUE_NO_EQUALS", Description: "description", ExampleValue: ""},
				{Key: "KEY_WITH_EQUALS", Description: "", ExampleValue: ""},
			},
			expectError: false,
		},
		{
			name:        "value with internal quotes not at ends",
			fileContent: `KEY_INTERNAL_QUOTE=abc"def # description`,
			expectedVars: []EnvVar{
				{Key: "KEY_INTERNAL_QUOTE", Description: "description", ExampleValue: `abc"def`},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			var err error

			if tt.name == "non-existent file" {
				filePath = filepath.Join(t.TempDir(), "non_existent_file.env")
			} else {
				tmpFile, err := os.CreateTemp(t.TempDir(), "test_env_example_*.env")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				filePath = tmpFile.Name()
				if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
					tmpFile.Close()
					t.Fatalf("Failed to write to temp file: %v", err)
				}
				tmpFile.Close()
			}

			vars, err := readEnvVarsFromFile(filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain '%s', but got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
				if !reflect.DeepEqual(vars, tt.expectedVars) {
					t.Errorf("Expected vars:\n%v\nGot vars:\n%v", tt.expectedVars, vars)
				}
			}

			if tt.name != "non-existent file" {
				os.Remove(filePath) // Clean up
			}
		})
	}
}

func TestWriteEnvFile(t *testing.T) {
	tests := []struct {
		name           string
		values         map[string]string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "empty map",
			values:         map[string]string{},
			expectedOutput: "",
			expectError:    false,
		},
		{
			name: "simple key-values",
			values: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			// Order is not guaranteed for maps, so check for presence of lines
			expectedOutput: "KEY1=value1\nKEY2=value2\n",
			expectError:    false,
		},
		{
			name: "values needing quoting",
			values: map[string]string{
				"KEY_SPACE": "value with space",
				"KEY_HASH":  "value#hash",
				"KEY_EMPTY": "",
				"KEY_QUOTE": `value"quote`,
				"KEY_MULTI": "line1\nline2",
			},
			expectedOutput: `KEY_SPACE="value with space"` + "\n" +
				`KEY_HASH="value#hash"` + "\n" +
				`KEY_EMPTY=""` + "\n" +
				`KEY_QUOTE="value\"quote"` + "\n" +
				`KEY_MULTI="line1\nline2"` + "\n",
			expectError: false,
		},
		{
			name: "value with backslash",
			values: map[string]string{
				"KEY_BACKSLASH": `value\backslash`,
			},
			expectedOutput: `KEY_BACKSLASH="value\\backslash"` + "\n",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			// Define testFilePath here
			testFilePath := filepath.Join(tmpDir, "test_output.env")

			// Override the global output path for this test
			originalEnvOutputFilePath := envOutputFilePath
			envOutputFilePath = testFilePath
			defer func() { envOutputFilePath = originalEnvOutputFilePath }()

			err := writeEnvFile(tt.values)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
				// Use the correctly defined testFilePath
				content, readErr := os.ReadFile(testFilePath)
				if readErr != nil {
					t.Fatalf("Failed to read temp .env file: %v", readErr)
				}

				// Normalize newlines for comparison and split into lines
				expectedLines := strings.Split(strings.ReplaceAll(tt.expectedOutput, "\r\n", "\n"), "\n")
				actualLines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")

				// Remove empty trailing lines that might result from split
				if len(expectedLines) > 0 && expectedLines[len(expectedLines)-1] == "" {
					expectedLines = expectedLines[:len(expectedLines)-1]
				}
				if len(actualLines) > 0 && actualLines[len(actualLines)-1] == "" {
					actualLines = actualLines[:len(actualLines)-1]
				}

				if len(expectedLines) != len(actualLines) {
					t.Errorf("Expected %d lines, got %d lines.\nExpected:\n%s\nGot:\n%s", len(expectedLines), len(actualLines), tt.expectedOutput, string(content))
				} else {
					expectedSet := make(map[string]bool)
					for _, line := range expectedLines {
						expectedSet[line] = true
					}
					for _, line := range actualLines {
						if _, ok := expectedSet[line]; !ok {
							t.Errorf("Unexpected line or incorrect format in output.\nExpected lines (any order):\n%v\nGot lines:\n%v\nFull expected output:\n%s\nFull actual output:\n%s", expectedLines, actualLines, tt.expectedOutput, string(content))
							break
						}
						delete(expectedSet, line) // Mark as found
					}
					if len(expectedSet) > 0 {
						t.Errorf("Missing expected lines: %v\nFull expected output:\n%s\nFull actual output:\n%s", expectedSet, tt.expectedOutput, string(content))
					}
				}
			}
		})
	}
}

func TestReadExistingEnvFile(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedValues map[string]string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid .env file",
			fileContent: `
KEY1=value1
KEY2="quoted value"
# Comment
KEY3=value3
KEY_EMPTY=""
KEY_WITH_ESCAPED_QUOTE="value with \"escaped\" quote"
KEY_WITH_BACKSLASH="value with \\ backslash"
`,
			expectedValues: map[string]string{
				"KEY1":                   "value1",
				"KEY2":                   "quoted value",
				"KEY3":                   "value3",
				"KEY_EMPTY":              "",
				"KEY_WITH_ESCAPED_QUOTE": "value with \"escaped\" quote",
				"KEY_WITH_BACKSLASH":     "value with \\ backslash",
			},
			expectError: false,
		},
		{
			name:           "empty file",
			fileContent:    "",
			expectedValues: map[string]string{},
			expectError:    false,
		},
		{
			name:           "non-existent file",
			fileContent:    "", // Content doesn't matter
			expectedValues: map[string]string{},
			expectError:    false, // Should return empty map, no error
		},
		{
			name: "file with only comments",
			fileContent: `
# KEY1=value1
# KEY2=value2
`,
			expectedValues: map[string]string{},
			expectError:    false,
		},
		{
			name:           "malformed line (no equals)",
			fileContent:    `KEY_NO_EQUALS`,
			expectedValues: map[string]string{}, // Should skip malformed lines
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filePath string
			var err error

			tmpDir := t.TempDir()
			if tt.name == "non-existent file" {
				filePath = filepath.Join(tmpDir, "non_existent.env")
			} else {
				tmpFile, err := os.CreateTemp(tmpDir, "test_existing_env_*.env")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				filePath = tmpFile.Name()
				if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
					tmpFile.Close()
					t.Fatalf("Failed to write to temp file: %v", err)
				}
				tmpFile.Close()
			}

			values, err := readExistingEnvFile(filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain '%s', but got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
				if !reflect.DeepEqual(values, tt.expectedValues) {
					t.Errorf("Expected values:\n%v\nGot values:\n%v", tt.expectedValues, values)
				}
			}
			if tt.name != "non-existent file" {
				os.Remove(filePath)
			}
		})
	}
}

func TestBackupEnvFile(t *testing.T) {
	tests := []struct {
		name           string
		srcContent     string
		srcExists      bool
		dstShouldExist bool // After successful backup
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "successful backup",
			srcContent:     "KEY=value\nANOTHER=val",
			srcExists:      true,
			dstShouldExist: true,
			expectError:    false,
		},
		{
			name:           "source file does not exist",
			srcContent:     "",
			srcExists:      false,
			dstShouldExist: false,
			expectError:    true,
			expectedErrMsg: "failed to open source file",
		},
		// Testing "cannot create destination file" is harder without mocking OS calls
		// or manipulating permissions in a portable way. We'll focus on the common cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "test_source.env")
			dstPath := filepath.Join(tmpDir, "test_dest.env.backup")

			if tt.srcExists {
				err := os.WriteFile(srcPath, []byte(tt.srcContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create source file: %v", err)
				}
			}

			err := backupEnvFile(srcPath, dstPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, but got nil")
				} else if tt.expectedErrMsg != "" && !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("Expected error message to contain '%s', but got '%s'", tt.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error, but got: %v", err)
				}
			}

			if tt.dstShouldExist {
				dstContent, readErr := os.ReadFile(dstPath)
				if readErr != nil {
					t.Fatalf("Failed to read destination backup file: %v", readErr)
				}
				if string(dstContent) != tt.srcContent {
					t.Errorf("Destination file content mismatch. Expected:\n%s\nGot:\n%s", tt.srcContent, string(dstContent))
				}
			} else {
				if _, statErr := os.Stat(dstPath); !os.IsNotExist(statErr) {
					t.Errorf("Destination file should not exist, but it does at %s", dstPath)
				}
			}
		})
	}
}

// Helper to create a temporary file with content for testing
func createTempFile(t *testing.T, dir string, pattern string, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	return tmpFile.Name()
}
