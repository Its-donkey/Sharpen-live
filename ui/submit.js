(function () {
  const section = document.getElementById('submit-streamer-section');
  const form = document.getElementById('submit-streamer-form');
  const toggle = document.getElementById('submit-toggle');
  if (!form) return;

  if (toggle && section) {
    toggle.addEventListener('click', () => {
      section.classList.toggle('is-collapsed');
    });
  }

  const nameInput = document.getElementById('streamer-name');
  const descInput = document.getElementById('streamer-description');
  const langSelect = document.getElementById('language-select');
  const langTags = form.querySelector('.language-tags');

  let inflightTarget = '';
  let debounceTimer;

  function log(...args) {
    if (window && window.console) {
      console.log('[submit.js]', ...args);
    }
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
      const resp = await fetch('/api/youtube/metadata', {
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
    selectWrapper.classList.toggle('is-visible', show);
  }

  function bindPlatformInputs() {
    if (shouldSkipMetadata()) return;
    const inputs = form.querySelectorAll('.channel-url-input');
    if (!inputs.length) return;
    log('binding metadata handlers to', inputs.length, 'platform input(s)');
    inputs.forEach((input) => {
      const row = input.closest('.platform-row');
      const select = row ? row.querySelector('.platform-select') : null;
      const selectWrapper = row ? row.querySelector('.platform-select-wrapper') : null;
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
        btn.addEventListener('click', (e) => {
          e.preventDefault();
          e.stopPropagation();
          const remaining = Array.from(selected.entries()).filter(([key]) => key !== value);
          selected.clear();
          remaining.forEach(([key, val]) => selected.set(key, val));
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

    langTags.addEventListener('click', (e) => {
      // prevent clicks in the language field from triggering unrelated handlers
      e.preventDefault();
      e.stopPropagation();
    });

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
    bindPlatformInputs();
    initLanguagePicker();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
