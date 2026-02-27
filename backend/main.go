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
	
	// Email Builder API
	r.POST("/api/generate", handleEmailGenerate)
	r.POST("/api/ai-generate", handleAIGenerate)
	r.POST("/api/upload", handleImageUpload)
	r.GET("/storage/*path", handleServeImage)
	
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

// ============ EMAIL BUILDER HANDLERS ============

var emailStorage = "./storage"

func init() {
	os.MkdirAll(emailStorage, 0755)
}

type EmailRequest struct {
	Type      string                 `json:"type"`
	Theme     map[string]string     `json:"theme"`
	Blocks    []map[string]interface{} `json:"blocks"`
	Preheader string                `json:"preheader"`
	Subject   string                `json:"subject"`
}

func handleEmailGenerate(c *gin.Context) {
	var req EmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Generate HTML (simplified - returns placeholder)
	html := generateEmailHTML(req)
	
	c.JSON(http.StatusOK, gin.H{"html": html, "id": "email_" + randomString(8)})
}

func handleAIGenerate(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt"`
		Type   string `json:"type"`
	}
	c.ShouldBindJSON(&req)
	
	// Generate from prompt
	emailReq := EmailRequest{
		Type:      req.Type,
		Subject:   "Email",
		Preheader: "Узнайте больше",
		Theme:     map[string]string{"primary": "#1a1a1a", "accent": "#4f6ef7"},
		Blocks: []map[string]interface{}{
			{"type": "header", "data": map[string]interface{}{"logo": "BRAND"}, "enabled": true},
			{"type": "hero", "data": map[string]interface{}{"title": "Заголовок", "description": req.Prompt}, "enabled": true},
		},
	}
	
	html := generateEmailHTML(emailReq)
	c.JSON(http.StatusOK, gin.H{"html": html, "id": "email_" + randomString(8)})
}

func handleImageUpload(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file"})
		return
	}
	defer file.Close()
	
	data, _ := io.ReadAll(file)
	
	// Generate unique name
	ext := filepath.Ext(header.Filename)
	filename := randomString(12) + ext
	
	os.WriteFile(filepath.Join(emailStorage, filename), data, 0644)
	
	c.JSON(http.StatusOK, gin.H{"url": "/storage/" + filename})
}

func handleServeImage(c *gin.Context) {
	path := c.Param("path")
	data, err := os.ReadFile(filepath.Join(emailStorage, filepath.Clean(path)))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	
	ext := filepath.Ext(path)
	contentType := "application/octet-stream"
	if ext == ".jpg" || ext == ".jpeg" {
		contentType = "image/jpeg"
	} else if ext == ".png" {
		contentType = "image/png"
	} else if ext == ".gif" {
		contentType = "image/gif"
	}
	
	c.Data(http.StatusOK, contentType, data)
}

func generateEmailHTML(req EmailRequest) string {
	// Simple HTML generation based on blocks
	bg := "#f0f0f0"
	primary := "#1a1a1a"
	accent := "#4f6ef7"
	
	if req.Theme != nil {
		if v, ok := req.Theme["background"]; ok {
			bg = v
		}
		if v, ok := req.Theme["primary"]; ok {
			primary = v
		}
		if v, ok := req.Theme["accent"]; ok {
			accent = v
		}
	}
	
	subject := req.Subject
	if subject == "" {
		subject = "Email"
	}
	
	preheader := req.Preheader
	if preheader == "" {
		preheader = "Узнайте больше"
	}
	
	html := `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.0 Transitional//EN">
<html lang="ru">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>` + subject + `</title>
<style>
body, table, td { font-family: Arial, Helvetica, sans-serif; }
</style>
</head>
<body style="margin:0; padding:0; background-color:` + bg + `;">
<div style="font-size:0; color:` + bg + `;">` + preheader + `&nbsp;&nbsp;</div>
<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%" style="background-color:` + bg + `;">
<tr><td align="center" style="padding:28px 15px;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width:600px;">`
	
	for _, block := range req.Blocks {
		enabled, _ := block["enabled"].(bool)
		if !enabled {
			continue
		}
		
		blockType, _ := block["type"].(string)
		data, _ := block["data"].(map[string]interface{})
		
		switch blockType {
		case "header":
			logo := "BRAND"
			if v, ok := data["logo"].(string); ok {
				logo = v
			}
			html += `<tr><td style="background:#0d1f3c; color:white; padding:22px 32px; font-size:20px; font-weight:bold;">` + logo + `</td></tr>`
		
		case "hero":
			title := "Заголовок"
			desc := "Описание"
			if v, ok := data["title"].(string); ok {
				title = v
			}
			if v, ok := data["description"].(string); ok {
				desc = v
			}
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:28px; font-weight:bold; color:` + primary + `; margin-bottom:16px;">` + title + `</div>
			<div style="color:#666; margin-bottom:24px;">` + desc + `</div>
			</td></tr>`
		
		case "text":
			content := "Текст"
			if v, ok := data["content"].(string); ok {
				content = v
			}
			html += `<tr><td style="background:white; padding:24px 32px;">` + content + `</td></tr>`
		
		case "button":
			text := "Кнопка"
			if v, ok := data["text"].(string); ok {
				text = v
			}
			html += `<tr><td style="background:white; padding:0 32px 32px; text-align:center;">
			<a href="#" style="display:inline-block; background:` + accent + `; color:white; padding:14px 28px; text-decoration:none; border-radius:4px;">` + text + `</a>
			</td></tr>`

		case "divider":
			color := "#e0e0e0"
			if v, ok := data["color"].(string); ok {
				color = v
			}
			html += `<tr><td style="padding:16px 32px;"><div style="border-top:1px solid ` + color + `;"></div></td></tr>`

		case "cta":
			title := "Заголовок CTA"
			desc := "Описание"
			btnText := "Нажать"
			btnLink := "#"
			icon := "→"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["description"].(string); ok { desc = v }
			if v, ok := data["button_text"].(string); ok { btnText = v }
			if v, ok := data["button_link"].(string); ok { btnLink = v }
			if v, ok := data["icon"].(string); ok { icon = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:18px; font-weight:bold; color:`+primary+`; margin-bottom:8px;">`+title+`</div>
			<div style="color:#666; margin-bottom:20px;">`+desc+`</div>
			<a href="`+btnLink+`" style="display:inline-block; background:`+accent+`; color:white; padding:14px 28px; text-decoration:none; border-radius:4px; font-weight:bold;">`+btnText+` `+icon+`</a>
			</td></tr>`

		case "quote":
			text := "Отзыв или цитата"
			author := "Автор"
			if v, ok := data["text"].(string); ok { text = v }
			if v, ok := data["author"].(string); ok { author = v }
			html += `<tr><td style="background:#f9f9f9; padding:32px; text-align:center;">
			<div style="font-size:16px; color:#333; font-style:italic; line-height:24px;">“`+text+`”</div>
			<div style="font-size:14px; color:#666; margin-top:16px; font-weight:bold;">— `+author+`</div>
			</td></tr>`

		case "event":
			title := "Вебинар"
			date := "15 марта 2026"
			time := "18:00 МСК"
			btnText := "Зарегистрироваться"
			btnLink := "#"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["date"].(string); ok { date = v }
			if v, ok := data["time"].(string); ok { time = v }
			if v, ok := data["button_text"].(string); ok { btnText = v }
			if v, ok := data["button_link"].(string); ok { btnLink = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:14px; color:#999; text-transform:uppercase; margin-bottom:8px;">Событие</div>
			<div style="font-size:22px; font-weight:bold; color:`+primary+`; margin-bottom:16px;">`+title+`</div>
			<div style="font-size:16px; color:#333; margin-bottom:8px;">📅 `+date+` · ⏰ `+time+`</div>
			<a href="`+btnLink+`" style="display:inline-block; background:`+accent+`; color:white; padding:14px 28px; text-decoration:none; border-radius:4px; font-weight:bold; margin-top:16px;">`+btnText+`</a>
			</td></tr>`

		case "stats":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					map[string]interface{}{"value": "10K+", "label": "Пользователей"},
					map[string]interface{}{"value": "99%", "label": "Uptime"},
					map[string]interface{}{"value": "24/7", "label": "Поддержка"},
				}
			}
			html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>`
			for i, item := range items {
				if i > 0 { html += `<td style="width:16px;"></td>` }
				itemMap, _ := item.(map[string]interface{})
				value := getString(itemMap, "value", "0")
				label := getString(itemMap, "label", "Метрика")
				html += `<td align="center" width="180"><div style="font-size:28px; font-weight:bold; color:`+primary+`;">`+value+`</div><div style="font-size:14px; color:#666; margin-top:4px;">`+label+`</div></td>`
			}
			html += `</tr></table></td></tr>`

		case "faq":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					map[string]interface{}{"question": "Как это работает?", "answer": "Очень просто!"},
					map[string]interface{}{"question": "Сколько стоит?", "answer": "Есть бесплатный тариф"},
				}
			}
			html += `<tr><td style="background:white; padding:32px;">`
			for _, item := range items {
				itemMap, _ := item.(map[string]interface{})
				q := getString(itemMap, "question", "Вопрос")
				a := getString(itemMap, "answer", "Ответ")
				html += `<div style="margin-bottom:16px;"><div style="font-size:16px; font-weight:bold; color:`+primary+`; margin-bottom:4px;">❓ `+q+`</div><div style="font-size:14px; color:#666; line-height:20px;">`+a+`</div></div>`
			}
			html += `</td></tr>`

		case "video":
			title := "Видео"
			desc := "Описание видео"
			thumbnail := ""
			videoLink := "https://youtube.com"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["description"].(string); ok { desc = v }
			if v, ok := data["thumbnail"].(string); ok { thumbnail = v }
			if v, ok := data["link"].(string); ok { videoLink = v }
			playBtn := `<div style="width:60px; height:60px; background:rgba(0,0,0,0.7); border-radius:50%; display:inline-block; text-align:center; line-height:60px; color:white; font-size:24px;">▶</div>`
			if thumbnail != "" {
				html += `<tr><td style="background:white; padding:32px; text-align:center;">
				<a href="`+videoLink+`" style="display:inline-block; position:relative;">
				<img src="`+thumbnail+`" width="500" height="280" style="display:block; border-radius:8px;">
				<div style="position:absolute; top:50%; left:50%; transform:translate(-50%,-50%);">`+playBtn+`</div>
				</a>
				<div style="font-size:18px; font-weight:bold; color:`+primary+`; margin-top:16px;">`+title+`</div>
				<div style="color:#666; margin-top:8px;">`+desc+`</div>
				</td></tr>`
			} else {
				html += `<tr><td style="background:white; padding:32px; text-align:center;">
				<a href="`+videoLink+`" style="display:inline-block;">`+playBtn+`</a>
				<div style="font-size:18px; font-weight:bold; color:`+primary+`; margin-top:16px;">`+title+`</div>
				<div style="color:#666; margin-top:8px;">`+desc+`</div>
				</td></tr>`
			}

		case "gallery":
			images, _ := data["images"].([]interface{})
			if len(images) == 0 {
				images = []interface{}{
					"https://via.placeholder.com/300x200",
					"https://via.placeholder.com/300x200",
					"https://via.placeholder.com/300x200",
				}
			}
			html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>`
			for i, img := range images {
				if i > 0 && i%3 == 0 { html += `</tr><tr>` }
				if i%3 > 0 { html += `<td style="width:8px;"></td>` }
				html += `<td align="center" width="180"><img src="`+img.(string)+`" width="180" height="120" style="display:block; border-radius:4px;"></td>`
			}
			html += `</tr></table></td></tr>`

		case "countdown":
			title := "До конца акции осталось"
			days := "03"
			hours := "12"
			minutes := "45"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["days"].(string); ok { days = v }
			if v, ok := data["hours"].(string); ok { hours = v }
			if v, ok := data["minutes"].(string); ok { minutes = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:16px; color:#666; margin-bottom:16px;">`+title+`</div>
			<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>
			<td align="center" width="80"><div style="font-size:32px; font-weight:bold; color:`+primary+`;">`+days+`</div><div style="font-size:12px; color:#999;">дней</div></td>
			<td align="center" width="40"><div style="font-size:32px; color:#ccc;">:</div></td>
			<td align="center" width="80"><div style="font-size:32px; font-weight:bold; color:`+primary+`;">`+hours+`</div><div style="font-size:12px; color:#999;">часов</div></td>
			<td align="center" width="40"><div style="font-size:32px; color:#ccc;">:</div></td>
			<td align="center" width="80"><div style="font-size:32px; font-weight:bold; color:`+primary+`;">`+minutes+`</div><div style="font-size:12px; color:#999;">минут</div></td>
			</tr></table>
			</td></tr>`

		case "banner":
			title := "Заголовок баннера"
			desc := "Описание"
			bg := "#1a1a2e"
			btnText := "Кнопка"
			btnLink := "#"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["description"].(string); ok { desc = v }
			if v, ok := data["background"].(string); ok { bg = v }
			if v, ok := data["button_text"].(string); ok { btnText = v }
			if v, ok := data["button_link"].(string); ok { btnLink = v }
			html += `<tr><td style="background:`+bg+`; padding:48px 32px; text-align:center;">
			<div style="font-size:28px; font-weight:bold; color:white; margin-bottom:12px;">`+title+`</div>
			<div style="font-size:16px; color:rgba(255,255,255,0.8); margin-bottom:24px;">`+desc+`</div>
			<a href="`+btnLink+`" style="display:inline-block; background:white; color:`+bg+`; padding:14px 32px; text-decoration:none; border-radius:4px; font-weight:bold;">`+btnText+`</a>
			</td></tr>`

		case "features":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					map[string]interface{}{"icon": "🚀", "title": "Быстро", "desc": "Работает мгновенно"},
					map[string]interface{}{"icon": "🔒", "title": "Безопасно", "desc": "Ваши данные защищены"},
					map[string]interface{}{"icon": "💎", "title": "Качественно", "desc": "Лучшие материалы"},
				}
			}
			html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>`
			for i, item := range items {
				if i > 0 && i%3 == 0 { html += `</tr><tr>` }
				if i%3 > 0 { html += `<td style="width:16px;"></td>` }
				itemMap, _ := item.(map[string]interface{})
				icon := getString(itemMap, "icon", "✓")
				title := getString(itemMap, "title", "Фича")
				desc := getString(itemMap, "desc", "Описание")
				html += `<td align="center" valign="top" width="180">
				<div style="font-size:32px; margin-bottom:8px;">`+icon+`</div>
				<div style="font-size:16px; font-weight:bold; color:`+primary+`; margin-bottom:4px;">`+title+`</div>
				<div style="font-size:13px; color:#666; line-height:18px;">`+desc+`</div>
				</td>`
			}
			html += `</tr></table></td></tr>`

		case "pricing":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					map[string]interface{}{"name": "Базовый", "price": "990₽", "period": "/мес", "features": "• 1 проект\n• Базовая поддержка"},
					map[string]interface{}{"name": "Pro", "price": "2990₽", "period": "/мес", "features": "• 5 проектов\n• Приоритет", "highlight": true},
					map[string]interface{}{"name": "Бизнес", "price": "9900₽", "period": "/мес", "features": "• Безлимит\n• 24/7 поддержка"},
				}
			}
			html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>`
			for i, item := range items {
				if i > 0 { html += `<td style="width:16px;"></td>` }
				itemMap, _ := item.(map[string]interface{})
				name := getString(itemMap, "name", "Тариф")
				price := getString(itemMap, "price", "0₽")
				period := getString(itemMap, "period", "")
				features := getString(itemMap, "features", "")
				highlight, _ := itemMap["highlight"].(bool)
				border := "1px solid #e0e0e0"
				if highlight { border = "2px solid " + accent }
				html += `<td align="center" valign="top" width="180" style="border:`+border+`; border-radius:8px; padding:24px 16px;">
				<div style="font-size:14px; color:#666; margin-bottom:8px;">`+name+`</div>
				<div style="font-size:28px; font-weight:bold; color:`+primary+`;">`+price+`<span style="font-size:12px; color:#999;">`+period+`</span></div>
				<div style="font-size:12px; color:#666; margin-top:16px; line-height:20px; white-space:pre-line;">`+features+`</div>
				</td>`
			}
			html += `</tr></table></td></tr>`

		case "spacer":
			height := "32"
			if v, ok := data["height"].(string); ok { height = v }
			html += `<tr><td style="font-size:0; height:`+height+`px; line-height:`+height+`px;">&nbsp;</td></tr>`

		case "columns":
			image := ""
			title := "Заголовок"
			content := "Текст"
			imageSide := "right"
			if v, ok := data["image"].(string); ok { image = v }
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["content"].(string); ok { content = v }
			if v, ok := data["imageSide"].(string); ok { imageSide = v }
			imgHTML := ""
			if image != "" {
				imgHTML = `<td align="center" valign="middle" width="260" style="padding:24px;"><img src="`+image+`" width="260" height="180" style="display:block; border-radius:4px;"></td>`
			}
			if imageSide == "left" {
				html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>` + imgHTML + `<td align="left" valign="middle" style="padding:24px;"><div style="font-size:20px; font-weight:bold; color:`+primary+`; margin-bottom:12px;">`+title+`</div><div style="font-size:14px; color:#666; line-height:22px;">`+content+`</div></td></tr></table></td></tr>`
			} else {
				html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr><td align="left" valign="middle" style="padding:24px;"><div style="font-size:20px; font-weight:bold; color:`+primary+`; margin-bottom:12px;">`+title+`</div><div style="font-size:14px; color:#666; line-height:22px;">`+content+`</div></td>` + imgHTML + `</tr></table></td></tr>`
			}

		case "alert":
			text := "Важное сообщение"
			alertType := "info"
			if v, ok := data["text"].(string); ok { text = v }
			if v, ok := data["type"].(string); ok { alertType = v }
			bg := "#e3f2fd"
			color := "#1565c0"
			icon := "ℹ️"
			if alertType == "success" { bg = "#e8f5e9"; color = "#2e7d32"; icon = "✅" }
			if alertType == "warning" { bg = "#fff3e0"; color = "#ef6c00"; icon = "⚠️" }
			if alertType == "error" { bg = "#ffebee"; color = "#c62828"; icon = "❌" }
			html += `<tr><td style="background:`+bg+`; padding:16px 24px; border-radius:8px; margin:16px 32px;">
			<span style="font-size:16px;">`+icon+`</span> <span style="color:`+color+`; margin-left:8px;">`+text+`</span>
			</td></tr>`

		case "image":
			src := ""
			alt := "Изображение"
			caption := ""
			if v, ok := data["src"].(string); ok { src = v }
			if v, ok := data["alt"].(string); ok { alt = v }
			if v, ok := data["caption"].(string); ok { caption = v }
			html += `<tr><td style="background:white; padding:16px 32px; text-align:center;">`
			if src != "" {
				html += `<img src="`+src+`" alt="`+alt+`" style="max-width:100%; height:auto; border-radius:4px;">`
			}
			if caption != "" {
				html += `<div style="font-size:12px; color:#999; margin-top:8px;">`+caption+`</div>`
			}
			html += `</td></tr>`

		case "html":
			content := ""
			if v, ok := data["content"].(string); ok { content = v }
			html += `<tr><td style="background:white; padding:16px 32px;">` + content + `</td></tr>`

		case "form":
			title := "Оставьте email"
			placeholder := "Ваш email"
			buttonText := "Отправить"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["placeholder"].(string); ok { placeholder = v }
			if v, ok := data["button"].(string); ok { buttonText = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:18px; font-weight:bold; color:`+primary+`; margin-bottom:16px;">`+title+`</div>
			<form style="margin:0;">
			<input type="email" placeholder="`+placeholder+`" style="width:70%; padding:12px; border:1px solid #ddd; border-radius:4px; font-size:14px;">
			<button type="submit" style="width:25%; padding:12px; background:`+accent+`; color:white; border:none; border-radius:4px; font-size:14px; font-weight:bold; cursor:pointer;">`+buttonText+`</button>
			</form>
			</td></tr>`

		case "badge":
			text := "NEW"
			badgeType := "new"
			if v, ok := data["text"].(string); ok { text = v }
			if v, ok := data["type"].(string); ok { badgeType = v }
			bg := "#2196f3"
			if badgeType == "sale" { bg = "#f44336" }
			if badgeType == "hot" { bg = "#ff9800" }
			if badgeType == "popular" { bg = "#9c27b0" }
			if badgeType == "success" { bg = "#4caf50" }
			html += `<tr><td style="background:white; padding:16px 32px; text-align:center;">
			<span style="display:inline-block; padding:6px 16px; background:`+bg+`; color:white; font-size:12px; font-weight:bold; border-radius:20px; text-transform:uppercase;">`+text+`</span>
			</td></tr>`

		case "list":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					"✓ Преимущество 1",
					"✓ Преимущество 2",
					"✓ Преимущество 3",
				}
			}
			html += `<tr><td style="background:white; padding:24px 32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">`
			for _, item := range items {
				itemStr, _ := item.(string)
				html += `<tr><td style="padding:8px 0; font-size:14px; color:#333; line-height:20px;">` + itemStr + `</td></tr>`
			}
			html += `</table></td></tr>`

		case "survey":
			question := "Как вам наш сервис?"
			if v, ok := data["question"].(string); ok { question = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:16px; color:#333; margin-bottom:16px;">`+question+`</div>
			<div>
			<span style="display:inline-block; padding:8px 16px; margin:4px; border:1px solid #ddd; border-radius:4px; cursor:pointer;">😟</span>
			<span style="display:inline-block; padding:8px 16px; margin:4px; border:1px solid #ddd; border-radius:4px; cursor:pointer;">😐</span>
			<span style="display:inline-block; padding:8px 16px; margin:4px; border:1px solid #ddd; border-radius:4px; cursor:pointer;">🙂</span>
			<span style="display:inline-block; padding:8px 16px; margin:4px; border:1px solid #ddd; border-radius:4px; cursor:pointer;">😍</span>
			</div>
			</td></tr>`

		case "download":
			title := "Скачать приложение"
			iosLink := "#"
			androidLink := "#"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["ios"].(string); ok { iosLink = v }
			if v, ok := data["android"].(string); ok { androidLink = v }
			html += `<tr><td style="background:white; padding:32px; text-align:center;">
			<div style="font-size:18px; font-weight:bold; color:`+primary+`; margin-bottom:16px;">`+title+`</div>
			<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>
			<td align="center" width="200"><a href="`+iosLink+`" style="display:inline-block; background:#000; color:white; padding:12px 20px; border-radius:8px; text-decoration:none; font-size:14px;"> App Store</a></td>
			<td align="center" width="200"><a href="`+androidLink+`" style="display:inline-block; background:#000; color:white; padding:12px 20px; border-radius:8px; text-decoration:none; font-size:14px;">▶ Google Play</a></td>
			</tr></table>
			</td></tr>`

		case "footer2":
			company := "Компания"
			email := "hello@example.com"
			phone := "+7 (999) 123-45-67"
			address := "Москва, ул. Примерная 1"
			if v, ok := data["company"].(string); ok { company = v }
			if v, ok := data["email"].(string); ok { email = v }
			if v, ok := data["phone"].(string); ok { phone = v }
			if v, ok := data["address"].(string); ok { address = v }
			html += `<tr><td style="background:#f5f5f5; padding:32px; text-align:center;">
			<div style="font-size:14px; color:#666; margin-bottom:8px;">`+company+`</div>
			<div style="font-size:12px; color:#999; margin-bottom:4px;">📍 `+address+`</div>
			<div style="font-size:12px; color:#999; margin-bottom:4px;">📧 <a href="mailto:`+email+`" style="color:#666;">`+email+`</a></div>
			<div style="font-size:12px; color:#999; margin-bottom:16px;">📞 <a href="tel:`+phone+`" style="color:#666;">`+phone+`</a></div>
			<div style="font-size:11px; color:#ccc;"><a href="{{unsubscribe}}" style="color:#999;">Отписаться от рассылки</a></div>
			</td></tr>`

		case "steps":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					"Шаг 1: Зарегистрируйтесь",
					"Шаг 2: Настройте профиль",
					"Шаг 3: Начните использовать",
				}
			}
			html += `<tr><td style="background:white; padding:32px;">`
			for i, item := range items {
				itemStr, _ := item.(string)
				html += `<div style="margin-bottom:16px;"><span style="display:inline-block; width:28px; height:28px; background:` + accent + `; color:white; border-radius:50%; text-align:center; line-height:28px; font-size:14px; font-weight:bold; margin-right:12px;">` + fmt.Sprintf("%d", i+1) + `</span><span style="font-size:14px; color:#333; vertical-align:middle;">` + itemStr + `</span></div>`
			}
			html += `</td></tr>`

		case "cards":
			items, _ := data["items"].([]interface{})
			if len(items) == 0 {
				items = []interface{}{
					map[string]interface{}{"title": "Карточка 1", "desc": "Описание 1"},
					map[string]interface{}{"title": "Карточка 2", "desc": "Описание 2"},
					map[string]interface{}{"title": "Карточка 3", "desc": "Описание 3"},
				}
			}
			html += `<tr><td style="background:white; padding:32px;"><table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>`
			for i, item := range items {
				if i > 0 { html += `<td style="width:16px;"></td>` }
				itemMap, _ := item.(map[string]interface{})
				title := getString(itemMap, "title", "Заголовок")
				desc := getString(itemMap, "desc", "Описание")
				html += `<td valign="top" width="180" style="border:1px solid #e0e0e0; border-radius:8px; padding:16px;">
				<div style="font-size:16px; font-weight:bold; color:` + primary + `; margin-bottom:8px;">` + title + `</div>
				<div style="font-size:13px; color:#666; line-height:18px;">` + desc + `</div>
				</td>`
			}
			html += `</tr></table></td></tr>`

		case "testimonial":
			name := "Иван Иванов"
			text := "Отличный сервис! Всё работает."
			avatar := "https://via.placeholder.com/60"
			role := "Клиент"
			if v, ok := data["name"].(string); ok { name = v }
			if v, ok := data["text"].(string); ok { text = v }
			if v, ok := data["avatar"].(string); ok { avatar = v }
			if v, ok := data["role"].(string); ok { role = v }
			html += `<tr><td style="background:#f9f9f9; padding:32px; text-align:center;">
			<img src="` + avatar + `" width="60" height="60" style="border-radius:50%; display:inline-block; margin-bottom:12px;">
			<div style="font-size:14px; color:#666; font-style:italic; margin-bottom:12px;">"` + text + `"</div>
			<div style="font-size:14px; font-weight:bold; color:` + primary + `;">` + name + `</div>
			<div style="font-size:12px; color:#999;">` + role + `</div>
			</td></tr>`

		case "stars":
			rating := "5"
			if v, ok := data["rating"].(string); ok { rating = v }
			html += `<tr><td style="background:white; padding:24px; text-align:center;">
			<div style="font-size:32px; margin-bottom:8px;">`
			for i := 0; i < 5; i++ {
				if i < 3 {
					html += `⭐`
				} else {
					html += `☆`
				}
			}
			html += `</div>
			<div style="font-size:14px; color:#666;">Оценка: ` + rating + `/5</div>
			</td></tr>`

		case "progress":
			current := "2"
			total := "3"
			title := "Шаг 2 из 3"
			if v, ok := data["current"].(string); ok { current = v }
			if v, ok := data["total"].(string); ok { total = v }
			if v, ok := data["title"].(string); ok { title = v }
			percent := 100 * (current[0]-'0') / (total[0]-'0')
			html += `<tr><td style="background:white; padding:24px 32px;">
			<div style="font-size:14px; color:#666; margin-bottom:8px;">` + title + `</div>
			<div style="width:100%; height:8px; background:#e0e0e0; border-radius:4px;">
			<div style="width:` + fmt.Sprintf("%d", percent) + `%; height:8px; background:` + accent + `; border-radius:4px;"></div>
			</div>
			</td></tr>`

		case "gift":
			title := "Подарок для вас!"
			desc := "Зарегистрируйтесь и получите бонус"
			icon := "🎁"
			if v, ok := data["title"].(string); ok { title = v }
			if v, ok := data["description"].(string); ok { desc = v }
			if v, ok := data["icon"].(string); ok { icon = v }
			html += `<tr><td style="background:linear-gradient(135deg, #667eea 0%, #764ba2 100%); padding:40px 32px; text-align:center;">
			<div style="font-size:48px; margin-bottom:16px;">` + icon + `</div>
			<div style="font-size:24px; font-weight:bold; color:white; margin-bottom:8px;">` + title + `</div>
			<div style="font-size:16px; color:rgba(255,255,255,0.9);">` + desc + `</div>
			</td></tr>`

		case "logo":
			src := ""
			link := "#"
			if v, ok := data["src"].(string); ok { src = v }
			if v, ok := data["link"].(string); ok { link = v }
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<a href="` + link + `">`
			if src != "" {
				html += `<img src="` + src + `" alt="Logo" style="max-width:200px; height:auto;">`
			} else {
				html += `<div style="font-size:24px; font-weight:bold; color:` + primary + `;">LOGO</div>`
			}
			html += `</a></td></tr>`

		case "share":
			text := "Поделиться"
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<div style="font-size:14px; color:#666; margin-bottom:12px;">` + text + `</div>
			<a href="#" style="display:inline-block; margin:0 8px; width:40px; height:40px; background:#4267B2; border-radius:50%; line-height:40px; color:white; text-decoration:none;">f</a>
			<a href="#" style="display:inline-block; margin:0 8px; width:40px; height:40px; background:#1DA1F2; border-radius:50%; line-height:40px; color:white; text-decoration:none;">t</a>
			<a href="#" style="display:inline-block; margin:0 8px; width:40px; height:40px; background:#0077B5; border-radius:50%; line-height:40px; color:white; text-decoration:none;">in</a>
			<a href="#" style="display:inline-block; margin:0 8px; width:40px; height:40px; background:#E4405F; border-radius:50%; line-height:40px; color:white; text-decoration:none;">ig</a>
			</td></tr>`

		case "qr":
			link := "https://example.com"
			size := "120"
			if v, ok := data["link"].(string); ok { link = v }
			if v, ok := data["size"].(string); ok { size = v }
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<img src="https://api.qrserver.com/v1/create-qr-code/?size=` + size + `x` + size + `&data=` + link + `" width="` + size + `" height="` + size + `" alt="QR">
			</td></tr>`

		case "seal":
			text := "СЕРТИФИКАТ"
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<div style="display:inline-block; width:120px; height:120px; border:4px solid #d4af37; border-radius:50%; display:flex; align-items:center; justify-content:center; transform:rotate(-15deg);">
			<div style="text-align:center;">
			<div style="font-size:14px; font-weight:bold; color:#d4af37; text-transform:uppercase;">` + text + `</div>
			<div style="font-size:10px; color:#d4af37; margin-top:4px;">✓</div>
			</div>
			</div>
			</td></tr>`

		case "timer":
			days := "02"
			hours := "12"
			minutes := "30"
			seconds := "45"
			if v, ok := data["days"].(string); ok { days = v }
			if v, ok := data["hours"].(string); ok { hours = v }
			if v, ok := data["minutes"].(string); ok { minutes = v }
			if v, ok := data["seconds"].(string); ok { seconds = v }
			html += `<tr><td style="background:#1a1a2e; padding:32px; text-align:center;">
			<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%"><tr>
			<td align="center"><div style="font-size:36px; font-weight:bold; color:white;">` + days + `</div><div style="font-size:12px; color:#888;">дней</div></td>
			<td align="center"><div style="font-size:24px; color:#555;">:</div></td>
			<td align="center"><div style="font-size:36px; font-weight:bold; color:white;">` + hours + `</div><div style="font-size:12px; color:#888;">часов</div></td>
			<td align="center"><div style="font-size:24px; color:#555;">:</div></td>
			<td align="center"><div style="font-size:36px; font-weight:bold; color:white;">` + minutes + `</div><div style="font-size:12px; color:#888;">минут</div></td>
			<td align="center"><div style="font-size:24px; color:#555;">:</div></td>
			<td align="center"><div style="font-size:36px; font-weight:bold; color:` + accent + `;">` + seconds + `</div><div style="font-size:12px; color:#888;">секунд</div></td>
			</tr></table>
			</td></tr>`

		case "barcode":
			code := "1234567890"
			if v, ok := data["code"].(string); ok { code = v }
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<img src="https://barcode.tec-it.com/barcode.png?data=` + code + `" alt="Barcode">
			</td></tr>`

		case "instagram":
			image := "https://via.placeholder.com/400x400"
			likes := "1,234"
			html += `<tr><td style="background:white; padding:24px 32px;">
			<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
			<tr><td align="center"><img src="` + image + `" width="400" height="400" style="display:block; border-radius:8px;"></td></tr>
			<tr><td align="center" style="padding:12px 0; color:#666; font-size:14px;">❤ ` + likes + `</td></tr>
			</table>
			</td></tr>`

		case "telegram":
			name := "Канал"
			desc := "Описание канала"
			members := "1,000"
			html += `<tr><td style="background:white; padding:24px 32px; text-align:center;">
			<div style="width:60px; height:60px; background:#229ED9; border-radius:50%; display:inline-flex; align-items:center; justify-content:center; margin-bottom:12px;">
			<span style="color:white; font-size:28px;">✈</span>
			</div>
			<div style="font-size:16px; font-weight:bold; color:` + primary + `; margin-bottom:4px;">` + name + `</div>
			<div style="font-size:14px; color:#666; margin-bottom:8px;">` + desc + `</div>
			<div style="font-size:12px; color:#999;">👥 ` + members + ` подписчиков</div>
			</td></tr>`

		case "youtube":
			videoID := "dQw4w9WgXcQ"
			title := "Видео"
			if v, ok := data["videoId"].(string); ok { videoID = v }
			if v, ok := data["title"].(string); ok { title = v }
			html += `<tr><td style="background:white; padding:24px 32px;">
			<a href="https://youtube.com/watch?v=` + videoID + `" target="_blank" style="display:block; position:relative;">
			<img src="https://img.youtube.com/vi/` + videoID + `/maxresdefault.jpg" alt="` + title + `" style="width:100%; max-width:536px; display:block; border-radius:8px;">
			<div style="position:absolute; top:50%; left:50%; transform:translate(-50%,-50%); width:68px; height:48px; background:rgba(0,0,0,0.8); border-radius:8px; display:flex; align-items:center; justify-content:center;">
			<div style="width:0; height:0; border-top:10px solid transparent; border-bottom:10px solid transparent; border-left:18px solid white; margin-left:4px;"></div>
			</div>
			</a>
			</td></tr>`

		case "spotify":
			track := "Название трека"
			artist := "Исполнитель"
			html += `<tr><td style="background:#191414; padding:16px 24px;">
			<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%">
			<tr>
			<td width="56" style="padding-right:12px;"><div style="width:56px; height:56px; background:#1DB954; border-radius:4px; display:flex; align-items:center; justify-content:center;"><span style="color:white; font-size:24px;">♪</span></div></td>
			<td><div style="color:white; font-size:14px; font-weight:bold;">` + track + `</div><div style="color:#b3b3b3; font-size:12px;">` + artist + `</div></td>
			</tr>
			</table>
			</td></tr>`

		case "discord":
			name := "Discord сервер"
			members := "1,000"
			html += `<tr><td style="background:#5865F2; padding:24px 32px; text-align:center;">
			<div style="font-size:32px; margin-bottom:8px;">💬</div>
			<div style="font-size:18px; font-weight:bold; color:white; margin-bottom:4px;">` + name + `</div>
			<div style="font-size:14px; color:rgba(255,255,255,0.8); margin-bottom:12px;">👥 ` + members + ` участников</div>
			<a href="#" style="display:inline-block; background:white; color:#5865F2; padding:10px 24px; border-radius:4px; text-decoration:none; font-weight:bold;">Присоединиться</a>
			</td></tr>`

		case "social":
			networks := []map[string]interface{}{
				{"type": "telegram", "link": "https://t.me/example"},
				{"type": "vk", "link": "https://vk.com/example"},
				{"type": "instagram", "link": "https://instagram.com/example"},
			}
			if v, ok := data["networks"].([]interface{}); ok {
				networks = make([]map[string]interface{}, len(v))
				for i, n := range v {
					if nm, ok := n.(map[string]interface{}); ok {
						networks[i] = nm
					}
				}
			}
			html += `<tr><td style="background:#1a1a2e; padding:16px; text-align:center;">`
			for _, n := range networks {
				networkType := "link"
				link := "https://example.com"
				if t, ok := n["type"].(string); ok {
					networkType = t
				}
				if l, ok := n["link"].(string); ok {
					link = l
				}
				var iconAlt string
				switch networkType {
				case "telegram":
					iconAlt = "Telegram"
				case "vk":
					iconAlt = "ВКонтакте"
				case "instagram":
					iconAlt = "Instagram"
				case "whatsapp":
					iconAlt = "WhatsApp"
				case "youtube":
					iconAlt = "YouTube"
				default:
					iconAlt = networkType
				}
				html += `<a href="` + link + `" target="_blank" style="display:inline-block; margin:0 8px;"><span style="display:inline-block; width:32px; height:32px; background:#3a4a5a; color:white; border-radius:50%; line-height:32px; font-size:14px;">` + iconAlt + `</span></a>`
			}
			html += `</td></tr>`
		}
	}
	
	html += `<tr><td style="background:#1a1a2e; color:#6a7a8a; padding:28px 32px; text-align:center; font-size:12px;">
	© 2026 Компания · <a href="#" style="color:#4a5a6a;">Отписаться</a>
	</td></tr>
	</table></td></tr></table></body></html>`
	
	return html
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func getString(data map[string]interface{}, key, def string) string {
	if val, ok := data[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return def
}
