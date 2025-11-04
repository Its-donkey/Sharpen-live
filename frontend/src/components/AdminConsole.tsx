import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useState
} from "react";
import {
  createAdminStreamer,
  deleteAdminStreamer,
  getAdminStreamers,
  getAdminSettings,
  getAdminYouTubeMonitor,
  getSubmissions,
  loginAdmin,
  moderateSubmission,
  updateAdminSettings,
  updateAdminStreamer
} from "../api";
import type {
  AdminSettings,
  AdminSettingsUpdate,
  Streamer,
  StreamerStatus,
  Submission,
  SubmissionPayload,
  YouTubeMonitorEvent
} from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";
import {
  PLATFORM_PRESETS,
  createPlatformRow,
  defaultStatusLabel,
  normalizeLanguagesInput,
  PlatformFormRow,
  sanitizePlatforms
} from "../utils/formHelpers";

interface AdminConsoleProps {
  token: string;
  setToken: (value: string) => void;
  clearToken: () => void;
  onStreamersUpdated: () => Promise<void> | void;
}

interface StatusState {
  message: string;
  tone: "idle" | "info" | "success" | "error";
}

interface StreamerFormState {
  name: string;
  description: string;
  status: StreamerStatus;
  statusLabel: string;
  languagesInput: string;
  platforms: PlatformFormRow[];
  statusLabelEdited: boolean;
}

type PlatformField = "name" | "channelUrl";

const defaultStatus: StatusState = { message: "", tone: "idle" };
const DEV_EMAIL = "admin@sharpen.live";
const DEV_PASSWORD = "changeme123";

function toFormState(streamer?: Streamer): StreamerFormState {
  const status = streamer?.status ?? "online";
  const defaultLabel = defaultStatusLabel(status);
  const currentLabel = streamer?.statusLabel ?? defaultLabel;
  return {
    name: streamer?.name ?? "",
    description: streamer?.description ?? "",
    status,
    statusLabel: currentLabel,
    languagesInput: streamer?.languages?.join(", ") ?? "",
    platforms:
      streamer?.platforms?.length
        ? streamer.platforms.map((platform) => createPlatformRow(platform))
        : [createPlatformRow()],
    statusLabelEdited: currentLabel.trim() !== defaultLabel.trim()
  };
}

function toPayload(state: StreamerFormState): SubmissionPayload {
  return {
    name: state.name.trim(),
    description: state.description.trim(),
    status: state.status,
    statusLabel: state.statusLabel.trim() || defaultStatusLabel(state.status),
    languages: normalizeLanguagesInput(state.languagesInput),
    platforms: sanitizePlatforms(state.platforms)
  };
}

function formIsValid(state: StreamerFormState): boolean {
  return (
    Boolean(state.name.trim()) &&
    Boolean(state.description.trim()) &&
    normalizeLanguagesInput(state.languagesInput).length > 0 &&
    sanitizePlatforms(state.platforms).length > 0
  );
}

export function AdminConsole({
  token,
  setToken,
  clearToken,
  onStreamersUpdated
}: AdminConsoleProps) {
  const isDev = import.meta.env.DEV;
  const defaultEmail = isDev ? DEV_EMAIL : "";
  const defaultPassword = isDev ? DEV_PASSWORD : "";
  const [email, setEmail] = useState(defaultEmail);
  const [password, setPassword] = useState(defaultPassword);
  const [status, setStatus] = useState<StatusState>(defaultStatus);
  const [loading, setLoading] = useState(false);
  const [submissions, setSubmissions] = useState<Submission[]>([]);
  const [streamers, setStreamers] = useState<Streamer[]>([]);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"streamers" | "settings" | "monitor">(
    "streamers"
  );
  const [settings, setSettings] = useState<AdminSettings | null>(null);
  const [settingsDraft, setSettingsDraft] = useState<AdminSettings | null>(null);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [settingsSaving, setSettingsSaving] = useState(false);
  const [monitorEvents, setMonitorEvents] = useState<YouTubeMonitorEvent[]>([]);
  const [monitorLoading, setMonitorLoading] = useState(false);
  const isAuthenticated = Boolean(token);

  const loadAdminData = useCallback(
    async (currentToken: string) => {
      setLoading(true);
      setStatus({ message: "Loading admin data…", tone: "info" });
      try {
        const [subData, streamerData] = await Promise.all([
          getSubmissions(currentToken),
          getAdminStreamers(currentToken)
        ]);
        setSubmissions(subData);
        setStreamers(streamerData);
        setStatus({ message: "Admin data updated.", tone: "success" });
      } catch (error) {
        setStatus({
          message: error instanceof Error ? error.message : "Unable to load admin data.",
          tone: "error"
        });
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const loadSettings = useCallback(
    async (currentToken: string) => {
      setSettingsLoading(true);
      try {
        const result = await getAdminSettings(currentToken);
        setSettings(result);
        setSettingsDraft(result);
        if (result.adminToken && result.adminToken !== currentToken) {
          setToken(result.adminToken);
        }
      } catch (error) {
        setStatus({
          message: error instanceof Error ? error.message : "Unable to load settings.",
          tone: "error"
        });
      } finally {
        setSettingsLoading(false);
      }
    },
    [setToken]
  );

  const loadMonitor = useCallback(
    async (currentToken: string) => {
      setMonitorLoading(true);
      try {
        const events = await getAdminYouTubeMonitor(currentToken);
        setMonitorEvents(events);
      } catch (error) {
        setStatus({
          message: error instanceof Error ? error.message : "Unable to load YouTube monitor.",
          tone: "error"
        });
      } finally {
        setMonitorLoading(false);
      }
    },
    []
  );

  useEffect(() => {
    if (!token) {
      setSubmissions([]);
      setStreamers([]);
      setSettings(null);
      setMonitorEvents([]);
      setMonitorLoading(false);
      return;
    }
    void loadAdminData(token);
  }, [token, loadAdminData]);

  useEffect(() => {
    if (!isAuthenticated || activeTab !== "settings") {
      return;
    }
    if (!settings && !settingsLoading) {
      void loadSettings(token);
    }
  }, [isAuthenticated, activeTab, settings, settingsLoading, loadSettings, token]);

  useEffect(() => {
    if (!isAuthenticated || activeTab !== "monitor" || !token) {
      return;
    }
    void loadMonitor(token);
  }, [isAuthenticated, activeTab, token, loadMonitor]);

  const sortedSubmissions = useMemo(
    () =>
      submissions
        .slice()
        .sort(
          (a, b) =>
            new Date(b.submittedAt).getTime() - new Date(a.submittedAt).getTime()
        ),
    [submissions]
  );

  const handleLoginSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedEmail = email.trim();
    const trimmedPassword = password;

    if (!trimmedEmail || !trimmedPassword) {
      setStatus({ message: "Email and password are required.", tone: "error" });
      return;
    }

    try {
      setLoading(true);
      setStatus({ message: "Logging in…", tone: "info" });
      const response = await loginAdmin(trimmedEmail, trimmedPassword);
      setToken(response.token);
      setStatus({ message: "Login successful.", tone: "success" });
      await loadAdminData(response.token);
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to log in.",
        tone: "error"
      });
    } finally {
      setLoading(false);
    }
  };

  const handleLogout = () => {
    clearToken();
    setSubmissions([]);
    setStreamers([]);
    setIsCreateOpen(false);
    setEmail(defaultEmail);
    setPassword(defaultPassword);
    setSettings(null);
    setSettingsDraft(null);
     setMonitorEvents([]);
     setMonitorLoading(false);
    setActiveTab("streamers");
    setStatus({ message: "Logged out of admin console.", tone: "info" });
  };

  const refreshAll = async () => {
    if (!token) {
      setStatus({ message: "Log in to load submissions.", tone: "error" });
      return;
    }
    await loadAdminData(token);
    await onStreamersUpdated();
  };

  const refreshMonitor = async () => {
    if (!token) {
      setStatus({ message: "Log in to view monitor data.", tone: "error" });
      return;
    }
    await loadMonitor(token);
  };

  const moderate = async (action: "approve" | "reject", id: string) => {
    if (!token) {
      setStatus({ message: "Log in to manage submissions.", tone: "error" });
      return;
    }
    try {
      setStatus({ message: `${action === "approve" ? "Approving" : "Rejecting"} submission…`, tone: "info" });
      await moderateSubmission(token, action, id);
      await refreshAll();
      setStatus({
        message: `Submission ${action === "approve" ? "approved" : "rejected"}.`,
        tone: "success"
      });
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to update submission.",
        tone: "error"
      });
    }
  };

  const updateStreamer = async (id: string, payload: SubmissionPayload) => {
    if (!token) {
      setStatus({ message: "Log in to manage the roster.", tone: "error" });
      return;
    }
    try {
      await updateAdminStreamer(token, id, payload);
      await refreshAll();
      setStatus({ message: "Streamer updated.", tone: "success" });
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to update streamer.",
        tone: "error"
      });
      throw error;
    }
  };

  const removeStreamer = async (id: string) => {
    if (!token) {
      setStatus({ message: "Log in to manage the roster.", tone: "error" });
      return;
    }
    try {
      await deleteAdminStreamer(token, id);
      await refreshAll();
      setStatus({ message: "Streamer removed from roster.", tone: "success" });
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to delete streamer.",
        tone: "error"
      });
      throw error;
    }
  };

  const createStreamer = async (payload: SubmissionPayload) => {
    if (!token) {
      setStatus({ message: "Log in to manage the roster.", tone: "error" });
      return;
    }
    try {
      await createAdminStreamer(token, payload);
      await refreshAll();
      setStatus({ message: "Streamer created.", tone: "success" });
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to create streamer.",
        tone: "error"
      });
      throw error;
    }
  };

  const handleSettingsFieldChange = (field: keyof AdminSettings, value: string) => {
    if (!settingsDraft) {
      return;
    }
    const nextSettings = { ...settingsDraft, [field]: value };

    if (field === "youtubeAlertsCallback" && value.trim() === "") {
      nextSettings.youtubeAlertsSecret = "";
      nextSettings.youtubeAlertsVerifyPrefix = "";
      nextSettings.youtubeAlertsVerifySuffix = "";
      nextSettings.youtubeAlertsHubUrl = "";
    }

    setSettingsDraft(nextSettings);
  };

  const handleSettingsSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!token || !settings || !settingsDraft) {
      return;
    }

    const updates: AdminSettingsUpdate = {};
    (Object.keys(settingsDraft) as Array<keyof AdminSettings>).forEach((key) => {
      if (settingsDraft[key] !== settings[key]) {
        updates[key] = settingsDraft[key];
      }
    });

    if (Object.keys(updates).length === 0) {
      setStatus({ message: "No changes to update.", tone: "info" });
      return;
    }

    try {
      setSettingsSaving(true);
      const response = await updateAdminSettings(token, updates);
      setStatus({ message: response.message || "Settings updated.", tone: "success" });
      await loadSettings(token);
    } catch (error) {
      setStatus({
        message: error instanceof Error ? error.message : "Unable to update settings.",
        tone: "error"
      });
    } finally {
      setSettingsSaving(false);
    }
  };

  return (
    <section className="admin-panel" aria-labelledby="admin-title">
      <div className="admin-header">
        <div>
          <h2 id="admin-title">Admin Dashboard</h2>
          <p className="admin-help">
            Review incoming submissions, approve qualified streamers, or update the roster.
          </p>
        </div>
      </div>

      {!isAuthenticated ? (
        <form className="admin-auth" onSubmit={handleLoginSubmit}>
          <label className="form-field form-field-wide">
            <span>Email</span>
            <input
              type="email"
              name="admin-email"
              placeholder="you@example.com"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </label>
          <label className="form-field form-field-wide">
            <span>Password</span>
            <div className="admin-auth-controls">
              <input
                type="password"
                name="admin-password"
                placeholder="Enter your password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
              <button type="submit" className="admin-auth-submit" disabled={loading}>
                {loading ? "Logging in…" : "Log in"}
              </button>
            </div>
          </label>
        </form>
      ) : (
        <div className="admin-actions">
          <button type="button" className="secondary-button" onClick={refreshAll} disabled={loading}>
            Refresh data
          </button>
          <button type="button" className="secondary-button" onClick={handleLogout} disabled={loading}>
            Log out
          </button>
        </div>
      )}

      <div className="admin-status" role="status" aria-live="polite" data-state={status.tone}>
        {status.message}
      </div>

      {isAuthenticated ? (
        <div className="admin-tabs" role="tablist">
          <button
            type="button"
            className={activeTab === "streamers" ? "admin-tab active" : "admin-tab"}
            onClick={() => setActiveTab("streamers")}
          >
            Streamers
          </button>
          <button
            type="button"
            className={activeTab === "monitor" ? "admin-tab active" : "admin-tab"}
            onClick={() => setActiveTab("monitor")}
          >
            Monitor
          </button>
          <button
            type="button"
            className={activeTab === "settings" ? "admin-tab active" : "admin-tab"}
            onClick={() => setActiveTab("settings")}
          >
            Settings
          </button>
        </div>
      ) : null}

      {isAuthenticated ? (
        activeTab === "streamers" ? (
          <div className="admin-grid" role="tabpanel">
          <section aria-labelledby="admin-submissions-title">
            <h3 id="admin-submissions-title">Pending submissions</h3>
            {loading && !submissions.length ? (
              <div className="admin-empty">Loading submissions…</div>
            ) : sortedSubmissions.length ? (
              <div className="admin-submissions">
                {sortedSubmissions.map((submission) => (
                  <SubmissionCard
                    key={submission.id}
                    submission={submission}
                    onApprove={() => moderate("approve", submission.id)}
                    onReject={() => moderate("reject", submission.id)}
                    disabled={loading}
                  />
                ))}
              </div>
            ) : (
              <div className="admin-empty">No pending submissions at the moment.</div>
            )}
          </section>

          <section aria-labelledby="admin-streamers-title">
            <div className="admin-streamers-header">
              <h3 id="admin-streamers-title">Current roster</h3>
              <button
                type="button"
                className="secondary-button"
                onClick={() => setIsCreateOpen((value) => !value)}
                disabled={loading}
              >
                {isCreateOpen ? "Cancel new streamer" : "Add streamer"}
              </button>
            </div>

            {isCreateOpen ? (
              <AdminCreateStreamer
                onSubmit={async (payload) => {
                  await createStreamer(payload);
                  setIsCreateOpen(false);
                }}
              />
            ) : null}

            {streamers.length ? (
              <div className="admin-streamers">
                {streamers.map((streamer) => (
                  <AdminStreamerCard
                    key={streamer.id}
                    streamer={streamer}
                    onUpdate={updateStreamer}
                    onDelete={removeStreamer}
                  />
                ))}
              </div>
            ) : (
              <div className="admin-empty">No streamers found. Add one to get started.</div>
            )}
          </section>
        </div>
        ) : activeTab === "monitor" ? (
          <section className="admin-monitor" role="tabpanel">
            <div className="admin-streamers-header">
              <h3>YouTube alerts monitor</h3>
              <button
                type="button"
                className="secondary-button"
                onClick={() => void refreshMonitor()}
                disabled={monitorLoading}
              >
                {monitorLoading ? "Refreshing…" : "Refresh monitor"}
              </button>
            </div>
            <p className="admin-help">
              Review recent PubSub subscription activity so you can confirm callbacks and
              troubleshoot alerts.
            </p>
            {monitorLoading && !monitorEvents.length ? (
              <div className="admin-empty">Loading YouTube subscription activity…</div>
            ) : monitorEvents.length ? (
              <div className="admin-monitor-events">
                {monitorEvents.map((event) => (
                  <AdminMonitorEventCard
                    key={`${event.timestamp}-${event.mode}-${event.channelId}`}
                    event={event}
                  />
                ))}
              </div>
            ) : (
              <div className="admin-empty">No YouTube PubSub events recorded yet.</div>
            )}
          </section>
        ) : (
          <section className="admin-settings" role="tabpanel">
            <h3>Environment settings</h3>
            <p className="admin-help">
              Update runtime environment values. Some changes may require restarting the server to
              take effect.
            </p>
            {settingsLoading && !settingsDraft ? (
              <div className="admin-empty">Loading settings…</div>
            ) : settingsDraft ? (
              <form className="admin-settings-form" onSubmit={handleSettingsSubmit}>
                <div className="form-grid">
                  <label className="form-field">
                    <span>Admin email</span>
                    <input
                      type="email"
                      value={settingsDraft.adminEmail}
                      onChange={(event) =>
                        handleSettingsFieldChange("adminEmail", event.target.value)
                      }
                      required
                    />
                  </label>
                  <label className="form-field">
                    <span>Admin password</span>
                    <input
                      type="password"
                      value={settingsDraft.adminPassword}
                      onChange={(event) =>
                        handleSettingsFieldChange("adminPassword", event.target.value)
                      }
                      required
                    />
                  </label>
                  <label className="form-field">
                    <span>Admin token</span>
                    <input
                      type="text"
                      value={settingsDraft.adminToken}
                      onChange={(event) =>
                        handleSettingsFieldChange("adminToken", event.target.value)
                      }
                      required
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube API key</span>
                    <input
                      type="password"
                      value={settingsDraft.youtubeApiKey}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeApiKey", event.target.value)
                      }
                      placeholder="Only used for YouTube lookups"
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube alerts callback URL</span>
                    <input
                      type="url"
                      value={settingsDraft.youtubeAlertsCallback}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeAlertsCallback", event.target.value)
                      }
                      placeholder="https://example.com/alerts"
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube alerts secret</span>
                    <input
                      type="password"
                      value={settingsDraft.youtubeAlertsSecret}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeAlertsSecret", event.target.value)
                      }
                      placeholder="Optional signing secret"
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube alerts verify prefix</span>
                    <input
                      type="text"
                      value={settingsDraft.youtubeAlertsVerifyPrefix}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeAlertsVerifyPrefix", event.target.value)
                      }
                      placeholder="Prefix for hub.verify_token"
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube alerts verify suffix</span>
                    <input
                      type="text"
                      value={settingsDraft.youtubeAlertsVerifySuffix}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeAlertsVerifySuffix", event.target.value)
                      }
                      placeholder="Suffix for hub.verify_token"
                    />
                  </label>
                  <label className="form-field">
                    <span>YouTube alerts hub URL</span>
                    <input
                      type="url"
                      value={settingsDraft.youtubeAlertsHubUrl}
                      onChange={(event) =>
                        handleSettingsFieldChange("youtubeAlertsHubUrl", event.target.value)
                      }
                      placeholder="Defaults to Google's PubSubHubbub hub"
                    />
                  </label>
                  <label className="form-field">
                    <span>Listen address</span>
                    <input
                      type="text"
                      value={settingsDraft.listenAddr}
                      onChange={(event) =>
                        handleSettingsFieldChange("listenAddr", event.target.value)
                      }
                    />
                  </label>
                  <label className="form-field">
                    <span>Data directory</span>
                    <input
                      type="text"
                      value={settingsDraft.dataDir}
                      onChange={(event) =>
                        handleSettingsFieldChange("dataDir", event.target.value)
                      }
                    />
                  </label>
                  <label className="form-field">
                    <span>Static directory</span>
                    <input
                      type="text"
                      value={settingsDraft.staticDir}
                      onChange={(event) =>
                        handleSettingsFieldChange("staticDir", event.target.value)
                      }
                    />
                  </label>
                  <label className="form-field">
                    <span>Streamers file</span>
                    <input
                      type="text"
                      value={settingsDraft.streamersFile}
                      onChange={(event) =>
                        handleSettingsFieldChange("streamersFile", event.target.value)
                      }
                    />
                  </label>
                  <label className="form-field">
                    <span>Submissions file</span>
                    <input
                      type="text"
                      value={settingsDraft.submissionsFile}
                      onChange={(event) =>
                        handleSettingsFieldChange("submissionsFile", event.target.value)
                      }
                    />
                  </label>
                </div>
                <div className="submit-streamer-actions">
                  <button type="submit" className="submit-streamer-submit" disabled={settingsSaving}>
                    {settingsSaving ? "Saving…" : "Save settings"}
                  </button>
                </div>
              </form>
            ) : (
              <div className="admin-empty">Settings unavailable.</div>
            )}
          </section>
        )
      ) : (
        <div className="admin-empty">
          Log in with your admin credentials to review submissions.
        </div>
      )}
    </section>
  );
}

interface SubmissionCardProps {
  submission: Submission;
  onApprove: () => void;
  onReject: () => void;
  disabled?: boolean;
}

function SubmissionCard({ submission, onApprove, onReject, disabled }: SubmissionCardProps) {
  return (
    <article className="admin-card" data-submission-id={submission.id}>
      <div className="admin-card-header">
        <h4>{submission.payload.name}</h4>
        <span className="admin-card-meta">
          Submitted {new Date(submission.submittedAt).toLocaleString()}
        </span>
      </div>
      <section>
        <strong>Description</strong>
        <p>{submission.payload.description}</p>
      </section>
      <section>
        <strong>Languages</strong>
        <p>{submission.payload.languages.join(" · ")}</p>
      </section>
      <section>
        <strong>Platforms</strong>
        <ul className="platform-list">
          {submission.payload.platforms.map((platform, index) => (
            <li key={`${platform.name}-${index}`}>
              {platform.name} · {platform.channelUrl}
            </li>
          ))}
        </ul>
      </section>
      <div className="admin-card-actions">
        <button type="button" data-variant="approve" onClick={onApprove} disabled={disabled}>
          Approve
        </button>
        <button type="button" data-variant="reject" onClick={onReject} disabled={disabled}>
          Reject
        </button>
      </div>
    </article>
  );
}

interface AdminStreamerCardProps {
  streamer: Streamer;
  onUpdate: (id: string, payload: SubmissionPayload) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}

function AdminStreamerCard({ streamer, onUpdate, onDelete }: AdminStreamerCardProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [state, setState] = useState<StreamerFormState>(() => toFormState(streamer));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setState(toFormState(streamer));
  }, [streamer]);

  const canSave = formIsValid(state) && !isSaving;

  const handleStatusChange = (value: StreamerStatus) => {
    setState((current) => {
      const nextStatus = value;
      const defaultLabel = defaultStatusLabel(nextStatus);
      const shouldUpdateLabel = !current.statusLabelEdited;
      return {
        ...current,
        status: nextStatus,
        statusLabel: shouldUpdateLabel ? defaultLabel : current.statusLabel
      };
    });
  };

  const handlePlatformChange = (rowId: string, key: PlatformField, value: string) => {
    setState((current) => ({
      ...current,
      platforms: current.platforms.map((platform) => {
        if (platform.rowId !== rowId) {
          return platform;
        }
        const nextPlatform: PlatformFormRow = { ...platform, [key]: value };
        if (key === "name") {
          const presetMatch = PLATFORM_PRESETS.some((option) => option.value === value);
          nextPlatform.preset = presetMatch ? value : "";
          nextPlatform.name = value;
          nextPlatform.id = undefined;
        }
        return nextPlatform;
      })
    }));
  };

  const handlePlatformPresetSelect = (rowId: string, value: string) => {
    setState((current) => ({
      ...current,
      platforms: current.platforms.map((platform) => {
        if (platform.rowId !== rowId) {
          return platform;
        }
        return {
          ...platform,
          preset: value,
          name: value,
          id: undefined
        };
      })
    }));
  };

  const handleRemovePlatform = (rowId: string) => {
    setState((current) => {
      if (current.platforms.length === 1) {
        return { ...current, platforms: [createPlatformRow()] };
      }
      return {
        ...current,
        platforms: current.platforms.filter((platform) => platform.rowId !== rowId)
      };
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!formIsValid(state)) {
      setError("Provide a name, description, at least one language, and valid platforms.");
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      await onUpdate(streamer.id, toPayload(state));
      setIsEditing(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to update streamer.");
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    const confirmed = window.confirm(
      `Remove ${streamer.name} from the roster? This action cannot be undone.`
    );
    if (!confirmed) {
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      await onDelete(streamer.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete streamer.");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <article className="admin-card" data-streamer-id={streamer.id}>
      <div className="admin-card-header">
        <div>
          <h4>{streamer.name}</h4>
          <span className="admin-card-meta">
            Status: {STATUS_DEFAULT_LABELS[streamer.status]} · Languages: {streamer.languages.join(" · ")}
          </span>
        </div>
        <div className="admin-card-actions">
          <button type="button" onClick={() => setIsEditing((value) => !value)}>
            {isEditing ? "Cancel edit" : "Edit"}
          </button>
          <button type="button" className="secondary-button" onClick={handleDelete} disabled={isSaving}>
            Delete
          </button>
        </div>
      </div>

      {isEditing ? (
        <form className="admin-streamer-form" onSubmit={handleSubmit}>
          <div className="form-grid">
            <label className="form-field">
              <span>Name</span>
              <input
                type="text"
                value={state.name}
                onChange={(event) =>
                  setState((current) => ({ ...current, name: event.target.value }))
                }
                required
              />
            </label>
            <label className="form-field">
              <span>Status</span>
              <select
                value={state.status}
                onChange={(event) => handleStatusChange(event.target.value as StreamerStatus)}
              >
                <option value="online">Online</option>
                <option value="busy">Workshop</option>
                <option value="offline">Offline</option>
              </select>
            </label>
            <label className="form-field">
              <span>Status label</span>
              <input
                type="text"
                value={state.statusLabel}
                onChange={(event) =>
                  setState((current) => ({
                    ...current,
                    statusLabel: event.target.value,
                    statusLabelEdited: event.target.value.trim().length > 0
                  }))
                }
              />
            </label>
            <label className="form-field form-field-wide">
              <span>Description</span>
              <textarea
                rows={3}
                value={state.description}
                onChange={(event) =>
                  setState((current) => ({ ...current, description: event.target.value }))
                }
              />
            </label>
            <label className="form-field form-field-wide">
              <span>Languages</span>
              <input
                type="text"
                value={state.languagesInput}
                onChange={(event) =>
                  setState((current) => ({ ...current, languagesInput: event.target.value }))
                }
                placeholder="Example: English, Japanese"
              />
            </label>
          </div>

          <fieldset className="platform-fieldset">
            <legend>Platforms</legend>
            <div className="platform-rows">
              {state.platforms.map((platform) => (
                <div className="platform-row" key={platform.rowId}>
                  <label className="form-field form-field-inline">
                    <span>Platform name</span>
                    <div className="platform-picker">
                      <select
                        className="platform-select"
                        value={
                          platform.preset ||
                          (PLATFORM_PRESETS.some((option) => option.value === platform.name)
                            ? platform.name
                            : "")
                        }
                        onChange={(event) =>
                          handlePlatformPresetSelect(platform.rowId, event.currentTarget.value)
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
                  <label className="form-field form-field-inline">
                    <span>Channel URL</span>
                    <input
                      type="url"
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
                </div>
              ))}
            </div>
            <button
              type="button"
              className="add-platform-button"
              onClick={() =>
                setState((current) => ({
                  ...current,
                  platforms: [...current.platforms, createPlatformRow()]
                }))
              }
            >
              + Add another platform
            </button>
          </fieldset>

          {error ? <div className="form-error">{error}</div> : null}

          <div className="submit-streamer-actions">
            <button type="submit" className="submit-streamer-submit" disabled={!canSave}>
              {isSaving ? "Saving…" : "Save changes"}
            </button>
            <button
              type="button"
              className="submit-streamer-cancel"
              onClick={() => {
                setState(toFormState(streamer));
                setIsEditing(false);
                setError(null);
              }}
            >
              Cancel
            </button>
          </div>
        </form>
      ) : (
        <div className="admin-card-body">
          <p>{streamer.description}</p>
          <div className="admin-card-meta">
            <strong>Platforms</strong>
            <ul className="platform-list">
              {streamer.platforms.map((platform, index) => (
                <li key={`${platform.name}-${index}`}>
                  {platform.name} · {platform.channelUrl}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </article>
  );
}

interface AdminCreateStreamerProps {
  onSubmit: (payload: SubmissionPayload) => Promise<void>;
}

interface AdminMonitorEventCardProps {
  event: YouTubeMonitorEvent;
}

function AdminMonitorEventCard({ event }: AdminMonitorEventCardProps) {
  const eventDate = event.timestamp ? new Date(event.timestamp) : null;
  const formattedTimestamp =
    eventDate && !Number.isNaN(eventDate.getTime())
      ? eventDate.toLocaleString()
      : "Unknown time";
  const modeLabel =
    event.mode && event.mode.length > 0
      ? event.mode.charAt(0).toUpperCase() + event.mode.slice(1)
      : "Event";
  const statusLine = event.error ? `${event.status} — ${event.error}` : event.status;

  return (
    <article className="admin-card" data-mode={event.mode}>
      <div className="admin-card-header">
        <h4>{modeLabel}</h4>
        <span className="admin-card-meta">{formattedTimestamp}</span>
      </div>
      <section>
        <strong>Channel</strong>
        <p>{event.channelId || "—"}</p>
      </section>
      <section>
        <strong>Status</strong>
        <p>{statusLine || "—"}</p>
      </section>
      <section>
        <strong>Callback</strong>
        <p>{event.callback || "—"}</p>
      </section>
      <section>
        <strong>Topic</strong>
        <p>{event.topic || "—"}</p>
      </section>
      <section>
        <strong>Verify token</strong>
        <p>{event.verifyToken || "—"}</p>
      </section>
      <section>
        <strong>Secret included</strong>
        <p>{event.hasSecret ? "Yes" : "No"}</p>
      </section>
    </article>
  );
}

function AdminCreateStreamer({ onSubmit }: AdminCreateStreamerProps) {
  const [state, setState] = useState<StreamerFormState>(() => toFormState());
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const canSave = formIsValid(state) && !isSaving;

  const handlePlatformFieldChange = (rowId: string, key: PlatformField, value: string) => {
    setState((current) => ({
      ...current,
      platforms: current.platforms.map((row) => {
        if (row.rowId !== rowId) {
          return row;
        }
        const nextRow: PlatformFormRow = { ...row, [key]: value };
        if (key === "name") {
          const presetMatch = PLATFORM_PRESETS.some((option) => option.value === value);
          nextRow.preset = presetMatch ? value : "";
          nextRow.name = value;
          nextRow.id = undefined;
        }
        return nextRow;
      })
    }));
  };

  const handlePlatformPresetSelect = (rowId: string, value: string) => {
    setState((current) => ({
      ...current,
      platforms: current.platforms.map((row) => {
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
    }));
  };

  const handleStatusChange = (value: StreamerStatus) => {
    setState((current) => {
      const nextStatus = value;
      const defaultLabel = defaultStatusLabel(nextStatus);
      const shouldUpdateLabel = !current.statusLabelEdited;
      return {
        ...current,
        status: nextStatus,
        statusLabel: shouldUpdateLabel ? defaultLabel : current.statusLabel
      };
    });
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!formIsValid(state)) {
      setError("Provide a name, description, at least one language, and valid platforms.");
      return;
    }
    setIsSaving(true);
    setError(null);
    try {
      await onSubmit(toPayload(state));
      setState(toFormState());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create streamer.");
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <form className="admin-streamer-form" onSubmit={handleSubmit}>
      <h4>Add new streamer</h4>
      <div className="form-grid">
        <label className="form-field">
          <span>Name</span>
          <input
            type="text"
            value={state.name}
            onChange={(event) =>
              setState((current) => ({ ...current, name: event.target.value }))
            }
            required
          />
        </label>
        <label className="form-field">
          <span>Status</span>
          <select
            value={state.status}
            onChange={(event) => handleStatusChange(event.target.value as StreamerStatus)}
          >
            <option value="online">Online</option>
            <option value="busy">Workshop</option>
            <option value="offline">Offline</option>
          </select>
        </label>
        <label className="form-field">
          <span>Status label</span>
          <input
            type="text"
            value={state.statusLabel}
            onChange={(event) =>
              setState((current) => ({
                ...current,
                statusLabel: event.target.value,
                statusLabelEdited: event.target.value.trim().length > 0
              }))
            }
            placeholder="Defaults to the selected status"
          />
        </label>
        <label className="form-field form-field-wide">
          <span>Description</span>
          <textarea
            rows={3}
            value={state.description}
            onChange={(event) =>
              setState((current) => ({ ...current, description: event.target.value }))
            }
          />
        </label>
        <label className="form-field form-field-wide">
          <span>Languages</span>
          <input
            type="text"
            value={state.languagesInput}
            onChange={(event) =>
              setState((current) => ({ ...current, languagesInput: event.target.value }))
            }
            placeholder="Example: English, Japanese"
          />
        </label>
      </div>

      <fieldset className="platform-fieldset">
        <legend>Platforms</legend>
        <div className="platform-rows">
          {state.platforms.map((platform) => (
            <div className="platform-row" key={platform.rowId}>
              <label className="form-field form-field-inline">
                <span>Platform name</span>
                <div className="platform-picker">
                  <select
                    className="platform-select"
                    value={
                      platform.preset ||
                      (PLATFORM_PRESETS.some((option) => option.value === platform.name)
                        ? platform.name
                        : "")
                    }
                    onChange={(event) =>
                      handlePlatformPresetSelect(platform.rowId, event.currentTarget.value)
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
              <label className="form-field form-field-inline">
                <span>Channel URL</span>
                <input
                  type="url"
                  value={platform.channelUrl}
                  onChange={(event) =>
                    handlePlatformFieldChange(platform.rowId, "channelUrl", event.target.value)
                  }
                  required
                />
              </label>
              <button
                type="button"
                className="remove-platform-button"
                onClick={() =>
                  setState((current) => {
                    if (current.platforms.length === 1) {
                      return { ...current, platforms: [createPlatformRow()] };
                    }
                    return {
                      ...current,
                      platforms: current.platforms.filter((row) => row.rowId !== platform.rowId)
                    };
                  })
                }
              >
                Remove
              </button>
            </div>
          ))}
        </div>
        <button
          type="button"
          className="add-platform-button"
          onClick={() =>
            setState((current) => ({
              ...current,
              platforms: [...current.platforms, createPlatformRow()]
            }))
          }
        >
          + Add another platform
        </button>
      </fieldset>

      {error ? <div className="form-error">{error}</div> : null}

      <div className="submit-streamer-actions">
        <button type="submit" className="submit-streamer-submit" disabled={!canSave}>
          {isSaving ? "Creating…" : "Create streamer"}
        </button>
        <button
          type="button"
          className="submit-streamer-cancel"
          onClick={() => {
            setState(toFormState());
            setError(null);
          }}
        >
          Reset form
        </button>
      </div>
    </form>
  );
}
