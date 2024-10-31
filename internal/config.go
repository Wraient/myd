package internal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// MydConfig struct with field names that match the config keys
type MydConfig struct {
	StoragePath             string `config:"StoragePath"`
	UpstreamName            string `config:"UpstreamName"`
	Username                string `config:"Username"`
}

// Default configuration values as a map
func defaultConfigMap() map[string]string {
	return map[string]string{
		"StoragePath":             "$HOME/.local/share/myd",
		"UpstreamName":            "dotfilestest",
		"Username":                "",
	}
}

var globalConfig *MydConfig

func SetGlobalConfig(config *MydConfig) {
	globalConfig = config
}

func GetGlobalConfig() *MydConfig {
	return globalConfig
}

// LoadConfig reads or creates the config file, adds missing fields, and returns the populated CurdConfig struct
func LoadConfig(configPath string) (MydConfig, error) {
	configPath = os.ExpandEnv(configPath) // Substitute environment variables like $HOME

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create the config file with default values if it doesn't exist
		fmt.Println("Config file not found. Creating default config...")
		if err := createDefaultConfig(configPath); err != nil {
			return MydConfig{}, fmt.Errorf("error creating default config file: %v", err)
		}
	}

	// Load the config from file
	configMap, err := loadConfigFromFile(configPath)
	if err != nil {
		return MydConfig{}, fmt.Errorf("error loading config file: %v", err)
	}

	// Add missing fields to the config map
	updated := false
	defaultConfigMap := defaultConfigMap()
	for key, defaultValue := range defaultConfigMap {
		if _, exists := configMap[key]; !exists {
			configMap[key] = defaultValue
			updated = true
		}
	}

	// Write updated config back to file if there were any missing fields
	if updated {
		if err := saveConfigToFile(configPath, configMap); err != nil {
			return MydConfig{}, fmt.Errorf("error saving updated config file: %v", err)
		}
	}

	// Populate the CurdConfig struct from the config map
	config := populateConfig(configMap)

	return config, nil
}

// Create a config file with default values in key=value format
// Ensure the directory exists before creating the file
func createDefaultConfig(path string) error {
	defaultConfig := defaultConfigMap()

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for key, value := range defaultConfig {
		line := fmt.Sprintf("%s=%s\n", key, value)
		if _, err := writer.WriteString(line); err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("error flushing writer: %v", err)
	}
	return nil
}

func ChangeToken(config *MydConfig, user *User) {
	fmt.Print("Enter your GitHub username: ")
	fmt.Scanln(&user.Username)
	
	fmt.Print("Please generate a token and paste it here: ")
	fmt.Scanln(&user.Token)
	
	err := WriteTokenToFile(user.Token, filepath.Join(os.ExpandEnv(config.StoragePath), "token"))
	if err != nil {
		Exit("Failed to save token", err)
	}
	
	err = WriteTokenToFile(user.Username, filepath.Join(os.ExpandEnv(config.StoragePath), "username"))
	if err != nil {
		Exit("Failed to save username", err)
	}
}

// CreateOrWriteTokenFile creates the token file if it doesn't exist and writes the token to it
func WriteTokenToFile(token string, filePath string) error {
    // Extract the directory path
    dir := filepath.Dir(filePath)

    // Create all necessary parent directories
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create directories: %v", err)
    }

    // Write the token to the file, creating it if it doesn't exist
    err := os.WriteFile(filePath, []byte(token), 0644)
    if err != nil {
        return fmt.Errorf("failed to write token to file: %v", err)
    }

    return nil
}

// Load config file from disk into a map (key=value format)
func loadConfigFromFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	configMap := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			configMap[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return configMap, nil
}

// Save updated config map to file in key=value format
func saveConfigToFile(path string, configMap map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for key, value := range configMap {
		line := fmt.Sprintf("%s=%s\n", key, value)
		if _, err := writer.WriteString(line); err != nil {
			return err
		}
	}
	return writer.Flush()
}

// Populate the CurdConfig struct from a map
func populateConfig(configMap map[string]string) MydConfig {
	config := MydConfig{}
	configValue := reflect.ValueOf(&config).Elem()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Type().Field(i)
		tag := field.Tag.Get("config")

		if value, exists := configMap[tag]; exists {
			fieldValue := configValue.FieldByName(field.Name)

			if fieldValue.CanSet() {
				switch fieldValue.Kind() {
				case reflect.String:
					fieldValue.SetString(value)
				case reflect.Int:
					intVal, _ := strconv.Atoi(value)
					fieldValue.SetInt(int64(intVal))
				case reflect.Bool:
					boolVal, _ := strconv.ParseBool(value)
					fieldValue.SetBool(boolVal)
				}
			}
		}
	}

	return config
}
