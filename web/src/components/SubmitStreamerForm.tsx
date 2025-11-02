import { FormEvent, useMemo, useState } from "react";
import { submitStreamer } from "../api";
import type { SubmissionPayload, StreamerStatus } from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";
import {
  createPlatformRow,
  defaultStatusLabel,
  normalizeLanguagesInput,
  PlatformFormRow,
  sanitizePlatforms
} from "../utils/formHelpers";

interface SubmitStreamerFormProps {
  isOpen: boolean;
  onToggle: () => void;
}

type PlatformField = "name" | "channelUrl" | "liveUrl";

export function SubmitStreamerForm({ isOpen, onToggle }: SubmitStreamerFormProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [status, setStatus] = useState<StreamerStatus>("online");
  const [statusLabel, setStatusLabel] = useState(defaultStatusLabel("online"));
  const [languagesInput, setLanguagesInput] = useState("");
  const [platforms, setPlatforms] = useState<PlatformFormRow[]>([createPlatformRow()]);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resultMessage, setResultMessage] = useState("");
  const [resultState, setResultState] = useState<"idle" | "success" | "error">("idle");
  const [statusLabelEdited, setStatusLabelEdited] = useState(false);

  const canSubmit = useMemo(() => {
    if (!name.trim() || !description.trim()) {
      return false;
    }
    if (!normalizeLanguagesInput(languagesInput).length) {
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
  }, [description, languagesInput, name, platforms]);

  const resetForm = () => {
    setName("");
    setDescription("");
    setStatus("online");
    setStatusLabel(defaultStatusLabel("online"));
    setLanguagesInput("");
    setPlatforms([createPlatformRow()]);
    setStatusLabelEdited(false);
  };

  const handlePlatformChange = (id: string, key: PlatformField, value: string): void => {
    setPlatforms((current) =>
      current.map((row) => (row.id === id ? { ...row, [key]: value } : row))
    );
  };

  const handleRemovePlatform = (id: string) => {
    setPlatforms((current) => {
      if (current.length === 1) {
        return [emptyPlatform()];
      }
      return current.filter((row) => row.id !== id);
    });
  };

  const handleStatusChange = (value: StreamerStatus) => {
    setStatus(value);
    if (!statusLabelEdited) {
      setStatusLabel(defaultStatusLabel(value));
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);
    setResultState("idle");
    setResultMessage("");

    const submission: SubmissionPayload = {
      name: name.trim(),
      description: description.trim(),
      status,
      statusLabel: statusLabel.trim() || defaultStatusLabel(status),
      languages: normalizeLanguagesInput(languagesInput),
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

            <label className="form-field">
              <span>Status *</span>
              <select
                name="status"
                value={status}
                onChange={(event) => handleStatusChange(event.target.value as StreamerStatus)}
                required
              >
                <option value="online" data-default-label="Online">
                  Online
                </option>
                <option value="busy" data-default-label="Workshop">
                  Workshop
                </option>
                <option value="offline" data-default-label="Offline">
                  Offline
                </option>
              </select>
            </label>

            <label className="form-field">
              <span>Status label</span>
              <input
                type="text"
                name="status-label"
                placeholder="Defaults to the selected status"
                value={statusLabel}
                onChange={(event) => {
                  setStatusLabel(event.target.value);
                  setStatusLabelEdited(event.target.value.trim().length > 0);
                }}
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
              <input
                type="text"
                name="languages"
                placeholder="Example: English, Japanese"
                value={languagesInput}
                onChange={(event) => setLanguagesInput(event.target.value)}
                required
              />
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
