# Ezhik Ideas — Telegram Mini App

## Описание
Telegram Mini App с генератором идей для проектов, бизнеса, контента.

## 🏷️ Версия: **v2.1** (2026-02-21)

### Tech Stack
- **Frontend:** HTML + CSS + Vanilla JS (CSP-safe, без inline-скриптов)
- **Backend:** Go (lightweight)
- **Deployment:** Cloudflare Tunnel / Serveo

### Структура проекта
```
ezhik-ideas/
├── frontend/              # Исходники
│   ├── index.html
│   ├── style.css
│   └── app.js
├── backend/
│   ├── frontend/         # Продакшн-копия
│   ├── main.go
│   └── ezhik-ideas      # Бинарник
├── CHANGELOG.md          # История версий
└── README.md
```

## 🔗 URL
- **Tunnel:** Serveo (обновляется при рестарте)
- **Fallback:** `http://213.176.78.194:8080` (локально)

## Как запустить

```bash
# Backend
cd /root/.openclaw/workspace/ezhik-ideas/backend
./ezhik-ideas &

# Tunnel
cloudflared tunnel --url http://localhost:8080
```

## API Endpoints
- `GET /api/idea?category=xxx` — новая идея (Groq)
- `GET /api/stats` — счётчик идей
- `POST /api/feedback` — лайк/дизлайк
- `GET /api/health` — статус сервера

## ✨ Функции v2.1

| Фича | Описание |
|------|----------|
| **Groq Integration** | Генерация через Groq API |
| **История (localStorage)** | Последние 10 идей сохраняются |
| **Clear History** | Кнопка очистки с подтверждением |
| **Smart API URL** | Автоматический fallback для Mini App |
| **Empty State UI** | Понятное сообщение при пустой истории |
| **Telegram Share** | Кнопка «Поделиться» через WebApp |

## 🛠️ Автоматизация

Heartbeat задачи в `crontab`:
- `moltbook-heartbeat.sh` — каждый час
- `ezhik-ideas-heartbeat.sh` — каждые 3 часа
- `learning-heartbeat.sh` — каждые 4 часа
- и др.

Подробнее: `/root/.openclaw/workspace/HEARTBEAT.md`

---

*Обновлено: 2026-02-21*
*Версия: v2.1*
