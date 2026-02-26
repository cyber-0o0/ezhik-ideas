package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var statsCount int

type Idea struct {
	ID        int    `json:"id"`
	Text      string `json:"idea"`
	Category  string `json:"category"`
}

type Stats struct {
	Count int `json:"count"`
}

type FeedbackRequest struct {
	Idea    string `json:"idea" binding:"required"`
	Feedback string `json:"feedback" binding:"required"`
}

type AIRequest struct {
	Prompt      string `json:"prompt" binding:"required"`
	SystemPrompt string `json:"systemPrompt"`
}

type YouTubeRequest struct {
	URL    string `json:"url" binding:"required"`
	Quality string `json:"quality"` // "worst", "best", "audio"
}

func main() {
	godotenv.Load("/root/.openclaw/workspace/ezhik-ideas/backend/.env")
	// Setup router
	r := gin.Default()
	
	// CORS
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// API routes
	r.GET("/api/idea", getIdea)
	r.GET("/api/stats", getStats)
	r.POST("/api/feedback", sendFeedback)
	r.POST("/api/ai", handleAI)
	r.POST("/api/youtube-dl", downloadYouTube)
	r.POST("/api/code", generateCode)
	
	// Serve frontend
	r.GET("/", func(c *gin.Context) {
		c.File("./frontend/index.html")
	})
	r.GET("/style.css", func(c *gin.Context) {
		c.File("./frontend/style.css")
	})
	r.GET("/app.js", func(c *gin.Context) {
		c.File("./frontend/app.js")
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server started on port %s", port)
	r.Run("0.0.0.0:" + port)
}

func downloadYouTube(c *gin.Context) {
	var req YouTubeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL is required"})
		return
	}

	// Quality mapping
	format := "worst[height<=720]"
	if req.Quality == "best" {
		format = "best[height<=1080]"
	} else if req.Quality == "audio" {
		format = "bestaudio"
	}

	// Create temp directory
	tmpDir := "/tmp/ezhik-yt"
	os.MkdirAll(tmpDir, 0755)
	
	// Generate unique filename
	filename := fmt.Sprintf("video_%d.mp4", time.Now().Unix())
	outputPath := filepath.Join(tmpDir, filename)

	// Build yt-dlp command
	cmd := exec.Command("yt-dlp", 
		"-f", format,
		"-o", outputPath,
		"--no-warnings",
		req.URL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("yt-dlp error: %s", string(output))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download video: " + err.Error()})
		return
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Video file not found"})
		return
	}

	// Open file
	file, err := os.Open(outputPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer file.Close()

	// Set headers for file download
	videoName := filepath.Base(outputPath)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", videoName))
	c.Header("Content-Type", "application/octet-stream")
	
	// Stream file to client
	io.Copy(c.Writer, file)
	
	// Cleanup after sending
	go func() {
		time.Sleep(5 * time.Second)
		os.Remove(outputPath)
	}()
}

func getIdea(c *gin.Context) {
	category := c.DefaultQuery("category", "бизнес")
	ideaText := generateIdea(category)
	statsCount++
	
	c.JSON(http.StatusOK, gin.H{
		"idea":    ideaText,
		"category": category,
	})
}

func getStats(c *gin.Context) {
	c.JSON(http.StatusOK, Stats{Count: statsCount})
}

func sendFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	log.Printf("Feedback: %s - %s", req.Idea, req.Feedback)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func handleAI(c *gin.Context) {
	var req AIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "Ты полезный AI-ассистент. Отвечай на русском языке."
	}
	
	response := callGroq(req.Prompt, systemPrompt)
	c.JSON(http.StatusOK, gin.H{"response": response})
}

func callGroq(prompt, systemPrompt string) string {
	client := &http.Client{}
	
	log.Println("GROQ_API_KEY:", GroqAPIKey)  // Debug
	
	reqBody := GroqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []GroqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	}
	
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return "Error: " + err.Error()
	}
	defer resp.Body.Close()
	
	log.Println("Response status:", resp.StatusCode)  // Debug
	
	var groqResp GroqResponse
	json.NewDecoder(resp.Body).Decode(&groqResp)
	
	if len(groqResp.Choices) > 0 {
		return groqResp.Choices[0].Message.Content
	}
	
	return "No response from AI"
}

var GroqAPIKey string

func init() {
	godotenv.Load()
	GroqAPIKey = os.Getenv("GROQ_API_KEY")
	if GroqAPIKey == "" {
		log.Fatal("GROQ_API_KEY not set in environment or .env file")
	}
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func generateCode(c *gin.Context) {
	var req struct {
		Language string `json:"language"`
		Task     string `json:"task"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	client := &http.Client{}
	prompt := fmt.Sprintf("Generate working code in %s for: %s. Return only the code, no explanations. Include comments if needed. Make it complete and runnable.", req.Language, req.Task)

	reqBody := GroqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []GroqMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Authorization", "Bearer "+GroqAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var groqResp GroqResponse
	json.NewDecoder(resp.Body).Decode(&groqResp)

	if len(groqResp.Choices) > 0 {
		c.JSON(http.StatusOK, gin.H{"code": groqResp.Choices[0].Message.Content})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No response from AI"})
	}
}

func generateIdea(category string) string {
	client := &http.Client{}
	
	var prompt string
	switch category {
	case "psx":
		prompt = "Generate a unique PSX-style 3D asset idea. Think retro low-poly aesthetic from PlayStation 1 era: pixelated textures, affine texture warping, no perspective correction, 16-bit color palette. Suggest specific objects like: retro electronics, vending machines, household items, packaging, street objects. Keep it practical for a solo 3D artist. Return only the idea text, no extra fluff."
	default:
		prompt = "Generate a unique and creative project idea for the category: " + category + ". The idea should be innovative and interesting for an 18-year-old developer and 3D artist. Return only the idea text, no extra fluff."
	}
	
	reqBody := GroqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []GroqMessage{
			{Role: "user", Content: prompt},
		},
	}
	
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+GroqAPIKey)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return "Error generating idea: " + err.Error()
	}
	defer resp.Body.Close()
	
	var groqResp GroqResponse
	json.NewDecoder(resp.Body).Decode(&groqResp)
	
	if len(groqResp.Choices) > 0 {
		return groqResp.Choices[0].Message.Content
	}
	
	return "No idea generated today, try again!"
}
