(function () {
  const form = document.getElementById('submit-streamer-form');
  if (!form) return;

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

  function bindPlatformInputs() {
    if (shouldSkipMetadata()) return;
    const inputs = form.querySelectorAll('.channel-url-input');
    if (!inputs.length) return;
    log('binding metadata handlers to', inputs.length, 'platform input(s)');
    const handler = (e) => queueMetadataFetch(e.target.value || '');
    inputs.forEach((input) => {
      input.addEventListener('blur', handler);
      input.addEventListener('change', handler);
      input.addEventListener('input', handler);
    });

    const firstVal = inputs[0].value;
    if (firstVal && firstVal.trim() !== '') {
      queueMetadataFetch(firstVal);
    }
  }

  // ------- Language picker helpers -------
  function initLanguagePicker() {
    if (!langSelect || !langTags) return;
    const allOptions = Array.from(langSelect.options)
      .filter((opt) => opt.value && opt.value.trim() !== '')
      .map((opt) => ({ value: opt.value, label: opt.textContent.trim(), selected: opt.dataset.selected === 'true' }));
    const selected = new Map(); // value -> label

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
      if (!selected.has(value)) {
        selected.set(value, label);
        renderTags();
        renderOptions();
      }
    });
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
