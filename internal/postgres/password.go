package postgres

import (
	"bytes"
	"fmt"
	"math/rand"
	"text/template"
	"time"

	v1alpha1 "github.com/lovelinuxalot/postgres-operator/api/v1alpha1"
)

// SecretData is the data that will be passed to the template
type SecretData struct {
	Username string
	Password string
	Host     string
	Database string
}

// GeneratePassword generates a random password based on the provided PasswordSpec
func GeneratePassword(spec *v1alpha1.PasswordSpec) string {
	// Create a new random generator with a seed based on the current time
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var letters []rune

	// Add custom or default numbers
	if spec.Numbers {
		if spec.NumbersOverride != "" {
			letters = append(letters, []rune(spec.NumbersOverride)...)
		} else {
			letters = append(letters, []rune("0123456789")...)
		}
	}

	// Add custom or default symbols
	if spec.Symbols {
		if spec.SymbolsOverride != "" {
			letters = append(letters, []rune(spec.SymbolsOverride)...)
		} else {
			letters = append(letters, []rune("!@#$%^&*()_+-=[]{}|;:,.<>?")...)
		}
	}

	// Add custom or default alphabets
	if spec.Alphabets {
		if spec.AlphabetsOverride != "" {
			letters = append(letters, []rune(spec.AlphabetsOverride)...)
		} else {
			letters = append(letters, []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")...)
		}
	}

	// Generate the password with the specified length
	password := make([]rune, spec.Length)
	for i := range password {
		password[i] = letters[rng.Intn(len(letters))]
	}

	return string(password)
}

// ApplyTemplate applies the user's custom template or the default template to generate the secret data
func ApplyTemplate(tmplString string, data SecretData) (map[string]string, error) {
	tmpl, err := template.New("secretTemplate").Parse(tmplString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Parse the generated string into a map (assuming a simple key=value format)
	secretMap := make(map[string]string)
	for _, line := range bytes.Split(buf.Bytes(), []byte("\n")) {
		parts := bytes.SplitN(line, []byte("="), 2)
		if len(parts) == 2 {
			secretMap[string(parts[0])] = string(parts[1])
		}
	}
	return secretMap, nil
}
