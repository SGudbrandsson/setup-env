package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var envOutputFilePath = ".env" // For testability

// readEnvVarsFromFile reads .env.example and parses it into EnvVar structs
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

// writeEnvFile creates or overwrites the .env file with the given values
func writeEnvFile(values map[string]string) error {
	envFile, err := os.Create(envOutputFilePath)
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

// readExistingEnvFile reads an existing .env file and returns its key-value pairs
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
			// Handle quoted values
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				unquotedValue := value[1 : len(value)-1]
				// Basic unescaping for \ and "
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

// backupEnvFile creates a backup of a file
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
