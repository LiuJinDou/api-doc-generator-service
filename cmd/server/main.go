package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-doc-generator/internal/config"
	"api-doc-generator/internal/parser"
	ginparser "api-doc-generator/internal/parser/gin"
	"api-doc-generator/internal/webhook"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize parser registry
	parserRegistry := parser.NewRegistry()
	parserRegistry.Register("go-gin", ginparser.NewGinParser())
	// Future parsers can be registered here:
	// parserRegistry.Register("node-express", express.NewExpressParser())
	// parserRegistry.Register("python-fastapi", fastapi.NewFastAPIParser())

	log.Printf("Registered parsers: %v", parserRegistry.List())

	// Setup HTTP server
	r := gin.Default()

	// Webhook endpoints
	webhookHandler := webhook.NewHandler(cfg, parserRegistry)
	r.POST("/webhook/github", webhookHandler.HandleGitHub)
	r.POST("/webhook/gitlab", webhookHandler.HandleGitLab)

	// Manual trigger API
	r.POST("/api/v1/analyze", webhookHandler.ManualTrigger)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"version": "1.0.0",
			"parsers": parserRegistry.List(),
		})
	})

	// Info endpoint
	r.GET("/api/v1/info", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service":           "API Documentation Generator",
			"version":           "1.0.0",
			"supported_parsers": parserRegistry.List(),
		})
	})

	// 静态文件服务 - 让docs目录可以被外部访问
	r.Static("/docs", "./docs")

	// Graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		log.Printf("🚀 API Doc Generator Service started on port %s", cfg.Server.Port)
		log.Printf("📡 Webhook endpoint: http://localhost:%s/webhook/github", cfg.Server.Port)
		log.Printf("🔧 Manual trigger: http://localhost:%s/api/v1/analyze", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("✅ Server exited gracefully")
}
