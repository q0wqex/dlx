package main

import (
	"strings"
)

// TranslateError converts technical yt-dlp or system errors into clear, friendly Russian error messages.
func TranslateError(err error, rawOutput string) string {
	if err == nil {
		return ""
	}

	combined := strings.ToLower(err.Error() + " " + rawOutput)

	switch {
	case strings.Contains(combined, "sign in to confirm you're not a bot") ||
		strings.Contains(combined, "bot detection") ||
		strings.Contains(combined, "captcha") ||
		strings.Contains(combined, "login required") ||
		strings.Contains(combined, "account is private"):
		return "Не удалось скачать видео: сайт требует авторизацию или заблокировал запрос."

	case strings.Contains(combined, "video unavailable") ||
		strings.Contains(combined, "this video has been removed") ||
		strings.Contains(combined, "not found") ||
		strings.Contains(combined, "404") ||
		strings.Contains(combined, "deleted"):
		return "Видео недоступно, удалено или имеет ограниченный доступ."

	case strings.Contains(combined, "is not a valid url") ||
		strings.Contains(combined, "unsupported url") ||
		strings.Contains(combined, "no suitable extractor") ||
		strings.Contains(combined, "unsupported url"):
		return "Данная ссылка не поддерживается или не содержит доступного медиаконтента."

	case strings.Contains(combined, "file is larger than max-filesize") ||
		strings.Contains(combined, "max_file_size") ||
		strings.Contains(combined, "too large"):
		return "Размер файла превышает допустимый лимит сервера."

	case strings.Contains(combined, "context deadline exceeded") ||
		strings.Contains(combined, "timeout") ||
		strings.Contains(combined, "timed out"):
		return "Превышено время ожидания загрузки (таймаут)."

	case strings.Contains(combined, "server is busy") ||
		strings.Contains(combined, "concurrency limit"):
		return "Сервер выполняет максимальное количество загрузок. Пожалуйста, повторите через минуту."

	case strings.Contains(combined, "ffmpeg"):
		return "Ошибка при обработке медиапотоков через ffmpeg."

	case strings.Contains(combined, "network is unreachable") ||
		strings.Contains(combined, "connection refused") ||
		strings.Contains(combined, "temporary failure in name resolution"):
		return "Ошибка сетевого соединения с сайтом-источником."

	default:
		return "Не удалось обработать видео. Проверьте ссылку или повторите попытку позже."
	}
}
