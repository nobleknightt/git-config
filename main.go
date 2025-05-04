package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-ini/ini"
	"github.com/google/uuid"
)

// Version information
const (
	appVersion = "0.1.0"
	appName    = "git-config"
)

// FormData holds user input from the form
type FormData struct {
	DirectoryName string
	KeyType       string
	GitUsername   string
	GitEmail      string
	SignCommits   bool
}

// ANSI color codes (using lipgloss preferred colors where possible)
var (
	styleGood    = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")) // Green
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange/Yellow
	styleInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")) // DeepSkyBlue
	styleKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")) // Cyan
	styleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red
	stylePath    = lipgloss.NewStyle().Italic(true)                          // Italic for paths
	styleKeyText = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E5E5")) // Light gray for key text
)

// File modes
const (
	dirMode    os.FileMode = 0755
	sshDirMode os.FileMode = 0700
)

func main() {
	// Check if this is a version command
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("%s version %s\n", appName, appVersion)
		return
	}

	var data FormData
	keyTypes := []string{"ed25519", "rsa"} // Consider adding ecdsa if desired

	// --- Form Definition (unchanged) ---
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Directory Name").
				Description("Enter the name of the directory to create or use (e.g., github-personal, work-project)"). // Updated description
				Placeholder("projects").
				Value(&data.DirectoryName).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("directory name cannot be empty")
					}
					// Basic check for invalid path characters (OS dependent, but covers common cases)
					if strings.ContainsAny(s, `/\:*?"<>|`) {
						return fmt.Errorf("directory name contains invalid characters")
					}
					return nil
				}),

			huh.NewSelect[string]().
				Title("SSH Key Type").
				Description("Select the SSH key type (ed25519 recommended)").
				Options(
					huh.NewOptions(keyTypes...)...,
				).
				Value(&data.KeyType),

			huh.NewInput().
				Title("Git Username").
				Description("Enter the Git username for this context").
				Placeholder("username").
				Value(&data.GitUsername).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("git username cannot be empty")
					}
					return nil
				}),

			huh.NewInput().
				Title("Git Email").
				Description("Enter the Git email for this context").
				Placeholder("user@example.com").
				Value(&data.GitEmail).
				Validate(func(s string) error {
					// Basic email format check
					if s == "" || !strings.Contains(s, "@") || !strings.Contains(s, ".") {
						return fmt.Errorf("please enter a valid email address")
					}
					return nil
				}),

			huh.NewConfirm().
				Title("Sign Commits?").
				Description("Sign Git commits using this SSH key? (Requires Git 2.34+)").
				Value(&data.SignCommits),
		),
	)

	err := form.Run()
	if err != nil {
		// Check for specific error types if needed (e.g., huh.ErrUserAborted)
		fmt.Fprintf(os.Stderr, "%s Form cancelled or failed: %v\n", styleError.Render("Error:"), err)
		os.Exit(1)
	}

	// Process the form data
	messages, err := processFormData(data)
	if err != nil {
		// Log error clearly before exiting
		fmt.Fprintf(os.Stderr, "%s %v\n", styleError.Render("Error:"), err)
		os.Exit(1)
	}

	printBorderedMessages(messages)
}

// printBorderedMessages prints all messages with a styled border
func printBorderedMessages(messages []string) {
	width := 80 // Keep fixed width for simplicity, adjust if needed

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#04B575")). // Green border
		Padding(1, 2).
		Width(width).
		Align(lipgloss.Left)

	// Join messages with newlines for rendering within the box
	content := strings.Join(messages, "\n")

	fmt.Println() // Add spacing before the box
	fmt.Println(boxStyle.Render(content))
	fmt.Println() // Add spacing after the box
}

// processFormData handles the core logic: dir creation/check, keygen, config updates
func processFormData(data FormData) ([]string, error) {
	messages := []string{}

	// 1. Check/Create the target directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	dirPath := filepath.Join(cwd, data.DirectoryName)
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for '%s': %w", dirPath, err)
	}

	// Check if directory already exists
	if _, err := os.Stat(absPath); err == nil {
		messages = append(messages, styleInfo.Render("Directory already exists:")+" "+stylePath.Render(absPath))
		// Directory exists, continue without creating
	} else if os.IsNotExist(err) {
		// Directory does not exist, create it
		err = os.MkdirAll(absPath, dirMode)
		if err != nil {
			return nil, fmt.Errorf("failed to create directory '%s': %w", stylePath.Render(absPath), err)
		}
		messages = append(messages, styleInfo.Render("Created directory:")+" "+stylePath.Render(absPath))
	} else {
		// Some other error occurred while checking directory status
		return nil, fmt.Errorf("failed to check directory status '%s': %w", stylePath.Render(absPath), err)
	}

	// 2. Generate SSH Key
	// This function checks for existing key files and will error out if they exist.
	// This prevents accidental overwriting of existing keys.
	keyName := fmt.Sprintf("%s-%s", data.DirectoryName, uuid.New().String())
	privateKeyPath, publicKeyPath, err := generateSSHKey(data.KeyType, keyName)
	if err != nil {
		// Attempt cleanup on failure? Maybe too complex for this script.
		return nil, fmt.Errorf("failed to generate SSH key: %w", err)
	}
	messages = append(messages, styleKey.Render("Generated SSH key:")+" "+stylePath.Render(privateKeyPath))

	// 3. Read public key content
	publicKeyContentBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key '%s': %w", stylePath.Render(publicKeyPath), err)
	}
	publicKeyContent := string(publicKeyContentBytes)

	// 4. Try to copy public key to clipboard
	clipboardErr := clipboard.WriteAll(publicKeyContent)

	// 5. Prepare paths for Git config (Git often needs POSIX-style paths)
	linuxPrivateKeyPath := convertToLinuxPath(privateKeyPath)
	linuxPublicKeyPath := convertToLinuxPath(publicKeyPath)

	// 6. Create/Update local .gitconfig
	// This function uses ini.Empty() and then saves, effectively overwriting or creating the file.
	// If you wanted to *merge* with an existing local config, you'd need to load it first.
	// For this script's purpose (setting specific user/key for a directory), overwriting is intended.
	localGitConfigPath, err := createLocalGitConfig(absPath, data, linuxPrivateKeyPath, linuxPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create local .gitconfig: %w", err)
	}
	messages = append(messages, styleWarn.Render("Created/Updated local .gitconfig:")+" "+stylePath.Render(localGitConfigPath)) // Updated message

	// 7. Update global .gitconfig
	// This function loads the existing global config and adds the includeIf directive if it doesn't exist.
	globalGitConfigPath, err := updateGlobalGitConfig(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to update global .gitconfig: %w", err)
	}
	messages = append(messages, styleWarn.Render("Updated global .gitconfig:")+" "+stylePath.Render(globalGitConfigPath))

	// --- Format Final Output Messages ---
	messages = append(messages, "") // Separator
	messages = append(messages, styleGood.Render("Setup completed successfully!"))
	messages = append(messages, "")
	messages = append(messages, styleKey.Render("Your SSH Public Key:"))
	messages = append(messages, styleKeyText.Render(strings.TrimSpace(publicKeyContent))) // Trim whitespace

	// Clipboard status message
	if clipboardErr == nil {
		messages = append(messages, "") // Seperator
		messages = append(messages, styleGood.Render("Public key copied to clipboard"))
	} else {
		messages = append(messages, styleWarn.Render(fmt.Sprintf("Could not copy public key to clipboard: %v", clipboardErr)))
	}

	// Instructions
	var keyUsage string
	if data.SignCommits {
		keyUsage = "as both an Authentication key AND a Signing key"
	} else {
		keyUsage = "as an Authentication key"
	}

	instructionPrefix := "Please add this key"
	if clipboardErr == nil {
		instructionPrefix = "Please add the copied key"
	}

	messages = append(messages, "")
	messages = append(messages, styleWarn.Render(fmt.Sprintf("%s to your Git provider (GitHub, GitLab, etc.) %s.", instructionPrefix, keyUsage)))
	messages = append(messages, styleWarn.Render("Find this under SSH and GPG keys (or similar) in your account settings."))

	return messages, nil
}

// generateSSHKey creates the SSH key pair in the user's .ssh directory
func generateSSHKey(keyType, keyName string) (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}
	sshDir := filepath.Join(homeDir, ".ssh")

	// Create .ssh directory if it doesn't exist
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(sshDir, sshDirMode); mkErr != nil {
			return "", "", fmt.Errorf("failed to create .ssh directory '%s': %w", stylePath.Render(sshDir), mkErr)
		}
	} else if err != nil {
		return "", "", fmt.Errorf("failed to check .ssh directory '%s': %w", stylePath.Render(sshDir), err)
	}

	// Define key paths
	// Ensure keyName is filesystem-safe (though directory name validation helps)
	safeKeyName := strings.ReplaceAll(keyName, string(filepath.Separator), "_")
	privateKeyPath := filepath.Join(sshDir, safeKeyName)
	publicKeyPath := privateKeyPath + ".pub"

	// Check if key files already exist (unlikely with UUID, but good practice)
	if _, err := os.Stat(privateKeyPath); err == nil {
		return "", "", fmt.Errorf("SSH key file already exists: %s. Please remove or rename it to generate a new one", stylePath.Render(privateKeyPath)) // Added suggestion
	}
	if _, err := os.Stat(publicKeyPath); err == nil {
		return "", "", fmt.Errorf("SSH public key file already exists: %s. Please remove or rename it to generate a new one", stylePath.Render(publicKeyPath)) // Added suggestion
	}

	// Prepare ssh-keygen command
	keygenArgs := []string{
		"-t", keyType,
		"-f", privateKeyPath, // Use the platform-native path for the -f argument
		"-N", "", // No passphrase
		"-C", safeKeyName, // Add keyName as comment
	}
	if keyType == "rsa" {
		keygenArgs = append(keygenArgs, "-b", "4096") // Specify RSA key size
	}

	cmd := exec.Command("ssh-keygen", keygenArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("ssh-keygen failed (output: %s): %w", strings.TrimSpace(string(output)), err)
	}

	// Set private key permissions (important!)
	if runtime.GOOS != "windows" { // Chmod typically not used/needed this way on Windows keys
		if err := os.Chmod(privateKeyPath, 0600); err != nil {
			// Log a warning, maybe not fatal? Or return error? Let's warn for now.
			fmt.Fprintf(os.Stderr, "%s Could not set private key permissions (chmod 600) on %s: %v\n", styleWarn.Render("Warning:"), stylePath.Render(privateKeyPath), err)
		}
	}

	return privateKeyPath, publicKeyPath, nil
}

// createLocalGitConfig generates the .gitconfig file within the target directory
// This function will overwrite an existing .gitconfig in the target directory.
func createLocalGitConfig(dirPath string, data FormData, linuxPrivateKeyPath, linuxPublicKeyPath string) (string, error) {
	cfg := ini.Empty() // Start with an empty config, effectively overwriting

	// [user] section
	userSection := cfg.Section("user")
	userSection.NewKey("name", data.GitUsername)
	userSection.NewKey("email", data.GitEmail)
	if data.SignCommits {
		// Use the Linux-style path here as Git often expects it for config values
		userSection.NewKey("signingkey", linuxPublicKeyPath)
	}

	// [core] section
	coreSection := cfg.Section("core")
	// Use Linux-style path for ssh command argument, even on Windows
	sshCommand := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes", linuxPrivateKeyPath)
	coreSection.NewKey("sshCommand", sshCommand)

	// Commit signing sections (only if requested)
	if data.SignCommits {
		// [gpg] section
		gpgSection := cfg.Section("gpg")
		gpgSection.NewKey("format", "ssh")

		// [commit] section
		commitSection := cfg.Section("commit")
		commitSection.NewKey("gpgsign", "true")

		// [tag] section (optional, but good practice to sign tags too)
		tagSection := cfg.Section("tag")
		tagSection.NewKey("gpgsign", "true")

	}

	// Save the config file
	gitConfigPath := filepath.Join(dirPath, ".gitconfig")
	err := cfg.SaveTo(gitConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to save local .gitconfig to '%s': %w", stylePath.Render(gitConfigPath), err)
	}
	return gitConfigPath, nil
}

// updateGlobalGitConfig adds an includeIf directive to the global ~/.gitconfig
// This function loads the existing global config and adds the directive if not present.
func updateGlobalGitConfig(targetDirPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	globalGitConfigPath := filepath.Join(homeDir, ".gitconfig")

	// Ensure the global config file exists, creating if necessary
	if _, err := os.Stat(globalGitConfigPath); os.IsNotExist(err) {
		fmt.Printf("%s Global .gitconfig not found at %s, creating it.\n", styleWarn.Render("Info:"), stylePath.Render(globalGitConfigPath))
		file, createErr := os.Create(globalGitConfigPath)
		if createErr != nil {
			return "", fmt.Errorf("failed to create global .gitconfig '%s': %w", stylePath.Render(globalGitConfigPath), createErr)
		}
		file.Close() // Close immediately after creation
	} else if err != nil {
		return "", fmt.Errorf("failed to check global .gitconfig '%s': %w", stylePath.Render(globalGitConfigPath), err)
	}

	// Load global .gitconfig (using loose load options for flexibility)
	cfg, err := ini.LoadSources(ini.LoadOptions{AllowBooleanKeys: true, Loose: true}, globalGitConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to load global .gitconfig '%s': %w", stylePath.Render(globalGitConfigPath), err)
	}

	// --- Prepare paths for the includeIf directive ---
	// The 'gitdir:' path for includeIf often requires forward slashes, even on Windows.
	// It should also usually end with a '/'
	includeIfDir := strings.ReplaceAll(targetDirPath, "\\", "/") + "/"
	// The 'path' value should point to the local .gitconfig file.
	// This path can often be relative to the global config or absolute.
	// Using an absolute path converted to forward slashes is generally safest.
	localConfigPath := filepath.Join(targetDirPath, ".gitconfig")
	includeIfPathValue := strings.ReplaceAll(localConfigPath, "\\", "/")

	// Add the includeIf section
	// Section name uses the specific gitdir path
	sectionName := fmt.Sprintf(`includeIf "gitdir:%s"`, includeIfDir)
	includeSection := cfg.Section(sectionName)

	// Check if this exact include already exists to prevent duplicates
	if key, _ := includeSection.GetKey("path"); key == nil || key.Value() != includeIfPathValue {
		includeSection.NewKey("path", includeIfPathValue)
	} else {
		// Optional: Add a message if the include already exists?
		// messages = append(messages, styleInfo.Render("Include directive already exists in global .gitconfig"))
		// For now, just don't add it again.
	}

	// Save the updated global config
	err = cfg.SaveTo(globalGitConfigPath)
	if err != nil {
		return "", fmt.Errorf("failed to save updated global .gitconfig '%s': %w", stylePath.Render(globalGitConfigPath), err)
	}
	return globalGitConfigPath, nil
}

// convertToLinuxPath converts a Windows path (e.g., C:\Users\X) to a
// POSIX-like path (e.g., /c/Users/X) often required by Git/SSH tools within config files.
// Non-Windows paths are returned unchanged.
func convertToLinuxPath(path string) string {
	if runtime.GOOS != "windows" {
		return path // No conversion needed for non-Windows
	}

	// Use filepath.ToSlash for basic conversion
	p := filepath.ToSlash(path)

	// Handle drive letters (e.g., C:/Users/...) -> /c/Users/...
	if len(p) > 1 && p[1] == ':' {
		p = "/" + strings.ToLower(string(p[0])) + p[2:]
	}
	return p
}
