/**
 * Secret Message Creation Interface
 *
 * Processes message creation requests with optional file uploads and custom TTL.
 * Submits data to /secret API endpoint and returns a shareable one-time link.
 * All event handlers are CSP-compliant.
 */

import { $, setupLanguage } from './utils.js';

// CSS manipulation helper
function setStyles(element, styles) {
  Object.assign(element.style, styles);
}

// Form submission handler
document.addEventListener('DOMContentLoaded', function() {
  // Initialize clipboard functionality
  new ClipboardJS('.btn');

  // Initialize language manager
  setupLanguage();

  // Custom file input handler
  const fileInput = document.getElementById('file-input');
  const fileNameSpan = $('.file-name');
  if (fileInput && fileNameSpan) {
    fileInput.addEventListener('change', function() {
      if (this.files && this.files.length > 0) {
        fileNameSpan.textContent = this.files[0].name;
        fileNameSpan.classList.add('has-file');
      } else {
        fileNameSpan.textContent = window.langManager?.translate('no_file_chosen') || 'No file chosen';
        fileNameSpan.classList.remove('has-file');
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

      showURL(data.token, data.filetoken, data.filename);
    })
    .catch(error => {
      console.error(`An error occurred: ${error}`);
      alert('An error occurred while creating the secret message.');
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
