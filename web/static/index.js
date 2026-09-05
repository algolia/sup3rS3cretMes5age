/**
 * Secret Message Creation Interface
 *
 * Processes message creation requests with optional file uploads and custom TTL.
 * Submits data to /secret API endpoint and returns a shareable one-time link.
 * All event handlers are CSP-compliant.
 */

import { $, $$, setupLanguage, translate } from './utils.js';

// CSS manipulation helper
function setStyles(element, styles) {
  Object.assign(element.style, styles);
}

// Form submission handler
document.addEventListener('DOMContentLoaded', function() {
  // Initialize clipboard functionality
  new ClipboardJS('.btn');

  // Initialize language manager; keep the promise so dynamic strings can
  // wait for the active locale to be applied before rendering
  const languageReady = setupLanguage();

  // Custom file input handler
  const fileInput = document.getElementById('file-input');
  const fileNameSpan = $('.file-name');
  // Capture the label shipped in the HTML so clearing a file selection can
  // restore it even before translations have loaded (or if they failed to).
  const noFileLabel = fileNameSpan ? fileNameSpan.textContent : null;
  if (fileInput && fileNameSpan) {
    fileInput.addEventListener('change', function() {
      if (this.files && this.files.length > 0) {
        fileNameSpan.textContent = this.files[0].name;
        fileNameSpan.classList.add('has-file');
      } else {
        fileNameSpan.classList.remove('has-file');
        // Prefer the active translation; fall back to the original HTML
        // label rather than a hardcoded string or the raw i18n key.
        fileNameSpan.textContent = translate('no_file_chosen', noFileLabel);
      }
    });
  }

  const form = $("#secretform");

  form.addEventListener('submit', function(e) {
    e.preventDefault();

    const formData = new FormData(form);

    // Make AJAX request using fetch
    fetch('/secret', {
      method: 'POST',
      body: formData
    })
    .then(response => {
      if (!response.ok) {
        throw new Error(`Request failed with status ${response.status}: ${response.statusText}`);
      }
      return response.json();
    })
    .then(data => {
      // Show success state
      setStyles($(".success-encrypted"), {
        opacity: '1',
        pointerEvents: 'auto',
        visibility: 'visible'
      });

      // Hide form elements
      setStyles($(".encrypt"), {
        opacity: '0',
        pointerEvents: 'none',
        visibility: 'hidden'
      });

      setStyles($(".ttl"), {
        opacity: '0',
        pointerEvents: 'none',
        visibility: 'hidden'
      });

      setStyles($(".input-field"), {
        opacity: '0',
        visibility: 'hidden',
        pointerEvents: 'none'
      });

      // Hide the form heading: the success overlay carries its own title
      // and is absolutely positioned at the top of the container, where it
      // would otherwise render on top of the form's heading. Direct-children
      // selectors keep the success section's own elements visible.
      $$('.container > h1, .container > p.subtitle').forEach(element =>
        setStyles(element, {
          opacity: '0',
          pointerEvents: 'none',
          visibility: 'hidden'
        })
      );

      showURL(data.token, data.filetoken, data.filename);
    })
    .catch(error => {
      console.error(`An error occurred: ${error}`);
      // A fast failure can land before the locale load resolves; wait for it
      // so the alert is rendered in the active language when one exists.
      languageReady.then(() =>
        window.alert(translate('error_creating', 'An error occurred while creating the secret message.'))
      );
    });
  });
});

function showURL(token, filetoken, filename) {
  const urlTextarea = $("#url");

  if (filetoken) {
    urlTextarea.value = 
      `${window.location.origin}/getmsg?token=${encodeURIComponent(token)}&filetoken=${encodeURIComponent(filetoken)}&filename=${encodeURIComponent(filename)}`;
    return;
  }

  urlTextarea.value = `${window.location.origin}/getmsg?token=${encodeURIComponent(token)}`;
}
