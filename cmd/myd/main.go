package main

import (
	"fmt"
	"os"
	"path/filepath"
	"context"
	"strings"
	"time"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
	"github.com/wraient/myd/internal"
)

func main() {
	// Load config from default location
	config, err := internal.LoadConfig("$HOME/.config/myd/config")
	if err != nil {
		internal.Exit("Failed to load config", err)
	}
	internal.SetGlobalConfig(&config)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Create user struct
	user := &internal.User{}

	switch os.Args[1] {
	case "init":
		internal.ChangeToken(&config, user)
	case "add":
		if len(os.Args) < 3 {
			internal.Exit("Error: Path required for add command", nil)
		}
		handleAdd(os.Args[2], &config)
	case "upload":
		handleUpload(&config, user)
	case "ignore":
		if len(os.Args) < 3 {
			internal.Exit("Error: Path required for ignore command", nil)
		}
		handleIgnore(os.Args[2], &config)
	case "list":
		handleList(&config)
	case "delete":
		handleDelete(&config)
	case "install":
		if len(os.Args) < 3 {
			internal.Exit("Error: GitHub repository URL required", nil)
		}
		handleInstall(os.Args[2], &config)
	case "-e":
		editConfig(&config)
	default:
		printUsage()
	}
}

func editConfig(config *internal.MydConfig) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // fallback to vim if EDITOR is not set
	}

	configPath := os.ExpandEnv("$HOME/.config/myd/config")
	cmd := exec.Command(editor, configPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		internal.Exit("Failed to edit config", err)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  myd init   - Initialize with GitHub token")
	fmt.Println("  myd add    - Add path to upload list")
	fmt.Println("  myd upload - Upload files to GitHub")
	fmt.Println("  myd ignore - Add path to .gitignore")
	fmt.Println("  myd list   - List tracked paths")
	fmt.Println("  myd delete - Delete paths from tracking")
	fmt.Println("  myd install - Install dotfiles from a GitHub repository")
	fmt.Println("  myd -e     - Edit config file")
}

func handleAdd(path string, config *internal.MydConfig) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		internal.Exit("Error getting absolute path", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		internal.Exit("Error: Path does not exist", nil)
	}

	// Create storage directory if it doesn't exist
	uploadDir := filepath.Join(os.ExpandEnv(config.StoragePath), config.UpstreamName)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		internal.Exit("Error creating upload directory", err)
	}

	// Read existing paths
	uploadListPath := filepath.Join(os.ExpandEnv(config.StoragePath), "toupload.txt")
	existingPaths := make(map[string]bool)
	
	if data, err := os.ReadFile(uploadListPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				existingPaths[line] = true
			}
		}
	}

	// Check if path already exists
	if existingPaths[absPath] {
		fmt.Printf("Path %s is already in upload list\n", absPath)
		return
	}

	// Add new path to toupload.txt
	f, err := os.OpenFile(uploadListPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		internal.Exit("Error opening toupload.txt", err)
	}
	defer f.Close()

	// Write the absolute path to toupload.txt
	if _, err := f.WriteString(absPath + "\n"); err != nil {
		internal.Exit("Error writing to toupload.txt", err)
	}

	fmt.Printf("Added %s to upload list\n", absPath)
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, input, info.Mode())
}

func copyDir(src, dst string, skipOriginalPath bool) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .original_path files only during install
		if skipOriginalPath && info.Name() == ".original_path" {
			return nil
		}

		// Get relative path from source directory
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		return copyFile(path, destPath)
	})
}

func copyFilesToRepo(config *internal.MydConfig, repoPath string) error {
	// Read toupload.txt
	uploadListPath := filepath.Join(os.ExpandEnv(config.StoragePath), "toupload.txt")
	paths, err := os.ReadFile(uploadListPath)
	if err != nil {
		return fmt.Errorf("failed to read toupload.txt: %v", err)
	}

	// Process each path
	for _, path := range strings.Split(string(paths), "\n") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		fmt.Printf("Processing path: %s\n", path)

		// Get file info
		info, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Warning: Skipping %s: %v\n", path, err)
			continue
		}

		// Replace $HOME and username with environment variables
		home := os.Getenv("HOME")
		username := os.Getenv("USER")
		originalPath := path
		if home != "" {
			originalPath = strings.ReplaceAll(originalPath, home, "$HOME")
		}
		if username != "" {
			originalPath = strings.ReplaceAll(originalPath, username, "$USER")
		}

		// Get destination path
		destPath := filepath.Join(repoPath, filepath.Base(path))
		fmt.Printf("Copying to: %s\n", destPath)

		if info.IsDir() {
			// For directories, copy the entire directory
			if err := copyDir(path, destPath, false); err != nil {
				return fmt.Errorf("failed to copy directory %s: %v", path, err)
			}
			
			// Create .original_path inside the copied directory
			originalPathFile := filepath.Join(destPath, ".original_path")
			if err := os.WriteFile(originalPathFile, []byte(originalPath), 0644); err != nil {
				return fmt.Errorf("failed to create .original_path in directory %s: %v", path, err)
			}
		} else {
			// For files, copy to destination
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %v", path, err)
			}
			
			// Update root .original_path file for single files
			originalPathFile := filepath.Join(repoPath, ".original_path")
			f, err := os.OpenFile(originalPathFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open .original_path: %v", err)
			}
			defer f.Close()
			
			if _, err := f.WriteString(originalPath + "\n"); err != nil {
				return fmt.Errorf("failed to write to .original_path: %v", err)
			}
		}

		fmt.Printf("Successfully processed: %s\n", path)
	}

	return nil
}

func handleUpload(config *internal.MydConfig, user *internal.User) {
	// Setup GitHub client
	tokenPath := filepath.Join(os.ExpandEnv(config.StoragePath), "token")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		internal.Exit("Failed to read GitHub token. Run 'myd init' first", err)
	}
	user.Token = string(tokenBytes)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: user.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Get authenticated user
	authenticatedUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		internal.Exit("Failed to get authenticated user", err)
	}
	user.Username = *authenticatedUser.Login

	repoPath := filepath.Join(os.ExpandEnv(config.StoragePath), config.UpstreamName)
	fmt.Printf("Repository path: %s\n", repoPath)
	
	// Check if repository exists on GitHub
	_, resp, err := client.Repositories.Get(ctx, user.Username, config.UpstreamName)
	repoExists := err == nil || resp.StatusCode != 404
	fmt.Printf("Repository exists: %v\n", repoExists)

	// Initialize local repository
	if _, err := os.Stat(repoPath); err == nil {
		fmt.Println("Using existing repository directory")
	} else {
		if repoExists {
			fmt.Println("Cloning existing repository")
			cmd := exec.Command("git", "clone", fmt.Sprintf("https://%s@github.com/%s/%s.git", user.Token, user.Username, config.UpstreamName), repoPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				internal.Exit(string(output), err)
			}
		} else {
			fmt.Println("Initializing new repository")
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				internal.Exit("Failed to create repository directory", err)
			}
			cmd := exec.Command("git", "init")
			cmd.Dir = repoPath
			output, err := cmd.CombinedOutput()
			if err != nil {
				internal.Exit(string(output), err)
			}
		}
	}

	// Remove everything except .git from the repo
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		internal.Exit("Failed to read repository directory", err)
	}
	for _, entry := range entries {
		if entry.Name() != ".git" {
			path := filepath.Join(repoPath, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				internal.Exit("Failed to clean repository", err)
			}
		}
	}

	// Copy files using the new function
	fmt.Println("Copying files to repository")
	if err := copyFilesToRepo(config, repoPath); err != nil {
		internal.Exit("Failed to copy files", err)
	}

	fmt.Println("Staging files")
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		internal.Exit(string(output), err)
	}

	// Check git status to see if there are changes
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		internal.Exit(fmt.Sprintf("Failed to get git status: %s", string(output)), err)
	}

	// If there are no changes, exit early
	if len(output) == 0 {
		fmt.Println("Everything up-to-date")
		return
	}

	fmt.Println("Committing changes")
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	// Configure git before committing
	configCmd := exec.Command("git", "config", "--local", "user.name", "myd")
	configCmd.Dir = repoPath
	if output, err := configCmd.CombinedOutput(); err != nil {
		internal.Exit(fmt.Sprintf("Failed to configure git user name: %s", string(output)), err)
	}

	configCmd = exec.Command("git", "config", "--local", "user.email", "myd@local")
	configCmd.Dir = repoPath
	if output, err := configCmd.CombinedOutput(); err != nil {
		internal.Exit(fmt.Sprintf("Failed to configure git user email: %s", string(output)), err)
	}

	cmd = exec.Command("git", "commit", "-m", fmt.Sprintf("automatic update %s", timeStr))
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=myd",
		"GIT_AUTHOR_EMAIL=myd@local",
		"GIT_COMMITTER_NAME=myd",
		"GIT_COMMITTER_EMAIL=myd@local",
	)
	output, err = cmd.CombinedOutput()
	if err != nil {
		internal.Exit(fmt.Sprintf("Failed to commit: %s", string(output)), err)
	}

	if !repoExists {
		fmt.Println("Creating new repository on GitHub")
		repo := &github.Repository{
			Name:    github.String(config.UpstreamName),
			Private: github.Bool(true),
		}
		_, _, err = client.Repositories.Create(ctx, "", repo)
		if err != nil {
			internal.Exit("Failed to create GitHub repository", err)
		}

		fmt.Println("Adding remote")
		cmd = exec.Command("git", "remote", "add", "origin", fmt.Sprintf("https://%s@github.com/%s/%s.git", user.Token, user.Username, config.UpstreamName))
		cmd.Dir = repoPath
		output, err = cmd.CombinedOutput()
		if err != nil {
			internal.Exit(fmt.Sprintf("Failed to add remote: %s", string(output)), err)
		}

		// Add these new commands to ensure main branch is set up correctly
		cmd = exec.Command("git", "branch", "-M", "main")
		cmd.Dir = repoPath
		output, err = cmd.CombinedOutput()
		if err != nil {
			internal.Exit(fmt.Sprintf("Failed to rename branch to main: %s", string(output)), err)
		}
	}

	fmt.Println("Pushing changes")
	cmd = exec.Command("git", "push", "--set-upstream", "origin", "main")
	cmd.Dir = repoPath
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Check if it's just an "up-to-date" message
		if strings.Contains(string(output), "Everything up-to-date") {
			fmt.Println("Everything up-to-date")
			return
		}
		internal.Exit(fmt.Sprintf("Failed to push changes: %s", string(output)), err)
	}

	fmt.Println("Successfully uploaded files to GitHub")
}

func handleIgnore(path string, config *internal.MydConfig) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		internal.Exit("Error getting absolute path", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
			internal.Exit("Error: Path does not exist", nil)
	}

	// Read toupload.txt to find the base directory
	uploadListPath := filepath.Join(os.ExpandEnv(config.StoragePath), "toupload.txt")
	uploadData, err := os.ReadFile(uploadListPath)
	if err != nil {
		internal.Exit("Error reading toupload.txt", err)
	}

	// Find the longest matching path from toupload.txt
	var baseDir string
	for _, line := range strings.Split(string(uploadData), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(absPath, line) && len(line) > len(baseDir) {
			baseDir = line
		}
	}

	if baseDir == "" {
		internal.Exit("Error: Path is not within any directory in toupload.txt", nil)
	}

	// Get the relative path from the base directory
	relPath, err := filepath.Rel(baseDir, absPath)
	if err != nil {
		internal.Exit("Error getting relative path", err)
	}

	// Join with the base name of the directory
	repoIgnorePath := filepath.Join(filepath.Base(baseDir), relPath)

	// Create or open .gitignore file
	repoPath := filepath.Join(os.ExpandEnv(config.StoragePath), config.UpstreamName)
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	
	// Read existing entries
	existingEntries := make(map[string]bool)
	if data, err := os.ReadFile(gitignorePath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				existingEntries[line] = true
			}
		}
	}

	// Check if entry already exists
	if existingEntries[repoIgnorePath] {
		fmt.Printf("Path %s is already in .gitignore\n", repoIgnorePath)
		return
	}

	// Append new entry
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		internal.Exit("Error opening .gitignore", err)
	}
	defer f.Close()

	if _, err := f.WriteString(repoIgnorePath + "\n"); err != nil {
		internal.Exit("Error writing to .gitignore", err)
	}

	fmt.Printf("Added %s to .gitignore\n", repoIgnorePath)
}

func handleList(config *internal.MydConfig) {
	// Read toupload.txt
	uploadListPath := filepath.Join(os.ExpandEnv(config.StoragePath), "toupload.txt")
	data, err := os.ReadFile(uploadListPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No paths are currently being tracked")
			return
		}
		internal.Exit("Error reading toupload.txt", err)
	}

	paths := strings.Split(string(data), "\n")
	if len(paths) == 0 || (len(paths) == 1 && paths[0] == "") {
		fmt.Println("No paths are currently being tracked")
		return
	}

	fmt.Println("Currently tracked paths:")
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path != "" {
			fmt.Printf("  %s\n", path)
		}
	}
}

func handleDelete(config *internal.MydConfig) {
	p := tea.NewProgram(internal.NewDeleteModel(config))
	if _, err := p.Run(); err != nil {
		internal.Exit("Error running delete menu", err)
	}
}

func handleInstall(repoURL string, config *internal.MydConfig) {
	// Parse GitHub URL
	repoName := filepath.Base(repoURL)
	repoName = strings.TrimSuffix(repoName, ".git")

	// Create temp directory for cloning
	tempDir := filepath.Join(os.ExpandEnv(config.StoragePath), "temp", repoName)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		internal.Exit("Failed to create temp directory", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the repository
	fmt.Printf("Cloning %s...\n", repoURL)
	cmd := exec.Command("git", "clone", repoURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		internal.Exit(string(output), err)
	}

	// Read root .original_path file
	rootOriginalPath := filepath.Join(tempDir, ".original_path")
	if data, err := os.ReadFile(rootOriginalPath); err == nil {
		// Handle single files
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			
			// Expand environment variables
			path := os.ExpandEnv(line)
			baseName := filepath.Base(path)
			srcPath := filepath.Join(tempDir, baseName)
			
			// Create parent directory if it doesn't exist
			parentDir := filepath.Dir(path)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				fmt.Printf("Warning: Failed to create directory for %s: %v\n", path, err)
				continue
			}

			// Copy file to original location
			if err := copyFile(srcPath, path); err != nil {
				fmt.Printf("Warning: Failed to copy %s: %v\n", baseName, err)
				continue
			}
			fmt.Printf("Installed %s\n", path)
		}
	}

	// Handle directories with .original_path
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		internal.Exit("Failed to read repository contents", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" {
			continue
		}

		dirPath := filepath.Join(tempDir, entry.Name())
		originalPathFile := filepath.Join(dirPath, ".original_path")
		
		data, err := os.ReadFile(originalPathFile)
		if err != nil {
			continue
		}

		originalPath := strings.TrimSpace(string(data))
		if originalPath == "" {
			continue
		}

		// Expand environment variables
		destPath := os.ExpandEnv(originalPath)

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			fmt.Printf("Warning: Failed to create directory for %s: %v\n", destPath, err)
			continue
		}

		// Copy directory to original location
		if err := copyDir(dirPath, destPath, true); err != nil {
			fmt.Printf("Warning: Failed to copy directory %s: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("Installed %s\n", destPath)
	}

	fmt.Println("Installation complete!")
}
