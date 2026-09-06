/**
 * Secret Message Retrieval Interface
 * 
 * Provides slider-based confirmation UI for retrieving one-time secret messages
 * from the /secret API endpoint. Supports both text messages and file downloads
 * with automatic base64 decoding. All event handlers are CSP-compliant.
 */

import { $, isValidLanguage, primaryLanguageTag, setupLanguage, translate, whenLanguageReady } from './utils.js';

// Initialize clipboard and language manager on DOMContentLoaded
document.addEventListener('DOMContentLoaded', function() {
    // Initialize clipboard functionality
    new ClipboardJS('.btn');

    // Initialize language manager; dynamic error rendering awaits it through
    // whenLanguageReady(), which also covers in-flight language switches
    setupLanguage();
});

// Slider input handler
document.getElementById("myRange").addEventListener('input', function() {
    if (this.value === '100') { // slider.value returns string
        showSecret();
    }
});

// New message button handler
$('.encrypt[name="newMsg"]').addEventListener('click', function() {
    // Navigate straight to /msg rather than through the root redirect
    // (which now forwards the query string): the current URL carries the
    // consumed one-time token/filetoken/filename parameters, and they must
    // not leak into the creation page's navigation history. Keep the active
    // language so the creation page renders in the same language. Only
    // known language codes are propagated (normalized via
    // primaryLanguageTag), and assign() is used instead of a location.href
    // assignment (a pattern flagged as XSS-prone).
    const url = new URL('/msg', window.location.origin);
    const normalizedLang = primaryLanguageTag(new URL(window.location).searchParams.get('lang'));
    if (isValidLanguage(normalizedLang)) {
        url.searchParams.set('lang', normalizedLang);
    }
    window.location.assign(url.toString());
});

// Validate and construct secret URL from token
function validateSecretUrl(token) {
    // Validate token format
    if (!token || typeof token !== 'string' || !/^[A-Za-z0-9_\-\.]+$/.test(token)) {
        console.error('Invalid token format');
        showMsg(translate('invalid_token', 'Invalid or missing token'));
        return null;
    }

    // Properly encode URL parameters
    const url = new URL('/secret', window.location.origin);
    url.searchParams.set('token', token);
    return url.toString();
}

// Fetch and display the secret message
async function showSecret() {
    // Dynamic error strings must render in the active language: a fast
    // failure (invalid/expired token) can otherwise land before the locale
    // load resolves and write English into the textarea, and a switch still
    // in flight would render the previous language. Awaiting a settled
    // promise resolves immediately, so this is free in the common case.
    await whenLanguageReady();

    const params = (new URL(window.location)).searchParams;

    const urlStr = validateSecretUrl(params.get('token'));
    if (!urlStr) {
        return;
    }

    // Replace jQuery AJAX with fetch
    fetch(urlStr, {
        method: 'GET'
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        showMsg(data.msg, params.get('filetoken'), params.get('filename'));
    })
    .catch(error => {
        console.error(`An error occurred: ${error}`);
        showMsg(translate('message_already_deleted', 'Message was already deleted :('));
    });
};

// Display the secret message and handle file download if applicable
function showMsg(msg, filetoken, filename) {
    // Hide progress bar if it exists
    const pbar = $('#pbar');
    if (pbar) {
        pbar.style.display = 'none';
    }

    // Set message text
    const textarea = $('#textarea1');
    if (textarea) {
        textarea.value = msg;
    }

    if (filetoken) {
        getSecret(filetoken, filename);
    }

    // Hide slider
    const slideContainer = $('.slidecontainer');
    if (slideContainer) {
        slideContainer.style.display = 'none';
    }

    // Show secret text box
    const inputField = $('.input-field');
    if (inputField) {
        inputField.style.display = 'block';
    }

    // Show copy to clipboard button
    const buttonDiv = $('.button');
    if (buttonDiv) {
        buttonDiv.style.display = 'block';
    }

    // Reset slider (in case of back button)
    document.getElementById("myRange").value = 0;
}

// Fetch the secret file and trigger download
function getSecret(token, name) {
    const urlStr = validateSecretUrl(token);
    if (!urlStr) {
        return;
    }

    fetch(urlStr, {
        method: 'get'
    }).then(response =>
        response.json()
    ).then(json => {
        saveData(json.msg, name);
    }).catch(function (err) {
        console.error(`An error occurred: ${err}`);
    });
}

var saveData = (function () {
    const a = document.createElement("a");
    document.body.appendChild(a);
    a.style.display = "none";
    return function (data, fileName) {
        const blob = b64toBlob([data], "application/octet-stream")
        const url = window.URL.createObjectURL(blob);
        a.href = url;
        a.download = fileName;
        a.click();
        window.URL.revokeObjectURL(url);
    };
}());

// Convert base64 string to Blob
function b64toBlob(b64Data, contentType, sliceSize) {
    sliceSize = sliceSize || 512;

    const byteCharacters = atob(b64Data);
    const byteArrays = [];

    for (let offset = 0; offset < byteCharacters.length; offset += sliceSize) {
        const slice = byteCharacters.slice(offset, offset + sliceSize);

        // Use Uint8Array directly without intermediate Array to avoid object injection
        const byteArray = new Uint8Array(slice.length);
        for (let i = 0; i < slice.length; i++) {
            byteArray[i] = slice.charCodeAt(i);
        }

        byteArrays.push(byteArray);
    }

    return new Blob(byteArrays, {type: contentType});
}
