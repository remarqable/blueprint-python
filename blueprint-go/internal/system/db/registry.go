package db

import (
	"fmt"
	"sync"
)

// ModelInfo holds metadata about a registered model
type ModelInfo struct {
	Module    string
	Model     string
	TableName string
	Order     int
}

// ModelRegistry tracks all GORM models across modules
type ModelRegistry struct {
	mu     sync.RWMutex
	models []ModelInfo
	order  int
}

var globalRegistry = &ModelRegistry{}

// Register adds a model to the registry
// Called via init() functions in model files
func Register(moduleName, modelName, tableName string) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.order++
	globalRegistry.models = append(globalRegistry.models, ModelInfo{
		Module:    moduleName,
		Model:     modelName,
		TableName: tableName,
		Order:     globalRegistry.order,
	})
}

// GetModels returns all registered models
func GetModels() []ModelInfo {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]ModelInfo, len(globalRegistry.models))
	copy(result, globalRegistry.models)
	return result
}

// PrintSummary prints a summary of all registered models
func PrintSummary() {
	models := GetModels()

	fmt.Println("\nDatabase Model Registry:")
	fmt.Println("--------   ----------   --------------------")
	fmt.Println("Module     Model        Table")
	fmt.Println("--------   ----------   --------------------")

	for _, m := range models {
		fmt.Printf("%-10s %-12s %s\n", m.Module, m.Model, m.TableName)
	}
	fmt.Println()
}
