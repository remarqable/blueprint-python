package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"blueprint-go/internal/app"
	"blueprint-go/internal/modules/core"
)

func main() {
	// Load configuration
	config := app.LoadConfig()

	// Create application
	application, err := app.New(config)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	// Register modules
	// Add your modules here in the order they should load
	application.RegisterModule(core.Module)
	// application.RegisterModule(tasks.Module)  // Example: add more modules

	// Initialize application
	modulesPath := "internal/modules"
	if err := application.Initialize(modulesPath); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		if err := application.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Run server
	if err := application.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
