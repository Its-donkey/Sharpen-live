import { useMemo } from "react";
import type { Streamer } from "../types";

interface StreamerTableProps {
  streamers: Streamer[];
  loading: boolean;
  error: string | null;
  onRetry?: () => void;
}

const STATUS_FALLBACK = "offline";
const STATUS_LABEL_FALLBACK = "Offline";

export function StreamerTable({
  streamers,
  loading,
  error,
  onRetry
}: StreamerTableProps) {
  const bodyContent = useMemo(() => {
    if (loading) {
      return (
        <tr>
          <td colSpan={4} className="table-status">
            Loading streamer roster…
          </td>
        </tr>
      );
    }

    if (error) {
      return (
        <tr>
          <td colSpan={4} className="table-status">
            <span>{error}</span>
            {onRetry ? (
              <button type="button" className="link-button" onClick={onRetry}>
                Try again
              </button>
            ) : null}
          </td>
        </tr>
      );
    }

    if (!streamers.length) {
      return (
        <tr>
          <td colSpan={4} className="table-status">
            No streamers available at the moment.
          </td>
        </tr>
      );
    }

    return streamers.map((streamer) => {
      const status = (streamer.status ?? STATUS_FALLBACK).toLowerCase();
      const isLive = status === "online";
      const languages = Array.isArray(streamer.languages)
        ? streamer.languages.join(" · ")
        : streamer.languages;

      return (
        <tr key={streamer.id}>
          <td data-label="Status">
            <span className={`status ${status}`}>
              {streamer.statusLabel || STATUS_LABEL_FALLBACK}
            </span>
          </td>
          <td data-label="Name">
            <strong>{streamer.name}</strong>
            {streamer.description ? (
              <div className="streamer-description">{streamer.description}</div>
            ) : null}
          </td>
          <td data-label="Streaming Platforms">
            {streamer.platforms?.length ? (
              <ul className="platform-list">
                {streamer.platforms.map((platform, index) => {
                  const liveUrl = platform.liveUrl || platform.channelUrl;
                  const channelUrl = platform.channelUrl || platform.liveUrl;
                  const targetUrl = isLive ? liveUrl : channelUrl;

                  return (
                    <li key={`${platform.name}-${index}`}>
                      {targetUrl ? (
                        <a
                          className="platform-link"
                          href={targetUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {platform.name}
                        </a>
                      ) : (
                        <span className="platform-link" aria-disabled="true">
                          {platform.name}
                        </span>
                      )}
                    </li>
                  );
                })}
              </ul>
            ) : (
              "—"
            )}
          </td>
          <td data-label="Language">
            <span className="lang">{languages || "—"}</span>
          </td>
        </tr>
      );
    });
  }, [error, loading, onRetry, streamers]);

  return (
    <section className="streamer-table" aria-label="Sharpen Live streamer roster">
      <table>
        <thead>
          <tr>
            <th scope="col">Status</th>
            <th scope="col">Name</th>
            <th scope="col">Streaming Platforms</th>
            <th scope="col">Language</th>
          </tr>
        </thead>
        <tbody>{bodyContent}</tbody>
      </table>
    </section>
  );
}
