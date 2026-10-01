package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"latex-editor/internal/compiler"
	"latex-editor/internal/server"
	"latex-editor/internal/workspace"
	"latex-editor/web"
)

func main() {
	port := flag.Int("port", 8081, "Port for the LaTeX editor web service")
	dir := flag.String("dir", ".", "Working directory containing LaTeX files")
	engineFlag := flag.String("engine", "pdflatex", "LaTeX engine binary (e.g. pdflatex, xelatex, lualatex)")
	openBrowser := flag.Bool("open", false, "Automatically open web browser on startup")
	flag.Parse()

	ws, err := workspace.New(*dir)
	if err != nil {
		log.Fatalf("Error initializing workspace: %v", err)
	}

	engine := compiler.NewPdflatexEngine()
	if *engineFlag != "" {
		engine.BinaryPath = *engineFlag
	}

	srv := server.New(ws, engine)

	if assetsFS, err := web.GetAssetsFS(); err == nil {
		srv.SetWebHandler(server.NewSPAHandler(assetsFS))
	}

	addr := fmt.Sprintf(":%d", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv.Routes(),
	}

	// Graceful shutdown channel
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("\n==================================================\n")
		fmt.Printf("  LaTeX Editor running at: http://localhost:%d\n", *port)
		fmt.Printf("  Serving directory: %s\n", ws.RootDir())
		fmt.Printf("==================================================\n\n")

		if *openBrowser {
			go func() {
				time.Sleep(100 * time.Millisecond)
				_ = exec.Command("xdg-open", fmt.Sprintf("http://localhost:%d", *port)).Start()
			}()
		}

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stopChan
	fmt.Println("\nShutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown warning: %v", err)
	}
	fmt.Println("Server stopped.")
}
