/**
 * Utility Functions Module
 * 
 * This module provides core utility functions for the sup3rS3cretMes5age application:
 * 
 * DOM Helpers:
 * - $() and $$(): jQuery-like selectors for querySelector and querySelectorAll
 * 
 * Internationalization (i18n):
 * - detectLanguage(): Auto-detects user language from URL, browser, or defaults to English
 * - isValidLanguage(): Validates if a language code is supported (en, fr, de, es, it)
 * - loadTranslations(): Fetches and applies translation JSON files dynamically
 * - applyTranslations(): Updates DOM elements with data-i18n attributes
 * - updateMetaTags(): Updates document title and meta descriptions for SEO
 * - switchLanguage(): Changes active language with URL persistence
 * 
 * All functions are exported as ES6 modules and are CSP-compliant.
 */

// Returns the first element matching the CSS selector
export function $(selector) {
  return document.querySelector(selector);
}

// Returns all elements matching the CSS selector
export function $$(selector) {
  return document.querySelectorAll(selector);
}

// Language management functions - simplified and fixed
export function detectLanguage() {
    // Check URL parameter first
    const urlParams = new URLSearchParams(window.location.search);
    const langParam = urlParams.get('lang');
    if (langParam && isValidLanguage(langParam)) {
        return langParam;
    }

    // Check browser language preference
    const browserLang = navigator.language || navigator.userLanguage;
    const langCode = browserLang.split('-')[0];
    if (isValidLanguage(langCode)) {
        return langCode;
    }

    // Default to English
    return 'en';
}

// Validate if the language is supported
export function isValidLanguage(lang) {
    const validLanguages = ['en', 'fr', 'de', 'es', 'it'];
    return validLanguages.includes(lang);
}

// Load translations for the specified language
export async function loadTranslations(language) {
    try {
        const response = await fetch(`/static/locales/${language}.json`);
        const translations = await response.json();

        // Store translations in a global object
        window.translations = translations;

        // Apply translations to current page
        applyTranslations();

        return translations;
    } catch (error) {
        console.error(`Failed to load translations for ${language}:`, error);
        // Fall back to English
        return loadTranslations('en');
    }
}

// Apply translations to the page elements with data-i18n attributes
export function applyTranslations() {
    // Translate elements with data-i18n attribute
    const elements = $$('[data-i18n]');
    elements.forEach(element => {
        const key = element.getAttribute('data-i18n');
        const translation = window.translations?.[key] || key;

        if (element.tagName === 'INPUT' || element.tagName === 'TEXTAREA') {
            element.placeholder = translation;
        } else {
            element.textContent = translation;
        }
    });

    // Update meta tags
    updateMetaTags();
}

// Update meta title and description based on translations
export function updateMetaTags() {
    const title = window.translations?.['meta_title'] || 'sup3rS3cretMes5age';
    const description = window.translations?.['meta_description'] || 'Send self-destructing one-time secret messages securely.';

    // Update standard meta tags
    const descMeta = $('meta[name="description"]');
    if (descMeta) {
        descMeta.setAttribute('content', description);
    }

    const titleElement = $('title');
    if (titleElement) {
        titleElement.textContent = title;
    }

    // Update Open Graph meta tags
    const ogTitle = $('meta[property="og:title"]');
    if (ogTitle) {
        ogTitle.setAttribute('content', title);
    }

    const ogDescription = $('meta[property="og:description"]');
    if (ogDescription) {
        ogDescription.setAttribute('content', description);
    }
}

// Switch language and reload translations
export function switchLanguage(newLanguage) {
    if (isValidLanguage(newLanguage)) {
        loadTranslations(newLanguage);

        // Update HTML lang attribute for accessibility
        document.documentElement.setAttribute('lang', newLanguage);

        // Update language selector value
        const languageSelect = document.getElementById('language-select');
        if (languageSelect && languageSelect.value !== newLanguage) {
            languageSelect.value = newLanguage;
        }

        // Update URL with language parameter
        const url = new URL(window.location);
        url.searchParams.set('lang', newLanguage);
        window.history.pushState({}, '', url);
    }
}

// Setup language on initial load
export function setupLanguage() {
    
  const currentLanguage = detectLanguage();
  loadTranslations(currentLanguage);

  // Set HTML lang attribute and selector value
  document.documentElement.setAttribute('lang', currentLanguage);

  // Set up global language manager
  window.langManager = {
          currentLanguage: currentLanguage,
          switchLanguage: switchLanguage,
          translate: function(key) {
              return window.translations?.[key] || key;
          }
      };

  const languageSelect = document.getElementById('language-select');
    
  if (languageSelect) {
    // Ensure selector reflects current language
    if (languageSelect.value !== currentLanguage) {
      languageSelect.value = currentLanguage;
    }
    // Add event listener for language selector (CSP-compliant)
    languageSelect.addEventListener('change', function() {
      switchLanguage(this.value);
    });
  }
}
