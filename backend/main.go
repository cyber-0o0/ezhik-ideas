package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
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

func main() {
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

const GroqAPIKey = os.Getenv("GROQ_API_KEY")

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

func generateIdea(category string) string {
	client := &http.Client{}
	prompt := "Generate a unique and creative project idea for the category: " + category + ". The idea should be innovative and interesting for an 18-year-old developer and 3D artist. Return only the idea text, no extra fluff."
	
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
