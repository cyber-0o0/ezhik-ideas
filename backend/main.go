package main

import (
	"log"
	"net/http"
	"os"
	"time"

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

func generateIdea(category string) string {
	ideas := map[string][]string{
		"бизнес": {
			"Сервис аренды инструментов для дачи и ремонта",
			"Мобильное приложение для поиска попутчиков в своём городе",
			"Платформа для обмена вещами между соседями",
			"Автоматизированный магазин для малого бизнеса",
			"Доставка продуктов с фермы",
			"Онлайн-курсы по 3D-печати",
		},
		"3D-проект": {
			"Набор 3D-моделей мебели в PSX-стиле для Unity",
			"Виртуальная выставка произведений искусства",
			"Интерактивная 3D-карта своего города",
			"Модели для настольных игр с печатью",
			"Ассеты для Roblox",
			"Персонажи для инди-игр",
			"Процедурный генератор заброшенных зданий",
			"Набор sci-fi терминалов с анимацией",
			"Лоу-поли пак еды и напитков для киоска",
		},
		"контент": {
			"Серия коротких видео про жизнь программиста",
			"Блог о 3D-моделировании для начинающих",
			"Подкаст про крипту простыми словами",
			"Telegram-канал с ежедневными идеями",
			"Обзоры на бесплатные ассеты",
		},
		"приложение": {
			"TODO-лист с геймификацией и достижениями",
			"Трекер привычек с статистикой и мотивацией",
			"Приложение для планирования путешествий",
			"Бот для напоминаний о важных делах",
			"Напоминалка про воду",
		},
		"крипта": {
			"Анализатор портфеля с уведомлениями",
			"Дашборд для отслеживания airdrops",
			"Сервис для изучения DeFi через практику",
			"Портфель NFT-арта с отслеживанием трендов",
			"Калькулятор gas для Ethereum",
			"Бот для мониторинга листинга новых токенов",
			"Симулятор крипто-трейдинга для новичков",
		},
		"сайт": {
			"Лендинг для фрилансера с портфолио",
			"Сайт-визитка для мастера по ремонту",
			"Блог-платформа с минималистичным дизайном",
			"Сервис для создания резюме онлайн",
			"Портфолио для 3D-художника",
		},
	}
	
	categoryIdeas, ok := ideas[category]
	if !ok {
		categoryIdeas = ideas["бизнес"]
	}
	
	now := time.Now()
	idx := (now.UnixNano() / 1e9) % int64(len(categoryIdeas))
	
	return categoryIdeas[idx]
}
