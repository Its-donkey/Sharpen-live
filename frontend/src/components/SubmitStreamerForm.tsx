import { FormEvent, useMemo, useState } from "react";
import { submitStreamer } from "../api";
import type { StreamerStatus, SubmissionPayload } from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";
import {
  PLATFORM_PRESETS,
  createPlatformRow,
  PlatformFormRow,
  sanitizePlatforms
} from "../utils/formHelpers";

interface SubmitStreamerFormProps {
  isOpen: boolean;
  onToggle: () => void;
}

type PlatformField = "name" | "channelUrl";
type PlatformErrorState = Record<PlatformField, boolean>;

interface ValidationErrors {
  name: boolean;
  description: boolean;
  languages: boolean;
  platforms: Record<string, PlatformErrorState>;
}

const DEFAULT_STATUS: StreamerStatus = "offline";
const TOP_LANGUAGES = [{ label: "English", value: "English" }];

const ADDITIONAL_LANGUAGES = [
  { label: "Afrikaans", value: "Afrikaans" },
  { label: "Albanian / Shqip", value: "Albanian" },
  { label: "Amharic / አማርኛ (Amariññā)", value: "Amharic" },
  { label: "Armenian / Հայերեն (Hayeren)", value: "Armenian" },
  { label: "Azerbaijani / Azərbaycanca", value: "Azerbaijani" },
  { label: "Basque / Euskara", value: "Basque" },
  { label: "Belarusian / Беларуская (Belaruskaya)", value: "Belarusian" },
  { label: "Bosnian / Bosanski", value: "Bosnian" },
  { label: "Bulgarian / Български (Bŭlgarski)", value: "Bulgarian" },
  { label: "Catalan / Català", value: "Catalan" },
  { label: "Cebuano / Binisaya", value: "Cebuano" },
  { label: "Croatian / Hrvatski", value: "Croatian" },
  { label: "Czech / Čeština", value: "Czech" },
  { label: "Danish / Dansk", value: "Danish" },
  { label: "Dutch / Nederlands", value: "Dutch" },
  { label: "Estonian / Eesti", value: "Estonian" },
  { label: "Filipino / Tagalog", value: "Filipino" },
  { label: "Finnish / Suomi", value: "Finnish" },
  { label: "Galician / Galego", value: "Galician" },
  { label: "Georgian / ქართული (Kartuli)", value: "Georgian" },
  { label: "German / Deutsch", value: "German" },
  { label: "Greek / Ελληνικά (Elliniká)", value: "Greek" },
  { label: "Gujarati / ગુજરાતી (Gujarātī)", value: "Gujarati" },
  { label: "Haitian Creole / Kreyòl Ayisyen", value: "Haitian Creole" },
  { label: "Hebrew / עברית (Ivrit)", value: "Hebrew" },
  { label: "Hmong / Hmoob", value: "Hmong" },
  { label: "Hungarian / Magyar", value: "Hungarian" },
  { label: "Icelandic / Íslenska", value: "Icelandic" },
  { label: "Igbo", value: "Igbo" },
  { label: "Italian / Italiano", value: "Italian" },
  { label: "Japanese / 日本語 (Nihongo)", value: "Japanese" },
  { label: "Javanese / Basa Jawa", value: "Javanese" },
  { label: "Kannada / ಕನ್ನಡ (Kannaḍa)", value: "Kannada" },
  { label: "Kazakh / Қазақ (Qazaq)", value: "Kazakh" },
  { label: "Khmer / ខ្មែរ (Khmer)", value: "Khmer" },
  { label: "Kinyarwanda", value: "Kinyarwanda" },
  { label: "Korean / 한국어 (Hangugeo)", value: "Korean" },
  { label: "Kurdish / کوردی", value: "Kurdish" },
  { label: "Lao / ລາວ", value: "Lao" },
  { label: "Latvian / Latviešu", value: "Latvian" },
  { label: "Lithuanian / Lietuvių", value: "Lithuanian" },
  { label: "Luxembourgish / Lëtzebuergesch", value: "Luxembourgish" },
  { label: "Macedonian / Македонски (Makedonski)", value: "Macedonian" },
  { label: "Malay / Bahasa Melayu", value: "Malay" },
  { label: "Malayalam / മലയാളം (Malayāḷam)", value: "Malayalam" },
  { label: "Maltese / Malti", value: "Maltese" },
  { label: "Marathi / मराठी (Marāṭhī)", value: "Marathi" },
  { label: "Mongolian / Монгол (Mongol)", value: "Mongolian" },
  { label: "Nepali / नेपाली (Nepālī)", value: "Nepali" },
  { label: "Norwegian / Norsk", value: "Norwegian" },
  { label: "Pashto / پښتو", value: "Pashto" },
  { label: "Persian / فارسی (Fārsi)", value: "Persian" },
  { label: "Polish / Polski", value: "Polish" },
  { label: "Punjabi / ਪੰਜਾਬੀ (Pañjābī)", value: "Punjabi" },
  { label: "Romanian / Română", value: "Romanian" },
  { label: "Serbian / Српски (Srpski)", value: "Serbian" },
  { label: "Sinhala / සිංහල (Siṁhala)", value: "Sinhala" },
  { label: "Slovak / Slovenčina", value: "Slovak" },
  { label: "Slovenian / Slovenščina", value: "Slovenian" },
  { label: "Somali / Soomaali", value: "Somali" },
  { label: "Swahili / Kiswahili", value: "Swahili" },
  { label: "Swedish / Svenska", value: "Swedish" },
  { label: "Tamil / தமிழ் (Tamiḻ)", value: "Tamil" },
  { label: "Telugu / తెలుగు (Telugu)", value: "Telugu" },
  { label: "Thai / ไทย", value: "Thai" },
  { label: "Turkish / Türkçe", value: "Turkish" },
  { label: "Ukrainian / Українська (Ukrayins'ka)", value: "Ukrainian" },
  { label: "Urdu / اردو", value: "Urdu" },
  { label: "Uzbek / Oʻzbek", value: "Uzbek" },
  { label: "Vietnamese / Tiếng Việt", value: "Vietnamese" },
  { label: "Welsh / Cymraeg", value: "Welsh" },
  { label: "Xhosa / isiXhosa", value: "Xhosa" },
  { label: "Yoruba / Yorùbá", value: "Yoruba" },
  { label: "Zulu / isiZulu", value: "Zulu" }
];

const LANGUAGE_ORDER = [
  ...TOP_LANGUAGES,
  ...ADDITIONAL_LANGUAGES.sort((a, b) => a.label.localeCompare(b.label))
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
  const [validationErrors, setValidationErrors] = useState<ValidationErrors>({
    name: false,
    description: false,
    languages: false,
    platforms: {}
  });

  const availableLanguages = useMemo(() => {
    return LANGUAGE_ORDER.filter((language) =>
      !selectedLanguages.includes(language.label)
    );
  }, [selectedLanguages]);

  const clearFieldError = (field: keyof Omit<ValidationErrors, "platforms">, hasValue: boolean) => {
    if (!hasValue || !validationErrors[field]) {
      return;
    }
    setValidationErrors((current) => ({ ...current, [field]: false }));
  };

  const clearPlatformError = (rowId: string, key: PlatformField, hasValue: boolean) => {
    if (!hasValue) {
      return;
    }
    setValidationErrors((current) => {
      const rowErrors = current.platforms[rowId];
      if (!rowErrors || !rowErrors[key]) {
        return current;
      }
      const nextRow = { ...rowErrors, [key]: false };
      const hasAnyError = nextRow.name || nextRow.channelUrl;
      const nextPlatforms = { ...current.platforms };
      if (hasAnyError) {
        nextPlatforms[rowId] = nextRow;
      } else {
        delete nextPlatforms[rowId];
      }
      return {
        ...current,
        platforms: nextPlatforms
      };
    });
  };

  const canSubmit = useMemo(() => {
    if (!name.trim() || !description.trim()) {
      return false;
    }
    if (!selectedLanguages.length) {
      return false;
    }
    if (
      !platforms.some((platform) => platform.name.trim() && platform.channelUrl.trim())
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
    setValidationErrors({
      name: false,
      description: false,
      languages: false,
      platforms: {}
    });
  };

  const handlePlatformChange = (rowId: string, key: PlatformField, value: string): void => {
    setPlatforms((current) =>
      current.map((row) => {
        if (row.rowId !== rowId) {
          return row;
        }
        const nextRow: PlatformFormRow = { ...row, [key]: value };
        if (key === "name") {
          const presetMatch = PLATFORM_PRESETS.some((platform) => platform.value === value);
          nextRow.preset = presetMatch ? value : "";
          nextRow.name = value;
          nextRow.id = undefined;
        }
        return nextRow;
      })
    );
    clearPlatformError(rowId, key, value.trim().length > 0);
  };

  const handlePlatformNameSelect = (rowId: string, value: string) => {
    setPlatforms((current) =>
      current.map((row) => {
        if (row.rowId !== rowId) {
          return row;
        }
        return {
          ...row,
          preset: value,
          name: value,
          id: undefined
        };
      })
    );
    clearPlatformError(rowId, "name", value.trim().length > 0);
  };

  const handleRemovePlatform = (rowId: string) => {
    setPlatforms((current) => {
      if (current.length === 1) {
        return [createPlatformRow()];
      }
      return current.filter((row) => row.rowId !== rowId);
    });
    setValidationErrors((current) => {
      if (!current.platforms[rowId]) {
        return current;
      }
      const nextPlatforms = { ...current.platforms };
      delete nextPlatforms[rowId];
      return {
        ...current,
        platforms: nextPlatforms
      };
    });
  };

  const handleLanguageSelect = (event: FormEvent<HTMLSelectElement>) => {
    const value = event.currentTarget.value;
    if (!value) {
      return;
    }
    const displayLabel = LANGUAGE_ORDER.find((language) => language.value === value)?.label ?? value;
    setSelectedLanguages((current) => [...current, displayLabel]);
    setLanguageSelection("");
    clearFieldError("languages", true);
  };

  const handleLanguageRemove = (language: string) => {
    setSelectedLanguages((current) => {
      const next = current.filter((item) => item !== language);
      if (next.length > 0) {
        clearFieldError("languages", true);
      }
      return next;
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setResultState("idle");
    setResultMessage("");

    const trimmedName = name.trim();
    const trimmedDescription = description.trim();

    const nextErrors: ValidationErrors = {
      name: trimmedName === "",
      description: trimmedDescription === "",
      languages: selectedLanguages.length === 0,
      platforms: {}
    };

    platforms.forEach((row) => {
      const rowErrors: PlatformErrorState = {
        name: row.name.trim() === "",
        channelUrl: row.channelUrl.trim() === ""
      };
      if (rowErrors.name || rowErrors.channelUrl) {
        nextErrors.platforms[row.rowId] = rowErrors;
      }
    });

    const hasPlatformErrors = Object.keys(nextErrors.platforms).length > 0;
    if (nextErrors.name || nextErrors.description || nextErrors.languages || hasPlatformErrors) {
      setValidationErrors(nextErrors);
      setIsSubmitting(false);
      setResultState("error");
      setResultMessage("Please correct the highlighted fields.");
      return;
    }

    const submission: SubmissionPayload = {
      name: trimmedName,
      description: trimmedDescription,
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
            <label className={`form-field${validationErrors.name ? " form-field-error" : ""}`}>
              <span>Streamer name *</span>
              <input
                type="text"
                name="streamer-name"
                value={name}
                onChange={(event) => {
                  const value = event.target.value;
                  setName(value);
                  clearFieldError("name", value.trim().length > 0);
                }}
                required
              />
            </label>

            <label className={`form-field form-field-wide${validationErrors.description ? " form-field-error" : ""}`}>
              <span>Description *</span>
              <p className="submit-streamer-help">What does the streamer do and what makes their streams unique?</p>
              <textarea
                name="description"
                rows={3}
                value={description}
                onChange={(event) => {
                  const value = event.target.value;
                  setDescription(value);
                  clearFieldError("description", value.trim().length > 0);
                }}
                required
              />
            </label>

            <label className={`form-field form-field-wide${validationErrors.languages ? " form-field-error" : ""}`}>
              <span>Languages *</span>
              <p className="submit-streamer-help">Select every language the streamer uses on their channel.</p>
              <div className="language-picker" data-invalid={validationErrors.languages}>
                <select
                  className="language-select"
                  value={languageSelection}
                  onChange={handleLanguageSelect}
                  required={!selectedLanguages.length}
                >
                  <option value="" disabled>
                    Languages
                  </option>
                  {availableLanguages.map((language) => (
                    <option key={language.value} value={language.value}>
                      {language.label}
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
              {validationErrors.languages ? (
                <p className="field-error-text">Select at least one language.</p>
              ) : null}
            </label>
          </div>

          <fieldset className="platform-fieldset">
            <legend>Streaming platforms *</legend>
            <p className="submit-streamer-help">
              Add each platform&rsquo;s name along with the regular channel URL and live stream URL.
              If they&rsquo;re the same, repeat the link in both fields.
            </p>

            <div className="platform-rows">
              {platforms.map((platform) => {
                const platformErrors = validationErrors.platforms[platform.rowId] ?? {
                  name: false,
                  channelUrl: false
                };
                const rowHasError = platformErrors.name || platformErrors.channelUrl;
                return (
                  <div className="platform-row" key={platform.rowId} data-platform-row>
                    <label
                      className={`form-field form-field-inline${
                        platformErrors.name ? " form-field-error" : ""
                      }`}
                    >
                      <span>Platform name</span>
                      <div className="platform-picker">
                        <select
                          className="platform-select"
                          name="platform-name"
                          value={
                            platform.preset ||
                            (PLATFORM_PRESETS.some((option) => option.value === platform.name)
                              ? platform.name
                              : "")
                          }
                          onChange={(event) =>
                            handlePlatformNameSelect(platform.rowId, event.currentTarget.value)
                          }
                          required
                        >
                          <option value="" disabled>
                            Choose platform
                          </option>
                          {PLATFORM_PRESETS.map((platformOption) => (
                            <option key={platformOption.value} value={platformOption.value}>
                              {platformOption.label}
                            </option>
                          ))}
                        </select>
                      </div>
                    </label>
                    <label
                      className={`form-field form-field-inline${
                        platformErrors.channelUrl ? " form-field-error" : ""
                      }`}
                    >
                      <span>Channel URL</span>
                      <input
                        type="url"
                        name="platform-channel"
                        placeholder="https://"
                        value={platform.channelUrl}
                        onChange={(event) =>
                          handlePlatformChange(platform.rowId, "channelUrl", event.target.value)
                        }
                        required
                      />
                    </label>

                    <button
                      type="button"
                      className="remove-platform-button"
                      onClick={() => handleRemovePlatform(platform.rowId)}
                    >
                      Remove
                    </button>
                    {rowHasError ? (
                      <p className="field-error-text">Provide the platform name and channel URL.</p>
                    ) : null}
                  </div>
                );
              })}
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
