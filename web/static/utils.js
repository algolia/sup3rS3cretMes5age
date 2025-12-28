/**
 * DOM Helper Functions
 * Provides convenient shortcuts for querySelector and querySelectorAll
 */

// Returns the first element matching the CSS selector
function $(selector) {
  return document.querySelector(selector);
}

// Returns all elements matching the CSS selector
function $$(selector) {
  return document.querySelectorAll(selector);
}
