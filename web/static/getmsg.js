/**
 * Secret Message Retrieval Interface
 * 
 * Provides slider-based confirmation UI for retrieving one-time secret messages
 * from the /secret API endpoint. Supports both text messages and file downloads
 * with automatic base64 decoding. All event handlers are CSP-compliant.
 */

// Initialize clipboard functionality
document.addEventListener('DOMContentLoaded', function() {
    new ClipboardJS('.btn');
});

// slider.oninput
document.getElementById("myRange").addEventListener('input', function() {
    if (this.value === '100') { // slider.value returns string
        showSecret();
    }
});

document.querySelector('.encrypt[name="newMsg"]').addEventListener('click', function() {
    window.location.href = window.location.origin;
});

function showSecret() {
    let params = (new URL(window.location)).searchParams;

    // Replace jQuery AJAX with fetch
    fetch(`${window.location.origin}/secret?token=${params.get('token')}`, {
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
        showMsg("Message was already deleted :(");
    });
};

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

function getSecret(token, name) {
    fetch(`${window.location.origin}/secret?token=${token}`, {
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
        const blob = b64toBlob([data], { type: "octet/stream" })
        const url = window.URL.createObjectURL(blob);
        a.href = url;
        a.download = fileName;
        a.click();
        window.URL.revokeObjectURL(url);
    };
}());

function b64toBlob(b64Data, contentType, sliceSize) {
    sliceSize = sliceSize || 512;

    const byteCharacters = atob(b64Data);
    let byteArrays = [];

    for (let offset = 0; offset < byteCharacters.length; offset += sliceSize) {
        const slice = byteCharacters.slice(offset, offset + sliceSize);

        let byteNumbers = new Array(slice.length);
        for (let i = 0; i < slice.length; i++) {
            byteNumbers[i] = slice.charCodeAt(i);
        }

        const byteArray = new Uint8Array(byteNumbers);
        byteArrays.push(byteArray);
    }

    return new Blob(byteArrays, {type: contentType});
}
