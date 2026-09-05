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
// Request ID counter to prevent race conditions in language switching
let translationRequestId = 0;

export function detectLanguage() {
  // Check URL parameter first (region subtags and case normalized away)
  const urlParams = new URLSearchParams(window.location.search);
  const langParam = primaryLanguageTag(urlParams.get('lang'));
  if (isValidLanguage(langParam)) {
    return langParam;
  }

  // Check browser language preference
  const browserLang = navigator.language || navigator.userLanguage;
  const langCode = primaryLanguageTag(browserLang);
  if (isValidLanguage(langCode)) {
    return langCode;
  }

  // Default to English
  return 'en';
}

// Supported languages, initialized from locales-manifest.json (the single
// source of truth, shared with the server) by setupLanguage. Empty until the
// manifest loads, so nothing is considered valid before that.
let supportedLanguages = [];

// Load the language manifest. On failure the list stays empty and every
// language resolves to the English default — the original HTML text stays
// visible, matching the locale-fetch failure behavior.
async function loadLanguageManifest() {
  try {
    const response = await fetch('/static/locales-manifest.json');
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status} while loading /static/locales-manifest.json`);
    }
    const manifest = await response.json();
    if (Array.isArray(manifest.languages) && manifest.languages.length > 0) {
      supportedLanguages = manifest.languages;
    }
  } catch (error) {
    console.error('Failed to load language manifest:', error);
  }
}

// Native display name for a language code (e.g. "de" -> "Deutsch"), provided
// by the browser's CLDR data so new languages need no label table.
function nativeLanguageName(code) {
  try {
    const name = new Intl.DisplayNames([code], { type: 'language' }).of(code);
    return name.charAt(0).toUpperCase() + name.slice(1);
  } catch {
    return code.toUpperCase();
  }
}

// Normalize a language tag to its primary subtag, mirroring the server-side
// primaryLanguageTag(): "fr-CA" -> "fr", "FR" -> "fr". Keeps client-side
// detection consistent with the server's Content-Language decision.
function primaryLanguageTag(tag) {
  return String(tag ?? '').trim().toLowerCase().split(/[-_]/)[0];
}

// Validate if the language is supported
export function isValidLanguage(lang) {
  return supportedLanguages.includes(lang);
}

// Load translations for the specified language
// requestId guards against race conditions from rapid language switching.
// Returns the language whose translations were actually applied (which may
// differ from the requested one if the fetch failed and English was used as
// fallback), or null when a newer request superseded this one.
export async function loadTranslations(language, requestId = null) {
  try {
    const response = await fetch(`/static/locales/${language}.json`);
    if (!response.ok) {
      throw new Error(`HTTP error ${response.status} while loading /static/locales/${language}.json`);
    }

    const translations = await response.json();

    // Guard against stale requests: only apply if this is the latest request
    if (requestId !== null && requestId !== translationRequestId) {
      // Discarding stale translation request
      return null;
    }

    // Store translations in a global object
    window.translations = translations;

    // Apply translations to current page
    applyTranslations();

    return language;
  } catch (error) {
    console.error(`Failed to load translations for ${language}:`, error);
    // If English (fallback) also fails, avoid infinite recursion.
    // Keep whatever translations are already loaded: applyTranslations()
    // skips keys without a translation, so the original HTML text stays
    // visible instead of being replaced by raw i18n keys.
    if (language === 'en') {
      if (!window.translations) {
        window.translations = {};
      }
      if (requestId !== null && requestId !== translationRequestId) {
        return null;
      }
      return 'en';
    }
    // Fall back to English
    return loadTranslations('en', requestId);
  }
}

// Own-property-only lookup: enumerates the loaded translations instead of
// using computed key access, so inherited names ("constructor", "__proto__")
// can never resolve and no dynamic object-injection sink exists.
function lookupTranslation(key) {
  const entry = Object.entries(window.translations ?? {}).find(([name]) => name === key);
  return entry ? entry[1] : undefined;
}

// Translate a key using the loaded locale. Falls back to the provided
// default (typically the English string) when translations are missing or
// not yet loaded, so dynamic strings degrade to content, never key names.
export function translate(key, fallback) {
  return lookupTranslation(key) || fallback || key;
}

// Apply translations to the page elements with data-i18n attributes
export function applyTranslations() {
  // Translate elements with data-i18n attribute
  const elements = $$('[data-i18n]');
  elements.forEach(element => {
    const key = element.getAttribute('data-i18n');
    const translation = lookupTranslation(key);

    // Skip keys with no translation: writing the raw key would replace
    // valid content. A missing key degrades gracefully to the original
    // HTML text (or the previously applied language), never to key names.
    if (!translation) {
      return;
    }

    // Skip elements currently showing dynamic content (e.g. a chosen file
    // name) so a language switch does not clobber it with a static label.
    if (element.classList.contains('has-file')) {
      return;
    }

    if (element.tagName === 'INPUT' || element.tagName === 'TEXTAREA') {
      element.placeholder = translation;
    } else if (element.tagName === 'META') {
      element.setAttribute('content', translation);
    } else {
      element.textContent = translation;
    }
  });

  // Update meta tags
  updateMetaTags();
}

// Update meta title and description based on translations
export function updateMetaTags() {
  const title = lookupTranslation('meta_title') || 'sup3rS3cretMes5age';
  const description = lookupTranslation('meta_description') || 'Send self-destructing one-time secret messages securely.';

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
// async to properly await translation loading and prevent race conditions
export async function switchLanguage(newLanguage) {
  if (!isValidLanguage(newLanguage)) {
    return;
  }

  // Increment request ID to invalidate any in-flight requests
  const currentRequestId = ++translationRequestId;

  // Update URL with language parameter: this records the user's intent, so
  // a reload retries the requested language even if this attempt falls back.
  const url = new URL(window.location);
  url.searchParams.set('lang', newLanguage);
  window.history.pushState({}, '', url);

  // Load translations with request ID to guard against race conditions
  const appliedLanguage = await loadTranslations(newLanguage, currentRequestId);

  // Nothing superseded us: reflect the language actually rendered. If the
  // fetch failed and English was used as fallback, the UI state must say
  // English too — otherwise the selector and <html lang> claim a language
  // that is not displayed.
  if (appliedLanguage === null) {
    return;
  }

  // Update HTML lang attribute for accessibility
  document.documentElement.setAttribute('lang', appliedLanguage);

  // Update language selector value
  const languageSelect = document.getElementById('language-select');
  if (languageSelect && languageSelect.value !== appliedLanguage) {
    languageSelect.value = appliedLanguage;
  }
}

// Setup language on initial load
export async function setupLanguage() {

  // Load the language list before detection/translation: it drives both
  await loadLanguageManifest();

  const currentLanguage = detectLanguage();

  // Increment request ID and use it for the initial load to avoid races
  const currentRequestId = ++translationRequestId;
  const appliedLanguage = await loadTranslations(currentLanguage, currentRequestId);

  // If a newer language request was made while we were loading, abort
  if (currentRequestId !== translationRequestId || appliedLanguage === null) {
    return;
  }

  // Set HTML lang attribute and selector value from the language actually
  // rendered (may be the English fallback if the requested one failed)
  document.documentElement.setAttribute('lang', appliedLanguage);

  const languageSelect = document.getElementById('language-select');

  if (languageSelect) {
    // Build the selector from the manifest — the same source of truth the
    // server uses — instead of hardcoded <option> markup.
    languageSelect.replaceChildren(...supportedLanguages.map(code => {
      const option = document.createElement('option');
      option.value = code;
      option.textContent = nativeLanguageName(code);
      return option;
    }));
    // Ensure selector reflects current language
    if (languageSelect.value !== appliedLanguage) {
      languageSelect.value = appliedLanguage;
    }
    // Add event listener for language selector (CSP-compliant)
    languageSelect.addEventListener('change', function() {
      switchLanguage(this.value);
    });
  }
}
