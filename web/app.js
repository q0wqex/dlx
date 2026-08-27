/**
 * DLX - Client Application Logic
 */

document.addEventListener('DOMContentLoaded', () => {
  // DOM Elements
  const urlInput = document.getElementById('urlInput');
  const pasteBtn = document.getElementById('pasteBtn');
  const downloadBtn = document.getElementById('downloadBtn');
  const downloadForm = document.getElementById('downloadForm');
  const settingsToggle = document.getElementById('settingsToggle');
  const settingsPanel = document.getElementById('settingsPanel');

  // Settings Elements
  const formatButtons = document.querySelectorAll('#formatSelector .segment-btn');
  const qualitySelect = document.getElementById('qualitySelect');
  const qualitySettingWrapper = document.getElementById('qualitySettingWrapper');
  const subtitlesCheckbox = document.getElementById('subtitlesCheckbox');
  const subtitlesLangInput = document.getElementById('subtitlesLangInput');
  const thumbnailCheckbox = document.getElementById('thumbnailCheckbox');

  // Info Card Elements
  const infoCard = document.getElementById('infoCard');
  const infoThumbnail = document.getElementById('infoThumbnail');
  const infoDuration = document.getElementById('infoDuration');
  const infoTitle = document.getElementById('infoTitle');
  const infoSite = document.getElementById('infoSite');
  const infoQualityBadge = document.getElementById('infoQualityBadge');

  // Status & Progress Elements
  const statusCard = document.getElementById('statusCard');
  const loadingState = document.getElementById('loadingState');
  const downloadingState = document.getElementById('downloadingState');
  const completedState = document.getElementById('completedState');
  const errorState = document.getElementById('errorState');

  const progressStage = document.getElementById('progressStage');
  const progressPercent = document.getElementById('progressPercent');
  const progressBarFill = document.getElementById('progressBarFill');
  const progressSpeed = document.getElementById('progressSpeed');
  const progressEta = document.getElementById('progressEta');

  const completedFileName = document.getElementById('completedFileName');
  const completedFileSize = document.getElementById('completedFileSize');
  const saveFileBtn = document.getElementById('saveFileBtn');
  const errorMessage = document.getElementById('errorMessage');
  const retryBtn = document.getElementById('retryBtn');
  const toast = document.getElementById('toast');

  // State
  let activeEventSource = null;
  let metadataAbortController = null;
  let debounceTimeout = null;
  let lastFetchedUrl = '';

  // 1. Initialize Settings from localStorage
  const SETTINGS_KEY = 'dlx_user_settings_v1';
  let currentSettings = {
    format: 'mp4',
    quality: 'best',
    subtitles: false,
    subtitleLanguage: 'ru,en',
    thumbnail: false,
  };

  function loadSettings() {
    try {
      const saved = localStorage.getItem(SETTINGS_KEY);
      if (saved) {
        currentSettings = { ...currentSettings, ...JSON.parse(saved) };
      }
    } catch (e) {
      console.warn('Failed to load settings from localStorage:', e);
    }
    applySettingsToUI();
  }

  function saveSettings() {
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(currentSettings));
    } catch (e) {
      console.warn('Failed to save settings to localStorage:', e);
    }
  }

  function applySettingsToUI() {
    // Format
    formatButtons.forEach(btn => {
      btn.classList.toggle('active', btn.dataset.value === currentSettings.format);
    });

    if (currentSettings.format === 'mp3') {
      qualitySettingWrapper.classList.add('hidden');
    } else {
      qualitySettingWrapper.classList.remove('hidden');
    }

    // Quality
    if (qualitySelect) {
      qualitySelect.value = currentSettings.quality || 'best';
    }

    // Subtitles
    if (subtitlesCheckbox) {
      subtitlesCheckbox.checked = !!currentSettings.subtitles;
      subtitlesLangInput.disabled = !currentSettings.subtitles;
      subtitlesLangInput.value = currentSettings.subtitleLanguage || 'ru,en';
    }

    // Thumbnail
    if (thumbnailCheckbox) {
      thumbnailCheckbox.checked = !!currentSettings.thumbnail;
    }
  }

  // 2. Settings Event Handlers
  settingsToggle.addEventListener('click', () => {
    const isExpanded = settingsToggle.getAttribute('aria-expanded') === 'true';
    settingsToggle.setAttribute('aria-expanded', !isExpanded);
    settingsPanel.classList.toggle('hidden', isExpanded);
  });

  formatButtons.forEach(btn => {
    btn.addEventListener('click', () => {
      formatButtons.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');
      currentSettings.format = btn.dataset.value;
      if (currentSettings.format === 'mp3') {
        qualitySettingWrapper.classList.add('hidden');
      } else {
        qualitySettingWrapper.classList.remove('hidden');
      }
      saveSettings();
    });
  });

  qualitySelect.addEventListener('change', () => {
    currentSettings.quality = qualitySelect.value;
    saveSettings();
  });

  subtitlesCheckbox.addEventListener('change', () => {
    currentSettings.subtitles = subtitlesCheckbox.checked;
    subtitlesLangInput.disabled = !currentSettings.subtitles;
    saveSettings();
  });

  subtitlesLangInput.addEventListener('input', () => {
    currentSettings.subtitleLanguage = subtitlesLangInput.value.trim();
    saveSettings();
  });

  thumbnailCheckbox.addEventListener('change', () => {
    currentSettings.thumbnail = thumbnailCheckbox.checked;
    saveSettings();
  });

  // 3. Clipboard & Paste Handling
  pasteBtn.addEventListener('click', async () => {
    try {
      if (!navigator.clipboard || !navigator.clipboard.readText) {
        showToast('Буфер обмена недоступен в вашем браузере');
        urlInput.focus();
        return;
      }

      const text = await navigator.clipboard.readText();
      const trimmed = (text || '').trim();

      if (isValidHttpUrl(trimmed)) {
        urlInput.value = trimmed;
        onUrlChanged(trimmed);
        showToast('Ссылка вставлена');
        urlInput.focus();
      } else {
        showToast('В буфере обмена нет подходящей ссылки');
      }
    } catch (err) {
      showToast('Разрешите доступ к буферу обмена для быстрой вставки');
      urlInput.focus();
    }
  });

  // 4. URL Input handling & Metadata Prefetching
  urlInput.addEventListener('input', () => {
    const val = urlInput.value.trim();
    onUrlChanged(val);
  });

  urlInput.addEventListener('paste', () => {
    setTimeout(() => {
      onUrlChanged(urlInput.value.trim());
    }, 50);
  });

  function onUrlChanged(url) {
    if (!isValidHttpUrl(url)) {
      infoCard.classList.add('hidden');
      if (metadataAbortController) {
        metadataAbortController.abort();
        metadataAbortController = null;
      }
      return;
    }

    if (url === lastFetchedUrl) return;

    clearTimeout(debounceTimeout);
    debounceTimeout = setTimeout(() => {
      fetchMetadata(url);
    }, 400);
  }

  async function fetchMetadata(url) {
    if (metadataAbortController) {
      metadataAbortController.abort();
    }
    metadataAbortController = new AbortController();
    lastFetchedUrl = url;

    try {
      const response = await fetch('/api/info', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url }),
        signal: metadataAbortController.signal,
      });

      if (!response.ok) return;

      const data = await response.json();
      if (data && data.title) {
        displayMetadata(data);
      }
    } catch (e) {
      // Ignore aborted fetches or network hiccups during typing
    }
  }

  function displayMetadata(data) {
    infoTitle.textContent = data.title || 'Видео';
    infoDuration.textContent = formatDuration(data.duration);
    infoSite.textContent = data.site || 'Web';
    infoQualityBadge.textContent = currentSettings.format === 'mp3' ? 'MP3 Audio' : (currentSettings.quality === 'best' ? 'Лучшее' : currentSettings.quality + 'p');

    if (data.thumbnail) {
      infoThumbnail.src = data.thumbnail;
      infoThumbnail.classList.remove('hidden');
    } else {
      infoThumbnail.classList.add('hidden');
    }

    infoCard.classList.remove('hidden');
  }

  // 5. Download Execution
  downloadForm.addEventListener('submit', (e) => {
    e.preventDefault();
    startDownload();
  });

  retryBtn.addEventListener('click', () => {
    startDownload();
  });

  async function startDownload() {
    const rawUrl = urlInput.value.trim();
    if (!isValidHttpUrl(rawUrl)) {
      showToast('Введите корректную ссылку на видео');
      urlInput.focus();
      return;
    }

    // Cancel any previous event stream
    if (activeEventSource) {
      activeEventSource.close();
      activeEventSource = null;
    }

    // Update UI to downloading state
    setUIState('downloading');
    updateProgressUI({
      stage: 'Подготовка к скачиванию...',
      percent: 0,
      speed: '-',
      eta: '-',
    });

    try {
      const payload = {
        url: rawUrl,
        format: currentSettings.format,
        quality: currentSettings.quality,
        subtitles: currentSettings.subtitles,
        subtitleLanguage: currentSettings.subtitleLanguage,
        thumbnail: currentSettings.thumbnail,
      };

      const response = await fetch('/api/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.error || 'Ошибка сервера при запуске скачивания');
      }

      const { id } = await response.json();
      listenToProgress(id);
    } catch (err) {
      showError(err.message || 'Не удалось начать загрузку');
    }
  }

  function listenToProgress(jobId) {
    const sseUrl = `/api/download/${jobId}/progress`;
    activeEventSource = new EventSource(sseUrl);

    activeEventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleProgressEvent(data);
      } catch (e) {
        console.error('Error parsing SSE event data:', e);
      }
    };

    activeEventSource.onerror = () => {
      // EventSource reconnects automatically on network drop, but check if complete
      console.warn('SSE connection drop/error');
    };
  }

  function handleProgressEvent(data) {
    if (data.status === 'downloading' || data.status === 'pending' || data.status === 'processing') {
      updateProgressUI(data);
    } else if (data.status === 'completed') {
      if (activeEventSource) {
        activeEventSource.close();
        activeEventSource = null;
      }
      showCompleted(data);
    } else if (data.status === 'error') {
      if (activeEventSource) {
        activeEventSource.close();
        activeEventSource = null;
      }
      showError(data.error || 'Произошла ошибка при скачивании');
    }
  }

  function updateProgressUI(data) {
    progressStage.textContent = data.stage || 'Скачивание';
    const pct = Math.min(100, Math.max(0, data.percent || 0));
    progressPercent.textContent = Math.round(pct) + '%';
    progressBarFill.style.width = pct + '%';
    progressSpeed.textContent = data.speed || '-';
    progressEta.textContent = data.eta ? `осталось ${data.eta}` : '-';
  }

  function showCompleted(data) {
    setUIState('completed');
    completedFileName.textContent = data.filename || 'video.mp4';
    completedFileSize.textContent = formatBytes(data.fileSize || 0);

    const downloadUrl = `/api/download/${data.id}/file`;
    saveFileBtn.href = downloadUrl;
    saveFileBtn.setAttribute('download', data.filename || 'video.mp4');

    // Trigger auto-download in browser
    saveFileBtn.click();
  }

  function showError(msg) {
    setUIState('error');
    errorMessage.textContent = msg;
  }

  function setUIState(state) {
    statusCard.classList.remove('hidden');
    loadingState.classList.add('hidden');
    downloadingState.classList.add('hidden');
    completedState.classList.add('hidden');
    errorState.classList.add('hidden');

    downloadBtn.disabled = state === 'downloading';

    if (state === 'loading') {
      loadingState.classList.remove('hidden');
    } else if (state === 'downloading') {
      downloadingState.classList.remove('hidden');
    } else if (state === 'completed') {
      completedState.classList.remove('hidden');
    } else if (state === 'error') {
      errorState.classList.remove('hidden');
    }
  }

  // 6. Helpers
  function isValidHttpUrl(string) {
    if (!string) return false;
    try {
      const url = new URL(string);
      return url.protocol === 'http:' || url.protocol === 'https:';
    } catch (_) {
      return false;
    }
  }

  function formatDuration(seconds) {
    if (!seconds || isNaN(seconds) || seconds <= 0) return '00:00';
    const sec = Math.floor(seconds);
    const hrs = Math.floor(sec / 3600);
    const mins = Math.floor((sec % 3600) / 60);
    const secs = sec % 60;

    const pad = (num) => String(num).padStart(2, '0');
    if (hrs > 0) {
      return `${hrs}:${pad(mins)}:${pad(secs)}`;
    }
    return `${pad(mins)}:${pad(secs)}`;
  }

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) {
      size /= 1024;
      i++;
    }
    return `${size.toFixed(1)} ${units[i]}`;
  }

  let toastTimer = null;
  function showToast(message) {
    toast.textContent = message;
    toast.classList.remove('hidden');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast.classList.add('hidden');
    }, 2800);
  }

  // Initialize
  loadSettings();
});
