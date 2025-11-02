export type StreamerStatus = "online" | "busy" | "offline";

export interface Platform {
  name: string;
  channelUrl: string;
  liveUrl: string;
}

export interface Streamer {
  id: string;
  name: string;
  description: string;
  status: StreamerStatus;
  statusLabel: string;
  languages: string[];
  platforms: Platform[];
}

export interface SubmissionPayload {
  name: string;
  description: string;
  status: StreamerStatus;
  statusLabel: string;
  languages: string[];
  platforms: Platform[];
}

export interface Submission {
  id: string;
  submittedAt: string;
  payload: SubmissionPayload;
}

export interface SuccessPayload {
  message: string;
  id?: string;
}

export interface ErrorPayload {
  message: string;
}

export const STATUS_DEFAULT_LABELS: Record<StreamerStatus, string> = {
  online: "Online",
  busy: "Workshop",
  offline: "Offline"
};
