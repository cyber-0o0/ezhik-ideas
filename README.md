# Ezhik Ideas — Telegram Mini App

## Описание
Telegram Mini App с генератором идей для проектов, бизнеса, контента.

## Архитектура

### Tech Stack
- **Frontend:** HTML + CSS + Vanilla JS (легко, быстро, без зависимостей)
- **Backend:** Go (lightweight)
- **Deployment:** Cloudflare Tunnel (trycloudflare)

### Структура проекта
```
/root/.openclaw/workspace/ezhik-ideas/
├── frontend/
│   ├── index.html
│   ├── style.css
│   └── app.js
├── backend/
│   ├── main.go
│   └── ezhik-ideas (binary)
└── README.md
```

## 🔗 URL (обновляется при перезапуске туннеля)

**Текущий:** https://rider-completely-arrangement-impacts.trycloudflare.com

## Как запустить

```bash
# Запуск сервера
cd /root/.openclaw/workspace/ezhik-ideas/backend
./ezhik-ideas &

# Запуск туннеля
cloudflared tunnel --url http://localhost:8080
```

## API Endpoints
- `GET /api/idea?category=xxx` — получить новую идею
- `GET /api/stats` — статистика
- `POST /api/feedback` — обратная связь

## Функции v1
1. Кнопка "Сгенерировать идею"
2. Категории: Бизнес, 3D, Контент, Приложение, Крипта, Сайт
3. Сохранение истории идей (в памяти)
4. Простой и понятный UI

## Heartbeat
- Работа над проектом: раз в 3-4 часа
- Минимум 1 задача за сессию

---

*Обновлено: 2026-02-21*
*URL: https://likelihood-nylon-living-coupon.trycloudflare.com*
