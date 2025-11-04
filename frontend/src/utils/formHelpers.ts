import type { Platform, StreamerStatus } from "../types";
import { STATUS_DEFAULT_LABELS } from "../types";

export interface PlatformFormRow extends Platform {
  rowId: string;
  preset: string;
}

export const MAX_LANGUAGES = 8;
export const MAX_PLATFORMS = 8;

export const PLATFORM_PRESETS = [
  { label: "YouTube", value: "YouTube" },
  { label: "Twitch", value: "Twitch" },
  { label: "Facebook Live", value: "Facebook Live" },
  { label: "Instagram Live", value: "Instagram Live" },
  { label: "Kick", value: "Kick" },
  { label: "TikTok Live", value: "TikTok Live" },
  { label: "Trovo", value: "Trovo" },
  { label: "Rumble", value: "Rumble" },
  { label: "Discord", value: "Discord" }
] as const;

export function randomId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2, 10);
}

export function createPlatformRow(initial?: Partial<Platform>): PlatformFormRow {
  const initialName = initial?.name ?? "";
  const presetMatch = PLATFORM_PRESETS.some((platform) => platform.value === initialName);
  return {
    rowId: randomId(),
    name: initialName,
    id: initial?.id,
    channelUrl: initial?.channelUrl ?? "",
    preset: presetMatch ? initialName : ""
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
      id: row.id?.trim() || undefined
    }))
    .filter((row) => row.name && row.channelUrl)
    .slice(0, MAX_PLATFORMS);
}

export function defaultStatusLabel(status: StreamerStatus): string {
  return STATUS_DEFAULT_LABELS[status];
}
