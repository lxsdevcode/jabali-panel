// Wire calls for the M48 admin docker-apps surface.
import { apiClient } from "../../../apiClient";
import type {
  CatalogEntry,
  InstallRequest,
  InstalledApp,
} from "./types";

const BASE = "/admin/docker-apps";

export async function listCatalog(): Promise<CatalogEntry[]> {
  const { data } = await apiClient.get<{ items: CatalogEntry[] }>(`${BASE}/catalog`);
  return data.items || [];
}

export async function listInstalled(): Promise<InstalledApp[]> {
  const { data } = await apiClient.get<{ items: InstalledApp[] }>(BASE);
  return data.items || [];
}

export async function installApp(req: InstallRequest): Promise<InstalledApp> {
  const { data } = await apiClient.post<InstalledApp>(BASE, req);
  return data;
}

export async function deleteApp(id: string, keepVolumes = false): Promise<void> {
  const qs = keepVolumes ? "?keep_volumes=1" : "";
  await apiClient.delete(`${BASE}/${id}${qs}`);
}

export async function lifecycleAction(
  id: string,
  action: "start" | "stop" | "restart" | "rebuild",
): Promise<void> {
  await apiClient.post(`${BASE}/${id}/${action}`);
}

export async function updateApp(id: string): Promise<{ outcome: "updated" | "rolled_back"; detail?: string }> {
  const { data } = await apiClient.post<{ outcome: "updated" | "rolled_back"; detail?: string }>(`${BASE}/${id}/update`);
  return data;
}

export interface LogsResponse {
  slug: string;
  logs: string;
}

export async function fetchLogs(id: string, lines = 200): Promise<LogsResponse> {
  const { data } = await apiClient.get<LogsResponse>(`${BASE}/${id}/logs?lines=${lines}`);
  return data;
}

export interface ExecResponse {
  slug: string;
  exit_code: number;
  stdout: string;
  stderr: string;
}

export async function execCmd(id: string, command: string, service?: string): Promise<ExecResponse> {
  const { data } = await apiClient.post<ExecResponse>(`${BASE}/${id}/exec`, {
    command,
    ...(service ? { service } : {}),
  });
  return data;
}

export interface BackupRow {
  id: string;
  time: string;
  hostname?: string;
  tags?: string[];
}
export interface ListBackupsResponse {
  slug: string;
  backups: BackupRow[];
}
export async function listBackups(id: string): Promise<ListBackupsResponse> {
  const { data } = await apiClient.get<ListBackupsResponse>(`${BASE}/${id}/backups`);
  return data;
}
export async function createBackup(id: string): Promise<{ snapshot_id: string; size_bytes?: number }> {
  const { data } = await apiClient.post<{ snapshot_id: string; size_bytes?: number }>(`${BASE}/${id}/backup`);
  return data;
}
export async function restoreBackup(id: string, snapshotId: string): Promise<void> {
  await apiClient.post(`${BASE}/${id}/backups/${snapshotId}/restore`);
}


export interface PatchRequest {
  update_mode?: "manual" | "auto";
  cpu_limit?: string;
  memory_limit?: string;
  pids_limit?: number;
  domain?: string;
  ports?: Array<{
    name: string;
    enabled?: boolean;
    bind_interface?: string;
    host_port?: number;
    reverse_proxy?: boolean;
  }>;
}

export async function patchApp(id: string, body: PatchRequest): Promise<void> {
  await apiClient.patch(`${BASE}/${id}`, body);
}
