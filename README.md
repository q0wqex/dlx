# DLX — Video Downloader

**DLX** — быстрый, легковесный и минималистичный self-hosted веб-сервис для скачивания видео и аудио из интернета.

Сервис представляет собой удобную веб-обёртку над официальным **yt-dlp** и **ffmpeg** с чистым современным интерфейсом в стиле Cobalt, минимальным количеством действий пользователя и мгновенным получением готового файла.

---

## Возможности

- **Минимальный путь до файла**: вставил ссылку → нажал «Скачать» → получил готовый файл.
- **Поддержка любых источников yt-dlp**: YouTube, YouTube Shorts, TikTok, Instagram Reels, X/Twitter, Reddit, Twitch, Vimeo, VK и сотни других платформ.
- **Интеграция с буфером обмена**: быстрая кнопка «Вставить» через Clipboard API (`navigator.clipboard.readText()`) и поддержка стандартных горячих клавиш.
- **Автоматический предпросмотр**: отображение обложки, названия, длительности и платформы без лишних обязательных шагов.
- **Опциональные настройки**: выбор формата (MP4 / MP3), разрешения (Лучшее, 4K, 2K, 1080p, 720p, 480p, 360p), встраивание субтитров и обложек с сохранением в `localStorage`.
- **Реальное отслеживание прогресса**: отображение процентов, скорости загрузки, ETA и фаз обработки через Server-Sent Events (SSE).
- **Стриминг файлов без утечек памяти**: видео передаётся напрямую из временного файла без загрузки в RAM сервера (`http.ServeContent`).
- **Безопасность и стабильность**: защита от SSRF (блокировка локальных и приватных сетей), ограничение параллельных загрузок через семафор, автоматический cleanup временных каталогов при старте и завершении.
- **Встроенный фронтенд**: HTML, CSS и JS скомпилированы прямо в Go-бинарник через `go:embed`.

---

## Архитектура развёртывания

```
Internet
   ↓
Nginx (network_mode: host, TLS dl.q0wqex.ru)
   ↓
http://127.0.0.1:8080
   ↓
DLX (Docker-контейнер, loopback порт 127.0.0.1:8080)
```

DLX слушает только локальный интерфейс хоста (`127.0.0.1:8080`), а внешний Nginx отвечает за HTTPS и проксирование.

---

## Структура проекта

```
dlx/
├── main.go               # Точка входа, конфигурация, graceful shutdown
├── config.go             # Загрузка переменных окружения
├── validator.go          # Валидация URL и защита от SSRF
├── sanitizer.go          # Кроссплатформенная очистка имён файлов
├── errors.go             # Понятные пользователю сообщения об ошибках
├── downloader/
│   ├── types.go          # Модели данных (MediaInfo, DownloadRequest, ProgressEvent)
│   ├── progress.go       # Парсинг машиночитаемого вывода yt-dlp
│   ├── ytdlp.go          # Формирование аргументов и запуск yt-dlp
│   └── manager.go        # Управление семафором, жизненным циклом файлов и SSE
├── server/
│   ├── server.go         # Роутинг, логирование, middleware
│   └── handlers.go       # Обработчики API (/health, /api/info, /api/download, /file)
├── web/
│   ├── embed.go          # Встраивание статики через go:embed
│   ├── index.html        # Семантическая разметка страницы
│   ├── style.css         # Тёмная тема и адаптивные стили
│   └── app.js            # Логика клиента (Clipboard API, SSE, localStorage)
├── Dockerfile            # Multi-stage сборка с официальным standalone yt-dlp и ffmpeg
├── docker-compose.yml   # Docker Compose манифест с привязкой к 127.0.0.1:8080
├── nginx.conf.example    # Пример конфигурации Nginx для dl.q0wqex.ru
└── README.md             # Документация проекта
```

---

## Быстрый старт с Docker Compose

### 1. Требования
- Установленный Docker и Docker Compose (v2+).

### 2. Клонирование и запуск

```bash
git clone https://github.com/q0wqex/dlx.git
cd dlx
docker compose up -d --build
```

Контейнер автоматически скачает официальный бинарник `yt-dlp` под архитектуру сервера (amd64 / arm64), скомпилирует Go-сервер и запустится на `127.0.0.1:8080`.

### 3. Проверка работоспособности

```bash
curl http://127.0.0.1:8080/health
# Ответ: {"status":"ok"}
```

---

## Обновление yt-dlp

Чтобы обновить `yt-dlp` до самой свежей версии из официального GitHub Releases, выполните пересборку контейнера без кэша:

```bash
docker compose build --no-cache
docker compose up -d
```

---

## Переменные окружения

Конфигурация задаётся через переменные окружения в `docker-compose.yml` или системном `.env`:

| Переменная | По умолчанию | Описание |
| :--- | :--- | :--- |
| `PORT` | `8080` | Порт, который слушает сервер внутри контейнера |
| `MAX_CONCURRENT_DOWNLOADS` | `2` | Максимальное число параллельных загрузок |
| `MAX_FILE_SIZE` | `5G` | Максимальный размер файла для скачивания (передаётся в yt-dlp) |
| `DOWNLOAD_TIMEOUT` | `30m` | Таймаут на одну задачу скачивания |
| `TEMP_DIR` | `/tmp/dlx` | Директория для временных файлов |
| `YTDLP_PATH` | `/usr/local/bin/yt-dlp` | Путь к исполняемому файлу yt-dlp |
| `FFMPEG_PATH` | `/usr/bin/ffmpeg` | Путь к исполняемому файлу ffmpeg |

---

## Настройка Nginx

Если Nginx запущен на хосте (или в Docker с `network_mode: host`), добавьте server block:

```nginx
server {
    listen 443 ssl http2;
    server_name dl.q0wqex.ru;

    ssl_certificate /etc/letsencrypt/live/dl.q0wqex.ru/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/dl.q0wqex.ru/privkey.pem;

    client_max_body_size 5000M;

    location / {
        proxy_pass http://127.0.0.1:8080;
        
        proxy_http_version 1.1;
        proxy_set_header Connection '';
        proxy_buffering off;
        proxy_request_buffering off;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_connect_timeout 60s;
        proxy_send_timeout 1800s;
        proxy_read_timeout 1800s;
    }
}
```

---

## Локальный запуск без Docker

Для запуска напрямую на машине разработчика:

1. Убедитесь, что в системе установлены `go` (1.24+), `yt-dlp` и `ffmpeg`.
2. Запустите:
```bash
go run .
```
3. Откройте в браузере: `http://localhost:8080`.

---

## HTTP API

- `GET /health` — проверка здоровья сервиса.
- `POST /api/info` — получение метаданных видео (JSON: `{"url": "https://..."}`).
- `POST /api/download` — постановка задачи на скачивание (JSON с URL, форматом, качеством, субтитрами). Возвращает `{"id": "<uuid>"}`.
- `GET /api/download/{id}/progress` — Server-Sent Events (SSE) поток со статусом и процентами.
- `GET /api/download/{id}/file` — получение готового файла.

---

## Лицензия

MIT License