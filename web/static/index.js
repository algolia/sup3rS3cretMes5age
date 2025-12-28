/**
 * Secret Message Creation Interface
 *
 * Processes message creation requests with optional file uploads and custom TTL.
 * Submits data to /secret API endpoint and returns a shareable one-time link.
 * All event handlers are CSP-compliant.
 */

// CSS manipulation helper
function setStyles(element, styles) {
  Object.assign(element.style, styles);
}

// Form submission handler
document.addEventListener('DOMContentLoaded', function() {
  // Initialize clipboard functionality
  new ClipboardJS('.btn');
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
        throw new Error('Request failed with status ' + response.status + ': ' + response.statusText);
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
      console.error('Error:', error);
      alert('An error occurred while creating the secret message.');
    });
  });
});

function showURL(token, filetoken, filename) {
  const urlTextarea = $("#url");

  if (filetoken) {
    urlTextarea.textContent = 
      `${window.location.origin}/getmsg?token=${encodeURIComponent(token)}&filetoken=${encodeURIComponent(filetoken)}&filename=${encodeURIComponent(filename)}`;
    return;
  }

  urlTextarea.textContent = `${window.location.origin}/getmsg?token=${encodeURIComponent(token)}`;
}
