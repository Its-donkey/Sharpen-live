import { useCallback, useEffect, useMemo, useState } from "react";
import { getStreamers } from "./api";
import { AdminPanel } from "./components/AdminPanel";
import { StreamerTable } from "./components/StreamerTable";
import { SubmitStreamerForm } from "./components/SubmitStreamerForm";
import { useAdminToken } from "./hooks/useAdminToken";
import type { Streamer } from "./types";
import "./styles.css";

export function App() {
  const [streamers, setStreamers] = useState<Streamer[]>([]);
  const [streamersLoading, setStreamersLoading] = useState(true);
  const [streamersError, setStreamersError] = useState<string | null>(null);
  const [isSubmitOpen, setIsSubmitOpen] = useState(false);
  const [isAdminOpen, setIsAdminOpen] = useState(false);
  const [adminToken, setAdminToken, clearAdminToken] = useAdminToken();

  const loadStreamers = useCallback(async () => {
    setStreamersLoading(true);
    setStreamersError(null);
    try {
      const data = await getStreamers();
      setStreamers(data);
    } catch (error) {
      setStreamersError(
        error instanceof Error ? error.message : "Unable to load streamers."
      );
    } finally {
      setStreamersLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadStreamers();
  }, [loadStreamers]);

  const currentYear = useMemo(() => new Date().getFullYear(), []);

  return (
    <div className="app-shell">
      <header className="surface site-header">
        <div className="logo-lockup">
          <div className="logo-icon" aria-hidden="true">
            <svg viewBox="0 0 120 120" role="img" aria-labelledby="sharpen-logo-title">
              <title id="sharpen-logo-title">Sharpen Live logo</title>
              <defs>
                <linearGradient id="bladeGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                  <stop offset="0%" stopColor="#f8fafc" stopOpacity="0.95" />
                  <stop offset="55%" stopColor="#cbd5f5" stopOpacity="0.85" />
                  <stop offset="100%" stopColor="#7dd3fc" stopOpacity="0.95" />
                </linearGradient>
              </defs>
              <path
                d="M14 68c12-20 38-54 80-58l6 36c-12 6-26 14-41 26l-45-4z"
                fill="url(#bladeGradient)"
                stroke="#0f172a"
                strokeWidth="4"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <path
                d="M19 76l35 4c-5 5-10 11-15 18l-26-8 6-14z"
                fill="rgba(15, 23, 42, 0.45)"
                stroke="#0f172a"
                strokeWidth="3.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <circle cx="32" cy="92" r="6" fill="#38bdf8" />
              <circle cx="88" cy="36" r="6" fill="#38bdf8" />
            </svg>
          </div>
          <div className="logo-text">
            <h1>Sharpen.Live</h1>
            <p>Streaming Knife Craftsmen</p>
          </div>
        </div>
        <div className="header-actions">
          <a className="cta" href="#streamers">
            Become a Partner
          </a>
          <button
            type="button"
            className="admin-button"
            onClick={() => setIsAdminOpen((value) => !value)}
            aria-expanded={isAdminOpen}
          >
            Admin
          </button>
        </div>
      </header>

      <main className="surface" id="streamers" aria-labelledby="streamers-title">
        <section className="intro">
          <h2
            id="streamers-title"
            style={{ margin: "0 0 1rem", fontSize: "clamp(1.8rem, 2vw + 1rem, 2.4rem)" }}
          >
            Live Knife Sharpening Studio
          </h2>
          <p style={{ margin: 0, color: "var(--fg-muted)", maxWidth: "720px", lineHeight: 1.7 }}>
            Discover bladesmiths and sharpening artists streaming in real time. Status indicators
            show who is live, who is prepping off camera, and who is offline. Premium partners share
            their booking links so you can send in your knives for a professional edge.
          </p>
        </section>

        <StreamerTable
          streamers={streamers}
          loading={streamersLoading}
          error={streamersError}
          onRetry={loadStreamers}
        />

        <SubmitStreamerForm
          isOpen={isSubmitOpen}
          onToggle={() => setIsSubmitOpen((value) => !value)}
        />

        <AdminPanel
          isOpen={isAdminOpen}
          onClose={() => setIsAdminOpen(false)}
          token={adminToken}
          setToken={setAdminToken}
          clearToken={clearAdminToken}
          onStreamersUpdated={loadStreamers}
        />
      </main>

      <footer>
        <span>
          &copy; {currentYear} Sharpen Live. All rights reserved.
        </span>
        <span>
          Interested in the roster? <a href="mailto:partners@sharpen.live">partners@sharpen.live</a>
        </span>
      </footer>
    </div>
  );
}

export default App;
