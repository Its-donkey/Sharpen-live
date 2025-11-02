import type { Platform, StreamerStatus } from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";

export interface PlatformFormRow extends Platform {
  id: string;
}

export const MAX_LANGUAGES = 8;
export const MAX_PLATFORMS = 8;

export function randomId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2, 10);
}

export function createPlatformRow(initial?: Partial<Platform>): PlatformFormRow {
  return {
    id: randomId(),
    name: initial?.name ?? "",
    channelUrl: initial?.channelUrl ?? "",
    liveUrl: initial?.liveUrl ?? ""
  };
}

export function normalizeLanguagesInput(value: string): string[] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean)
    .slice(0, MAX_LANGUAGES);
}

export function sanitizePlatforms(rows: PlatformFormRow[]): Platform[] {
  return rows
    .map((row) => ({
      name: row.name.trim(),
      channelUrl: row.channelUrl.trim(),
      liveUrl: row.liveUrl.trim()
    }))
    .filter((row) => row.name && row.channelUrl && row.liveUrl)
    .slice(0, MAX_PLATFORMS);
}

export function defaultStatusLabel(status: StreamerStatus): string {
  return STATUS_DEFAULT_LABELS[status];
}
