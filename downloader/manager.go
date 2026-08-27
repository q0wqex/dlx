package downloader

import (
	"bufio"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrorTranslator is a function type to translate errors.
type ErrorTranslator func(err error, rawOutput string) string

// FilenameSanitizer is a function type to sanitize filenames.
type FilenameSanitizer func(rawName, defaultExt, fallbackName string) string

// JobManager coordinates job concurrency, execution, and cleanup.
type JobManager struct {
	YtDlpPath              string
	FFmpegPath             string
	MaxFileSize            string
	TempDir                string
	DownloadTimeout        time.Duration
	MaxConcurrentDownloads int

	TranslateError   ErrorTranslator
	SanitizeFilename FilenameSanitizer

	sem  chan struct{}
	jobs sync.Map // map[string]*DownloadJob

	ctx       context.Context
	cancelAll context.CancelFunc
	wg        sync.WaitGroup
}

// NewJobManager initializes a JobManager instance.
func NewJobManager(
	ytDlpPath, ffmpegPath, maxFileSize, tempDir string,
	timeout time.Duration,
	maxConcurrent int,
	errTranslator ErrorTranslator,
	sanitizer FilenameSanitizer,
) *JobManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &JobManager{
		YtDlpPath:              ytDlpPath,
		FFmpegPath:             ffmpegPath,
		MaxFileSize:            maxFileSize,
		TempDir:                tempDir,
		DownloadTimeout:        timeout,
		MaxConcurrentDownloads: maxConcurrent,
		TranslateError:         errTranslator,
		SanitizeFilename:       sanitizer,
		sem:                    make(chan struct{}, maxConcurrent),
		ctx:                    ctx,
		cancelAll:              cancel,
	}
}

// CleanupStartup removes stale temporary directories.
func (m *JobManager) CleanupStartup() {
	if err := os.RemoveAll(m.TempDir); err != nil {
		log.Printf("[CLEANUP] Warning: failed to clean old temp dir %s: %v", m.TempDir, err)
	}
	if err := os.MkdirAll(m.TempDir, 0755); err != nil {
		log.Printf("[CLEANUP] Error creating temp dir %s: %v", m.TempDir, err)
	} else {
		log.Printf("[STARTUP] Temp directory initialized at %s", m.TempDir)
	}
}

// StartJob creates and launches a download task.
func (m *JobManager) StartJob(req DownloadRequest) (*DownloadJob, error) {
	// Check concurrency limit before creating job
	select {
	case m.sem <- struct{}{}:
		// Slot acquired
	default:
		return nil, ErrServerBusy
	}

	job, err := newJob(req, m.TempDir)
	if err != nil {
		<-m.sem
		return nil, err
	}

	jobCtx, cancel := context.WithTimeout(m.ctx, m.DownloadTimeout)
	job.CancelFunc = cancel

	m.jobs.Store(job.ID, job)
	m.wg.Add(1)

	go func() {
		defer func() {
			<-m.sem
			m.wg.Done()
		}()
		m.executeJob(jobCtx, job)
	}()

	return job, nil
}

// GetJob retrieves a job by ID.
func (m *JobManager) GetJob(id string) (*DownloadJob, bool) {
	val, ok := m.jobs.Load(id)
	if !ok {
		return nil, false
	}
	job, ok := val.(*DownloadJob)
	return job, ok
}

// DeleteJob cleans up job files and removes it from memory.
func (m *JobManager) DeleteJob(id string) {
	if val, ok := m.jobs.LoadAndDelete(id); ok {
		job := val.(*DownloadJob)
		if job.CancelFunc != nil {
			job.CancelFunc()
		}
		if job.TempDir != "" {
			_ = os.RemoveAll(job.TempDir)
		}
	}
}

func (m *JobManager) executeJob(ctx context.Context, job *DownloadJob) {
	defer func() {
		close(job.doneChan)
		// Schedule auto-cleanup after 15 minutes in case file is never requested
		time.AfterFunc(15*time.Minute, func() {
			m.DeleteJob(job.ID)
		})
	}()

	startTime := time.Now()
	log.Printf("[DOWNLOAD START] ID=%s URL=%s Format=%s Quality=%s", job.ID, job.Request.URL, job.Request.Format, job.Request.Quality)

	job.Broadcast(NewProgressEvent(job.ID, StatusDownloading, 0.0, "-", "-", "Запуск загрузчика"))

	args := BuildDownloadArgs(m.YtDlpPath, m.FFmpegPath, m.MaxFileSize, job.TempDir, job.Request)
	cmd := exec.CommandContext(ctx, m.YtDlpPath, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := "Ошибка инициализации потока stdout"
		log.Printf("[DOWNLOAD ERROR] ID=%s: %v", job.ID, err)
		ev := NewProgressEvent(job.ID, StatusError, 0, "-", "-", "Ошибка")
		ev.Error = errMsg
		job.Broadcast(ev)
		return
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := "Ошибка инициализации потока stderr"
		log.Printf("[DOWNLOAD ERROR] ID=%s: %v", job.ID, err)
		ev := NewProgressEvent(job.ID, StatusError, 0, "-", "-", "Ошибка")
		ev.Error = errMsg
		job.Broadcast(ev)
		return
	}

	if err := cmd.Start(); err != nil {
		errMsg := m.TranslateError(err, "")
		log.Printf("[DOWNLOAD ERROR] ID=%s failed to start yt-dlp: %v", job.ID, err)
		ev := NewProgressEvent(job.ID, StatusError, 0, "-", "-", "Ошибка")
		ev.Error = errMsg
		job.Broadcast(ev)
		return
	}

	var stderrBuilder strings.Builder
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuilder.WriteString(line)
			stderrBuilder.WriteString("\n")
		}
	}()

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		if event, ok := ParseProgressLine(line, job.ID); ok {
			job.Broadcast(*event)
		}
	}

	cmdErr := cmd.Wait()
	stderrWg.Wait()

	if cmdErr != nil {
		rawStderr := stderrBuilder.String()
		log.Printf("[DOWNLOAD FAILED] ID=%s error=%v stderr=%s", job.ID, cmdErr, rawStderr)
		friendlyMsg := m.TranslateError(cmdErr, rawStderr)

		ev := NewProgressEvent(job.ID, StatusError, 0, "-", "-", "Ошибка")
		ev.Error = friendlyMsg
		job.Broadcast(ev)
		return
	}

	// Locate downloaded file inside job temp dir
	targetFile, targetName, size, err := m.findDownloadedFile(job.TempDir, job.Request.Format)
	if err != nil {
		log.Printf("[DOWNLOAD ERROR] ID=%s file not found in %s: %v", job.ID, job.TempDir, err)
		ev := NewProgressEvent(job.ID, StatusError, 0, "-", "-", "Ошибка")
		ev.Error = "Не удалось обнаружить сохранённый файл"
		job.Broadcast(ev)
		return
	}

	job.FilePath = targetFile
	job.FileName = targetName
	job.FileSize = size

	duration := time.Since(startTime)
	log.Printf("[DOWNLOAD COMPLETE] ID=%s File=%s Size=%d Duration=%s", job.ID, targetName, size, duration.Round(time.Millisecond))

	completedEvent := ProgressEvent{
		ID:        job.ID,
		Status:    StatusCompleted,
		Percent:   100.0,
		Speed:     "-",
		ETA:       "-",
		Stage:     "Готово к сохранению",
		Filename:  targetName,
		FileSize:  size,
		Timestamp: time.Now().UnixMilli(),
	}
	job.Broadcast(completedEvent)
}

func (m *JobManager) findDownloadedFile(dir, requestedFormat string) (filePath string, cleanName string, size int64, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", 0, err
	}

	var bestEntry os.DirEntry
	var bestSize int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Ignore temporary part/ytdl files
		if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".ytdl") || strings.HasSuffix(name, ".temp") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.Size() > bestSize {
			bestSize = info.Size()
			bestEntry = entry
		}
	}

	if bestEntry == nil || bestSize == 0 {
		return "", "", 0, ErrFileNotFound
	}

	rawName := bestEntry.Name()
	ext := filepath.Ext(rawName)
	fallback := "video.mp4"
	if strings.ToLower(requestedFormat) == "mp3" {
		fallback = "audio.mp3"
	}

	clean := m.SanitizeFilename(strings.TrimSuffix(rawName, ext), ext, fallback)
	return filepath.Join(dir, rawName), clean, bestSize, nil
}

// Shutdown cancels active jobs and waits for background routines to stop.
func (m *JobManager) Shutdown(ctx context.Context) {
	log.Println("[SHUTDOWN] Cancelling all active download jobs...")
	m.cancelAll()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[SHUTDOWN] All download jobs terminated cleanly")
	case <-ctx.Done():
		log.Println("[SHUTDOWN] Timed out waiting for download jobs to terminate")
	}

	// Final cleanup of temp directory
	_ = os.RemoveAll(m.TempDir)
}
