package i18n

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// translations stores all loaded translations
// Structure: lang -> module -> key -> value
var translations = make(map[string]map[string]map[string]string)
var mu sync.RWMutex

// Load loads all translation files from the modules directory
func Load(modulesPath string) error {
	mu.Lock()
	defer mu.Unlock()

	entries, err := os.ReadDir(modulesPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		moduleName := entry.Name()
		langPath := filepath.Join(modulesPath, moduleName, "views", "lang")

		langFiles, err := os.ReadDir(langPath)
		if err != nil {
			// Module might not have translations, skip
			continue
		}

		for _, langFile := range langFiles {
			if filepath.Ext(langFile.Name()) != ".json" {
				continue
			}

			langCode := langFile.Name()[:len(langFile.Name())-5] // Remove .json

			data, err := os.ReadFile(filepath.Join(langPath, langFile.Name()))
			if err != nil {
				log.Printf("Warning: Could not read translation file %s/%s: %v",
					moduleName, langFile.Name(), err)
				continue
			}

			var langData map[string]string
			if err := json.Unmarshal(data, &langData); err != nil {
				log.Printf("Warning: Could not parse translation file %s/%s: %v",
					moduleName, langFile.Name(), err)
				continue
			}

			// Initialize maps if needed
			if translations[langCode] == nil {
				translations[langCode] = make(map[string]map[string]string)
			}
			if translations[langCode][moduleName] == nil {
				translations[langCode][moduleName] = make(map[string]string)
			}

			// Copy translations, skip metadata
			for k, v := range langData {
				if k != "_meta" {
					translations[langCode][moduleName][k] = v
				}
			}

			log.Printf("Loaded translations: %s/%s (%d keys)",
				moduleName, langCode, len(langData))
		}
	}

	return nil
}

// Translate translates text with module fallback
// Fallback chain: module translation -> core translation -> original text
func Translate(lang, module, text string) string {
	mu.RLock()
	defer mu.RUnlock()

	// Try module-specific translation
	if module != "core" {
		if langData, ok := translations[lang]; ok {
			if moduleData, ok := langData[module]; ok {
				if translated, ok := moduleData[text]; ok {
					return translated
				}
			}
		}
	}

	// Fallback to core module
	if langData, ok := translations[lang]; ok {
		if coreData, ok := langData["core"]; ok {
			if translated, ok := coreData[text]; ok {
				return translated
			}
		}
	}

	// Return original text if no translation found
	return text
}

// T is a shorthand for Translate
func T(lang, module, text string) string {
	return Translate(lang, module, text)
}

// GetAvailableLanguages returns all available language codes
func GetAvailableLanguages() []string {
	mu.RLock()
	defer mu.RUnlock()

	langs := make([]string, 0, len(translations))
	for lang := range translations {
		langs = append(langs, lang)
	}
	return langs
}
