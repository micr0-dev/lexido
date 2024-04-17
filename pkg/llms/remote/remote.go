package remote

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	lexio "github.com/micr0-dev/lexido/pkg/io"
)

const defaultConfig = `{
	"api_config": {
	  "url": "https://api.example.com/v1/chat/completions", // Replace with your API endpoint
	  "headers": {
		"Content-Type": "application/json",
		"Authorization": "Bearer 123124534545634", // Replace with your API key
		"Accept": "application/json"
	  },
	  "data_template": {
		"model": "example-model",
		"messages": "<PROMPT>" // This is where the prompt will be inserted
	  }
	}
  }`

// Config represents the structure of the JSON configuration file
type Config struct {
	ApiConfig struct {
		URL          string            `json:"url"`
		Headers      map[string]string `json:"headers"`
		DataTemplate interface{}       `json:"data_template"`
		FieldOutput  string            `json:"field_to_extract"`
	} `json:"api_config"`
}

// replacePrompt recursively searches for the <PROMPT> placeholder and replaces it
func replacePrompt(data interface{}, prompt string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = replacePrompt(value, prompt)
		}
	case []interface{}:
		for i, item := range v {
			v[i] = replacePrompt(item, prompt)
		}
	case string:
		if v == "<PROMPT>" {
			return prompt
		}
	}
	return data
}

// LoadConfig loads the configuration from the file and returns it
func LoadConfig() (Config, error) {
	filepath, err := lexio.GetFilePath("remoteConfig.json")
	if err != nil {
		return Config{}, err
	}

	// Load the configuration from file
	configFile, err := os.ReadFile(filepath)
	if err != nil {
		// Create a default configuration file if it doesn't exist
		err := os.WriteFile(filepath, []byte(defaultConfig), 0644)
		if err != nil {
			return Config{}, err
		}
		return Config{}, errors.New("A remote configuration file not found. A default configuration file has been created at " + filepath)
	}

	var config Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return Config{}, err
	}

	return config, nil
}

// ExtractOutput initiates the extraction process by unmarshaling the JSON response and calling findField recursively.
func ExtractOutput(response []byte, field string) string {
	var output map[string]interface{}
	if err := json.Unmarshal(response, &output); err != nil {
		log.Fatal(err)
	}
	return findField(output, field)
}

// findField recursively searches for the field within the nested JSON structure.
func findField(data interface{}, field string) string {
	switch v := data.(type) {
	case map[string]interface{}:
		// If the field exists at this level, return it.
		if value, exists := v[field]; exists {
			// Convert the value to a string, if possible.
			if strValue, ok := value.(string); ok {
				return strValue
			}
			// If it's not a string but a nested structure, you might want to handle it differently or return an indication of its type.
			return fmt.Sprintf("Found, but not a string: %T", value)
		}
		// Otherwise, search recursively in each value.
		for _, value := range v {
			if found := findField(value, field); found != "" {
				return found
			}
		}
	case []interface{}:
		// Search each element in the array.
		for _, item := range v {
			if found := findField(item, field); found != "" {
				return found
			}
		}
	}
	// Return an empty string if the field is not found.
	return ""
}

// Generate sends a POST request to the API endpoint with the prompt and returns the response
func Generate(prompt string) (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}

	// Replace <PROMPT> in the DataTemplate
	config.ApiConfig.DataTemplate = replacePrompt(config.ApiConfig.DataTemplate, prompt)

	// Marshal the data template back into JSON for the API request
	jsonData, err := json.Marshal(config.ApiConfig.DataTemplate)
	if err != nil {
		log.Fatal(err)
	}

	// Create and send the API request
	req, err := http.NewRequest("POST", config.ApiConfig.URL, strings.NewReader(string(jsonData)))
	if err != nil {
		log.Fatal(err)
	}
	for key, value := range config.ApiConfig.Headers {
		req.Header.Add(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return ExtractOutput(responseBody, config.ApiConfig.FieldOutput), nil
}
