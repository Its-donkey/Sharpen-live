(function () {
  const section = document.getElementById('submit') || document.getElementById('submit-streamer-section');
  const form = document.getElementById('submit-streamer-form');
  const toggle = document.getElementById('submit-toggle');
  if (!form) return;

  if (toggle && section) {
    const showForm = () => {
      section.classList.remove('is-collapsed');
      toggle.setAttribute('hidden', 'true');
    };
    if (!section.classList.contains('is-collapsed')) {
      toggle.setAttribute('hidden', 'true');
    }
    toggle.addEventListener('click', showForm);
  }

  const nameInput = document.getElementById('streamer-name');
  const descInput = document.getElementById('streamer-description');
  const langSelect = document.getElementById('language-select');
  const langTags = form.querySelector('.language-tags');

  let inflightTarget = '';
  let debounceTimer;
  const setChannelId = (channelId) => {
    const firstRow = form.querySelector('.platform-row');
    if (!firstRow) return;
    const channelInput = firstRow.querySelector('.platform-channel-id');
    if (!channelInput) return;
    if (channelId) {
      channelInput.value = channelId;
    } else {
      channelInput.value = '';
    }
  };

  function log(...args) {
    if (window && window.console) {
      console.log('[submit.js]', ...args);
    }
  }

  function initStreamersWatch() {
    const paths = ['/api/streamers/watch', '/streamers/watch'];
    if (typeof EventSource === 'undefined') {
      return;
    }
    let lastTimestamp = null;
    let source = null;

    const connect = (idx) => {
      if (idx >= paths.length) return;
      source = new EventSource(paths[idx]);
      source.addEventListener('message', (evt) => {
        const ts = parseInt((evt && evt.data) || '', 10);
        if (Number.isNaN(ts)) return;
        if (lastTimestamp === null) {
          lastTimestamp = ts;
          return;
        }
        if (ts > lastTimestamp) {
          window.location.reload();
        }
      });
      source.addEventListener('error', () => {
        try {
          source && source.close();
        } catch (_) {}
        connect(idx + 1);
      });
    };

    connect(0);
  }

  // ------- Metadata helpers -------
  function shouldSkipMetadata() {
    return !form || !nameInput || !descInput;
  }

  async function fetchMetadata(target) {
    const url = (target || '').trim();
    if (!url || url === inflightTarget) return;
    inflightTarget = url;
    log('fetching metadata for', url);
    try {
      const resp = await fetch('/api/metadata', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url })
      });
      inflightTarget = '';
      if (!resp.ok) {
        log('metadata fetch failed', resp.status, resp.statusText);
        return;
      }
      const data = await resp.json();
      log('metadata response', data);
      if (descInput && descInput.value.trim() === '') {
        if (data.description && data.description.trim() !== '') {
          descInput.value = data.description.trim();
        } else if (data.title && data.title.trim() !== '') {
          descInput.value = data.title.trim();
        }
      }
      if (nameInput && nameInput.value.trim() === '') {
        if (data.title && data.title.trim() !== '') {
          nameInput.value = data.title.trim();
        } else if (data.handle && data.handle.trim() !== '') {
          nameInput.value = data.handle.trim();
        }
      }
      if (data.channelId) {
        setChannelId(data.channelId.trim());
      } else {
        setChannelId('');
      }
    } catch (err) {
      inflightTarget = '';
      log('metadata fetch error', err);
    }
  }

  function queueMetadataFetch(value) {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => fetchMetadata(value || ''), 300);
  }

  function platformURLForHandle(platform, handle) {
    const cleanHandle = (handle || '').trim().replace(/^@+/, '');
    if (!cleanHandle) return '';
    switch ((platform || '').toLowerCase()) {
      case 'youtube':
        return `https://youtube.com/@${cleanHandle}`;
      case 'twitch':
        return `https://twitch.tv/${cleanHandle}`;
      case 'facebook':
        return `https://facebook.com/${cleanHandle}`;
      default:
        return '';
    }
  }

  function updatePlatformVisibility(input, selectWrapper) {
    if (!selectWrapper) return;
    const value = (input && input.value ? input.value : '').trim();
    const show = value.startsWith('@');
    selectWrapper.classList.toggle('platform-select-hidden', !show);
  }

  function bindPlatformInputs() {
    if (shouldSkipMetadata()) return;
    const inputs = form.querySelectorAll('.channel-url-input');
    if (!inputs.length) return;
    log('binding metadata handlers to', inputs.length, 'platform input(s)');
    inputs.forEach((input) => {
      const row = input.closest('.platform-row');
      const select = row ? row.querySelector('.platform-select select, .platform-select-input') : null;
      const selectWrapper = row ? row.querySelector('.platform-select') : null;
      const metadataHandler = (e) => queueMetadataFetch(e.target.value || '');
      const visibilityHandler = () => updatePlatformVisibility(input, selectWrapper);

      input.addEventListener('blur', metadataHandler);
      input.addEventListener('change', metadataHandler);
      input.addEventListener('input', (e) => {
        visibilityHandler();
        metadataHandler(e);
      });

      if (select) {
        select.addEventListener('change', () => {
          const current = (input.value || '').trim();
          if (!current.startsWith('@')) return;
          const url = platformURLForHandle(select.value, current);
          if (!url) return;
          input.value = url;
          visibilityHandler();
          queueMetadataFetch(url);
        });
      }

      visibilityHandler();
    });

    const firstVal = inputs[0].value;
    if (firstVal && firstVal.trim() !== '') {
      queueMetadataFetch(firstVal);
    }
  }

  // ------- Platform row management -------
  let platformRowCounter = 0;
  let maxPlatforms = 0;
  let platformRowsContainer = null;
  let addPlatformButton = null;

  function updateAddButtonState() {
    if (!addPlatformButton || !platformRowsContainer || !maxPlatforms) return;
    const totalRows = platformRowsContainer.querySelectorAll('.platform-row').length;
    addPlatformButton.disabled = totalRows >= maxPlatforms;
  }

  function initPlatformRowManagement() {
    const platformFieldset = form.querySelector('.platform-fieldset');
    if (!platformFieldset) return;

    platformRowsContainer = platformFieldset.querySelector('.platform-rows');
    addPlatformButton = platformFieldset.querySelector('.add-platform-button');
    if (!platformRowsContainer || !addPlatformButton) return;

    const parsedMax = parseInt(addPlatformButton.dataset.maxPlatforms || '', 10);
    maxPlatforms = Number.isNaN(parsedMax) ? 0 : parsedMax;

    // Initialize counter based on existing rows
    const existingRows = platformRowsContainer.querySelectorAll('.platform-row');
    platformRowCounter = existingRows.length;

    // Add platform button handler
    addPlatformButton.addEventListener('click', (e) => {
      e.preventDefault();
      addPlatformRow();
    });

    // Bind remove handlers to existing rows
    bindRemoveHandlers();
    updateAddButtonState();
  }

  function addPlatformRow() {
    if (!platformRowsContainer || !addPlatformButton) return;

    const existingRows = platformRowsContainer.querySelectorAll('.platform-row');
    if (existingRows.length === 0) return;
    if (maxPlatforms && existingRows.length >= maxPlatforms) return;

    // Clone the first row as template
    const template = existingRows[0];
    const newRow = template.cloneNode(true);

    // Generate new row ID
    const newRowId = `row-${platformRowCounter++}`;
    newRow.setAttribute('data-platform-row', newRowId);

    // Update hidden ID input
    const idInput = newRow.querySelector('input[name="platform_id"]');
    if (idInput) idInput.value = newRowId;
    const channelIdInput = newRow.querySelector('.platform-channel-id');
    if (channelIdInput) {
      channelIdInput.value = '';
    }

    // Clear the URL input
    const urlInput = newRow.querySelector('.channel-url-input');
    if (urlInput) {
      urlInput.value = '';
      urlInput.removeAttribute('id'); // Remove id to avoid duplicates
    }

    // Reset platform select
    const platformSelect = newRow.querySelector('.platform-select select');
    if (platformSelect) {
      platformSelect.value = '';
      platformSelect.removeAttribute('data-row');
      platformSelect.setAttribute('data-row', newRowId);
    }

    // Hide platform select wrapper initially
    const selectWrapper = newRow.querySelector('.platform-select');
    if (selectWrapper) {
      selectWrapper.classList.add('platform-select-hidden');
    }

    // Ensure remove button exists and is visible
    let removeButton = newRow.querySelector('.remove-platform-button');
    if (!removeButton) {
      // Create remove button if it doesn't exist
      const buttonContainer = newRow.querySelector('.platform-row-inner');
      removeButton = document.createElement('button');
      removeButton.type = 'button';
      removeButton.className = 'remove-platform-button';
      removeButton.name = 'remove_platform';
      removeButton.textContent = 'Remove';
      newRow.insertBefore(removeButton, buttonContainer.nextSibling);
    }
    removeButton.value = newRowId;

    // Clear any error messages
    const errorMsg = newRow.querySelector('.field-error-text');
    if (errorMsg) errorMsg.remove();

    // Remove error class from label
    const urlLabel = newRow.querySelector('.platform-url');
    if (urlLabel) urlLabel.classList.remove('form-field-error');

    // Append new row
    platformRowsContainer.appendChild(newRow);

    // Rebind all handlers
    bindPlatformInputs();
    bindRemoveHandlers();
    updateAddButtonState();

    // Focus new input
    if (urlInput) urlInput.focus();
  }

  function bindRemoveHandlers() {
    const removeButtons = form.querySelectorAll('.remove-platform-button');
    removeButtons.forEach((btn) => {
      // Remove old listeners by cloning
      const newBtn = btn.cloneNode(true);
      btn.parentNode.replaceChild(newBtn, btn);

      newBtn.addEventListener('click', (e) => {
        e.preventDefault();
        const rowId = newBtn.value;
        const row = form.querySelector(`.platform-row[data-platform-row="${rowId}"]`);
        if (!row) return;

        // Only remove if there's more than one row
        const allRows = form.querySelectorAll('.platform-row');
        if (allRows.length <= 1) return;

        row.remove();

        // Rebind handlers after removal
        bindPlatformInputs();
        bindRemoveHandlers();
        updateAddButtonState();
      });
    });
  }

  // ------- Language picker helpers -------
  function initLanguagePicker() {
    if (!langSelect || !langTags) return;
    const addLangBtn = form.querySelector('.add-language-button');
    const picker = form.querySelector('.language-picker');
    const allOptions = Array.from(langSelect.options)
      .filter((opt) => opt.value && opt.value.trim() !== '')
      .map((opt) => ({ value: opt.value, label: opt.textContent.trim(), selected: opt.dataset.selected === 'true' }));
    const selected = new Map(); // value -> label

    function showSelect() {
      langSelect.classList.remove('is-hidden');
      if (addLangBtn) addLangBtn.classList.add('is-hidden');
      if (picker) picker.classList.add('is-select-visible');
      langSelect.focus();
    }

    function hideSelect() {
      langSelect.value = '';
      langSelect.classList.add('is-hidden');
      if (addLangBtn) addLangBtn.classList.remove('is-hidden');
      if (picker) picker.classList.remove('is-select-visible');
    }

    function addLanguage(value, label) {
      const trimmed = (value || '').trim();
      if (!trimmed || selected.has(trimmed)) return;
      selected.set(trimmed, label || trimmed);
      renderTags();
      renderOptions();
    }

    function renderTags() {
      langTags.innerHTML = '';
      if (selected.size === 0) {
        const empty = document.createElement('span');
        empty.className = 'language-empty';
        empty.textContent = 'No languages selected yet.';
        langTags.appendChild(empty);
        return;
      }
      selected.forEach((label, value) => {
        const pill = document.createElement('span');
        pill.className = 'language-pill';
        pill.textContent = label;

        const btn = document.createElement('button');
        btn.type = 'button';
        btn.setAttribute('aria-label', 'Remove ' + label);
        btn.dataset.removeLanguage = value;
        btn.textContent = '×';
        btn.addEventListener('click', () => {
          selected.delete(value);
          renderTags();
          renderOptions();
        });
        pill.appendChild(btn);

        const hidden = document.createElement('input');
        hidden.type = 'hidden';
        hidden.name = 'languages[]';
        hidden.value = value;

        langTags.appendChild(pill);
        langTags.appendChild(hidden);
      });
    }

    function renderOptions() {
      langSelect.innerHTML = '';
      const placeholder = document.createElement('option');
      placeholder.value = '';
      placeholder.textContent = 'Select a language…';
      langSelect.appendChild(placeholder);
      allOptions.forEach((opt) => {
        if (selected.has(opt.value)) return;
        const option = document.createElement('option');
        option.value = opt.value;
        option.textContent = opt.label;
        langSelect.appendChild(option);
      });
      langSelect.selectedIndex = 0;
    }

    // Seed selected from initial state
    allOptions.forEach((opt) => {
      if (opt.selected) {
        selected.set(opt.value, opt.label);
      }
    });

    renderTags();
    renderOptions();

    langSelect.addEventListener('change', (e) => {
      const option = e.target.selectedOptions && e.target.selectedOptions[0];
      if (!option) return;
      const value = option.value.trim();
      if (!value) return;
      const label = option.textContent.trim() || value;
      addLanguage(value, label);
      hideSelect();
    });

    if (addLangBtn) {
      addLangBtn.addEventListener('click', () => {
        if (langSelect.disabled) return;
        showSelect();
      });
    }

    // Keep select hidden initially
    hideSelect();
  }

  function init() {
    initLanguagePicker();
    initPlatformRowManagement(); // Initialize platform add/remove handlers
    bindPlatformInputs();
    initStreamersWatch();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
