import { FormEvent, useMemo, useState } from "react";
import { submitStreamer } from "../api";
import type { StreamerStatus, SubmissionPayload } from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";
import {
  createPlatformRow,
  PlatformFormRow,
  sanitizePlatforms
} from "../utils/formHelpers";

interface SubmitStreamerFormProps {
  isOpen: boolean;
  onToggle: () => void;
}

type PlatformField = "name" | "channelUrl" | "liveUrl";

const DEFAULT_STATUS: StreamerStatus = "offline";
const TOP_LANGUAGES = [
  "English",
  "Mandarin Chinese",
  "Hindi",
  "Spanish",
  "French",
  "Arabic",
  "Bengali",
  "Russian",
  "Portuguese",
  "Indonesian"
];

const ADDITIONAL_LANGUAGES = [
  "Afrikaans",
  "Albanian",
  "Amharic",
  "Armenian",
  "Azerbaijani",
  "Basque",
  "Belarusian",
  "Bosnian",
  "Bulgarian",
  "Catalan",
  "Cebuano",
  "Croatian",
  "Czech",
  "Danish",
  "Dutch",
  "Estonian",
  "Filipino",
  "Finnish",
  "Galician",
  "Georgian",
  "German",
  "Greek",
  "Gujarati",
  "Haitian Creole",
  "Hebrew",
  "Hmong",
  "Hungarian",
  "Icelandic",
  "Igbo",
  "Italian",
  "Japanese",
  "Javanese",
  "Kannada",
  "Kazakh",
  "Khmer",
  "Kinyarwanda",
  "Korean",
  "Kurdish",
  "Lao",
  "Latvian",
  "Lithuanian",
  "Luxembourgish",
  "Macedonian",
  "Malay",
  "Malayalam",
  "Maltese",
  "Marathi",
  "Mongolian",
  "Nepali",
  "Norwegian",
  "Pashto",
  "Persian",
  "Polish",
  "Punjabi",
  "Romanian",
  "Serbian",
  "Sinhala",
  "Slovak",
  "Slovenian",
  "Somali",
  "Swahili",
  "Swedish",
  "Tamil",
  "Telugu",
  "Thai",
  "Turkish",
  "Ukrainian",
  "Urdu",
  "Uzbek",
  "Vietnamese",
  "Welsh",
  "Xhosa",
  "Yoruba",
  "Zulu"
];

const LANGUAGE_ORDER = [
  ...TOP_LANGUAGES,
  ...ADDITIONAL_LANGUAGES.filter((language) => !TOP_LANGUAGES.includes(language)).sort((a, b) =>
    a.localeCompare(b)
  )
];

export function SubmitStreamerForm({ isOpen, onToggle }: SubmitStreamerFormProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedLanguages, setSelectedLanguages] = useState<string[]>([]);
  const [languageSelection, setLanguageSelection] = useState("");
  const [platforms, setPlatforms] = useState<PlatformFormRow[]>([createPlatformRow()]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resultMessage, setResultMessage] = useState("");
  const [resultState, setResultState] = useState<"idle" | "success" | "error">("idle");

  const availableLanguages = useMemo(
    () => LANGUAGE_ORDER.filter((language) => !selectedLanguages.includes(language)),
    [selectedLanguages]
  );

  const canSubmit = useMemo(() => {
    if (!name.trim() || !description.trim()) {
      return false;
    }
    if (!selectedLanguages.length) {
      return false;
    }
    if (
      !platforms.some(
        (platform) =>
          platform.name.trim() && platform.channelUrl.trim() && platform.liveUrl.trim()
      )
    ) {
      return false;
    }
    return true;
  }, [description, name, platforms, selectedLanguages]);

  const resetForm = () => {
    setName("");
    setDescription("");
    setSelectedLanguages([]);
    setLanguageSelection("");
    setPlatforms([createPlatformRow()]);
  };

  const handlePlatformChange = (id: string, key: PlatformField, value: string): void => {
    setPlatforms((current) =>
      current.map((row) => (row.id === id ? { ...row, [key]: value } : row))
    );
  };

  const handleRemovePlatform = (id: string) => {
    setPlatforms((current) => {
      if (current.length === 1) {
        return [createPlatformRow()];
      }
      return current.filter((row) => row.id !== id);
    });
  };

  const handleLanguageSelect = (event: FormEvent<HTMLSelectElement>) => {
    const value = event.currentTarget.value;
    if (!value) {
      return;
    }
    setSelectedLanguages((current) => [...current, value]);
    setLanguageSelection("");
  };

  const handleLanguageRemove = (language: string) => {
    setSelectedLanguages((current) => current.filter((item) => item !== language));
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setResultState("idle");
    setResultMessage("");

    const submission: SubmissionPayload = {
      name: name.trim(),
      description: description.trim(),
      status: DEFAULT_STATUS,
      statusLabel: STATUS_DEFAULT_LABELS[DEFAULT_STATUS],
      languages: selectedLanguages,
      platforms: sanitizePlatforms(platforms)
    };

    try {
      const result = await submitStreamer(submission);
      setResultMessage(result.message || "Submission received and queued for review.");
      setResultState("success");
      resetForm();
    } catch (error) {
      setResultMessage(error instanceof Error ? error.message : "Submission failed.");
      setResultState("error");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <section className="submit-streamer" aria-labelledby="submit-streamer-title">
      <div className="submit-streamer-header">
        <h2 id="submit-streamer-title">Know a streamer we should feature?</h2>
        <button type="button" className="submit-streamer-toggle" onClick={onToggle}>
          {isOpen ? "Hide form" : "Submit a streamer"}
        </button>
      </div>

      {isOpen ? (
        <form
          className="submit-streamer-form"
          onSubmit={handleSubmit}
          aria-live="polite"
        >
          <p className="submit-streamer-help">
            Share the details below and our team will review the submission before adding the
            streamer to the roster. No additional access is required.
          </p>

          <div className="form-grid">
            <label className="form-field">
              <span>Streamer name *</span>
              <input
                type="text"
                name="streamer-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </label>

            <label className="form-field form-field-wide">
              <span>Description *</span>
              <textarea
                name="description"
                rows={3}
                placeholder="What makes this streamer unique?"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                required
              />
            </label>

            <label className="form-field form-field-wide">
              <span>Languages *</span>
              <div className="language-picker">
                <select
                  className="language-select"
                  value={languageSelection}
                  onChange={handleLanguageSelect}
                  required={!selectedLanguages.length}
                >
                  <option value="" disabled>
                    Select a language
                  </option>
                  {availableLanguages.map((language) => (
                    <option key={language} value={language}>
                      {language}
                    </option>
                  ))}
                </select>
                <div className="language-tags">
                  {selectedLanguages.length ? (
                    selectedLanguages.map((language) => (
                      <span className="language-pill" key={language}>
                        {language}
                        <button
                          type="button"
                          onClick={() => handleLanguageRemove(language)}
                          aria-label={`Remove ${language}`}
                        >
                          ×
                        </button>
                      </span>
                    ))
                  ) : (
                    <span className="language-empty">No languages selected yet.</span>
                  )}
                </div>
              </div>
            </label>
          </div>

          <fieldset className="platform-fieldset">
            <legend>Streaming platforms *</legend>
            <p className="submit-streamer-help">
              Add each platform&rsquo;s name along with the regular channel URL and live stream URL.
              If they&rsquo;re the same, repeat the link in both fields.
            </p>

            <div className="platform-rows">
              {platforms.map((platform) => (
                <div className="platform-row" key={platform.id} data-platform-row>
                  <label className="form-field form-field-inline">
                    <span>Platform name</span>
                    <input
                      type="text"
                      name="platform-name"
                      value={platform.name}
                      onChange={(event) =>
                        handlePlatformChange(platform.id, "name", event.target.value)
                      }
                      required
                    />
                  </label>
                  <label className="form-field form-field-inline">
                    <span>Channel URL</span>
                    <input
                      type="url"
                      name="platform-channel"
                      placeholder="https://"
                      value={platform.channelUrl}
                      onChange={(event) =>
                        handlePlatformChange(platform.id, "channelUrl", event.target.value)
                      }
                      required
                    />
                  </label>
                  <label className="form-field form-field-inline">
                    <span>Live stream URL</span>
                    <input
                      type="url"
                      name="platform-live"
                      placeholder="https://"
                      value={platform.liveUrl}
                      onChange={(event) =>
                        handlePlatformChange(platform.id, "liveUrl", event.target.value)
                      }
                      required
                    />
                  </label>
                  <button
                    type="button"
                    className="remove-platform-button"
                    onClick={() => handleRemovePlatform(platform.id)}
                  >
                    Remove
                  </button>
                </div>
              ))}
            </div>

            <button
              type="button"
              className="add-platform-button"
              onClick={() => setPlatforms((current) => [...current, createPlatformRow()])}
            >
              + Add another platform
            </button>
          </fieldset>

          <div className="submit-streamer-actions">
            <button type="submit" className="submit-streamer-submit" disabled={!canSubmit || isSubmitting}>
              {isSubmitting ? "Submitting…" : "Submit streamer"}
            </button>
            <button
              type="button"
              className="submit-streamer-cancel"
              onClick={() => {
                resetForm();
                setResultMessage("");
                setResultState("idle");
                onToggle();
              }}
            >
              Cancel
            </button>
          </div>

          {resultState !== "idle" ? (
            <div
              className="submit-streamer-result"
              role="status"
              data-state={resultState}
            >
              {resultMessage}
            </div>
          ) : null}
        </form>
      ) : null}
    </section>
  );
}
