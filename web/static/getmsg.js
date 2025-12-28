/**
 * Secret Message Retrieval Interface
 * 
 * Provides slider-based confirmation UI for retrieving one-time secret messages
 * from the /secret API endpoint. Supports both text messages and file downloads
 * with automatic base64 decoding. All event handlers are CSP-compliant.
 */


// Toggle element visibility
function toggle(element) {
    if (element.style.display === 'none' || element.style.display === '') {
        element.style.display = 'block';
    } else {
        element.style.display = 'none';
    }
}

var slider = document.getElementById("myRange");

slider.oninput = function() {
    if (this.value == 100) {
        showSecret();
    }
}

document.querySelector('.encrypt[name="newMsg"]').addEventListener('click', function() {
    window.location.href = window.location.origin;
});

function showSecret() {
    new ClipboardJS('.btn');

    let params = (new URL(window.location)).searchParams;
    console.log(window.location.origin + "/secret?token=" + params.get('token') + "&filetoken=" + params.get('filetoken') + "&filename=" + params.get('filename'));

    // Replace jQuery AJAX with fetch
    fetch(window.location.origin + "/secret?token=" + params.get('token'), {
        method: 'GET'
    })
    .then(response => {
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        return response.json();
    })
    .then(data => {
        console.log('Submission was successful.');
        console.log(data);
        showMsg(data.msg, params.get('filetoken'), params.get('filename'));
    })
    .catch(error => {
        console.log('An error occurred.');
        console.log(error);
        showMsg("Message was already deleted :(");
    });
};

function showMsg(msg, filetoken, filename) {
    // Hide progress bar if it exists
    const pbar = $('#pbar');
    if (pbar) {
        pbar.style.display = 'none';
    }

    // Set message text - use textContent for text, value for textarea
    const textarea = $('#textarea1');
    if (textarea) {
        textarea.value = msg;  // Use .value for textarea, not .textContent
    }

    if (filetoken) {
        console.log('filetoken=', filetoken);
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
    slider.value = 0;
}

function getSecret(token, name) {
    fetch(window.location.origin + "/secret?token=" + token, {
        method: 'get'
    }).then(response =>
        response.json()
    ).then(json => {
        saveData(json.msg, name);
    }).catch(function (err) {
        console.error(err);
    });
}

var saveData = (function () {
    var a = document.createElement("a");
    document.body.appendChild(a);
    a.style = "display: none";
    return function (data, fileName) {
        console.log("data=", data);
        console.log("fileName=", fileName);
        var blob = b64toBlob([data], { type: "octet/stream" })
        var url = window.URL.createObjectURL(blob);
        a.href = url;
        a.download = fileName;
        a.click();
        window.URL.revokeObjectURL(url);
    };
}());

function b64toBlob(b64Data, contentType, sliceSize) {
    sliceSize = sliceSize || 512;

    var byteCharacters = atob(b64Data);
    var byteArrays = [];

    for (var offset = 0; offset < byteCharacters.length; offset += sliceSize) {
        var slice = byteCharacters.slice(offset, offset + sliceSize);

        var byteNumbers = new Array(slice.length);
        for (var i = 0; i < slice.length; i++) {
            byteNumbers[i] = slice.charCodeAt(i);
        }

        var byteArray = new Uint8Array(byteNumbers);

        byteArrays.push(byteArray);
    }

    return new Blob(byteArrays, {type: contentType});
}
