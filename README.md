# Environment Setup CLI

This command-line application helps you set up your project's environment variables by interactively creating or updating a `.env` file based on a `.env.example` template. It uses an interactive form to guide you through setting each variable.

## Features

*   Reads variable definitions, example values, and descriptions from `.env.example`.
*   Prefills values from an existing `.env` file if present.
*   Uses example values from `.env.example` as defaults if a variable is not in `.env` or is empty.
*   Provides an interactive terminal UI to input or confirm values for each environment variable.
*   Displays a summary of proposed changes (additions, modifications, cleared values) before writing to the `.env` file.
*   Requires user confirmation before applying changes.
*   Automatically backs up an existing `.env` file to `.env.old` before writing new changes.
*   Supports quoted values and basic escape sequences in `.env` and `.env.example` files.

## Screenshots

![Modifying an .env file](https://github.com/user-attachments/assets/acadd9cb-e446-4525-a600-0c7ce89062b3)

## Prerequisites

*   Go (version 1.18 or higher, due to generics usage in dependencies like `huh`).
*   A `.env.example` file in the root directory of your project.

## Installation

If you have `go` installed on your computer, you can just fetch the latest version like this:
```bash
go install github.com/SGudbrandsson/setup-env@latest
```

Alternatively, you can [download the binary for your platform in the releases area](https://github.com/SGudbrandsson/setup-env/releases)

## How to Use

### 1. Prepare `.env.example`

Create a `.env.example` file in the root of your project. This file serves as the template for your `.env` file. The application will parse this file to determine which variables to ask for.

**Format:**

*   Each line can define a variable, optionally with an example value and a description.
*   `KEY=VALUE`
*   `KEY="VALUE WITH SPACES"`
*   `KEY=` (for an empty default value)
*   Descriptions are taken from comments (`#`) on the same line as the variable.
    *   Example: `DB_HOST=localhost # The hostname of your database server`
*   Lines that are purely comments (start with `#` at the beginning of the line) or empty lines are ignored.

Example [` .env.example `](.env.example:1):
```env
# .env.example

APP_NAME="My Awesome App" # Application name
DB_HOST=127.0.0.1 # Database host
DB_USER=admin # Database user
DB_PORT=5432 # Database port
DB_PASSWORD=secretpassw0rd # Database password

# This variable has no default value in the example but will be prompted for
API_KEY=
```

### 2. Build the Application

Navigate to the project directory in your terminal and run:
```bash
go build
```
This will create an executable file (e.g., `setup-env` on Linux/macOS or `setup-env.exe` on Windows, named after the project directory).

### 3. Run the Application

Execute the built application from your terminal:
```bash
./setup-env
```
(or `setup-env.exe` on Windows)

The application will:
1.  Read your [` .env.example `](.env.example:1).
2.  Check for an existing `.env` file and load its values.
3.  Present an interactive form, prefilling values from `.env` or `.env.example`.
    *   Use `Up/Down` arrow keys, `Tab`, and `Shift+Tab` to navigate fields.
    *   Press `Enter` to confirm a field and move to the next.
    *   Press `Esc` or `Ctrl+C` to quit at any time.
4.  After you complete the form, it will display a summary of changes.
5.  Ask for confirmation to save the changes to the `.env` file.
    *   If you confirm, it will back up any existing `.env` to `.env.old` and then write the new `.env` file.
    *   If you discard, no changes will be made.

## How It Works

The application performs the following steps (as seen in [` main.go `](main.go:1)):

1.  **Reads [` .env.example `](.env.example:1):**
    *   Parses each line to extract the variable key, its example value (if any), and its description (from inline comments).
2.  **Reads Existing `.env` (Optional):**
    *   If a `.env` file exists, its current key-value pairs are read to prefill the form. This ensures existing settings are not lost unless explicitly changed by the user.
3.  **Builds an Interactive Form:**
    *   Uses the `charmbracelet/huh` library to create a user-friendly terminal-based form.
    *   Each variable from `.env.example` becomes an input field.
    *   Fields are pre-filled in this order of priority:
        1.  Value from existing `.env` file.
        2.  Example value from `.env.example` (if the variable wasn't in `.env` or was empty).
        3.  Empty string if no value is found in either.
4.  **Collects User Input:**
    *   The user navigates the form, modifying values as needed.
5.  **Displays Diff and Confirms:**
    *   Compares the new values from the form with the values from the existing `.env` (if any).
    *   Shows a "diff" highlighting what will be added, changed, or cleared.
    *   Prompts the user to confirm whether to save these changes.
6.  **Backs Up and Writes `.env`:**
    *   If the user confirms:
        *   If `.env` already exists, it's backed up to `.env.old`.
        *   The new values are written to the `.env` file. Values are quoted if they contain spaces, special characters, or are empty, to ensure proper parsing by most .env libraries.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details (assuming a LICENSE file will be added, standard for MIT).
