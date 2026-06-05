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
