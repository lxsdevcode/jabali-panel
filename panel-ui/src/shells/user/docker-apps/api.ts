// Wire calls for the M49 tenant docker-apps surface (GH #170).
// Mirrors the admin api.ts but against /docker-apps (subject-scoped) and a
// constrained verb set — no exec / edit-compose / update (ADR-0117 D8).
import { apiClient } from "../../../apiClient";
import type {
  CatalogEntry,
  InstallRequest,
  InstalledApp,
} from "../../admin/docker-apps/types";

const BASE = "/docker-apps";

export async function listCatalog(): Promise<CatalogEntry[]> {
  const { data } = await apiClient.get<{ items: CatalogEntry[] }>(`${BASE}/catalog`);
  return data.items || [];
}

export async function listInstalled(): Promise<InstalledApp[]> {
  const { data } = await apiClient.get<{ items: InstalledApp[] }>(BASE);
  return data.items || [];
}

export async function installApp(req: InstallRequest): Promise<InstalledApp> {
  // Install pulls an image + composes up + waits for health — override the
  // 15s axios default to the backend's 10-min ceiling.
  const { data } = await apiClient.post<InstalledApp>(BASE, req, { timeout: 10 * 60 * 1000 });
  return data;
}

export async function deleteApp(id: string): Promise<void> {
  await apiClient.delete(`${BASE}/${id}`);
}

export async function lifecycleAction(
  id: string,
  action: "start" | "stop" | "restart",
): Promise<void> {
  await apiClient.post(`${BASE}/${id}/${action}`);
}

export function catalogIconUrl(slug: string): string {
  return `/api/v1${BASE}/catalog/${slug}/icon`;
}

export interface LogsResponse {
  slug: string;
  logs: string;
}

export async function fetchLogs(id: string, lines = 200): Promise<LogsResponse> {
  const { data } = await apiClient.get<LogsResponse>(`${BASE}/${id}/logs?lines=${lines}`);
  return data;
}

export interface EnvVarView {
  name: string;
  value: string;
  secret?: boolean;
  generated?: boolean;
}

export async function fetchEnv(id: string): Promise<EnvVarView[]> {
  const { data } = await apiClient.get<{ env: EnvVarView[] }>(`${BASE}/${id}/env`);
  return data.env || [];
}

export interface DockerUsage {
  used_bytes: number;
  quota_bytes: number;
  over_quota: boolean;
}

export async function fetchUsage(): Promise<DockerUsage> {
  const { data } = await apiClient.get<DockerUsage>(`${BASE}/usage`);
  return data;
}
