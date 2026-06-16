// Hooks for the Python Application Manager (ADR-0131 / GH #203).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { apiClient } from "../../../apiClient";

export type PythonApp = {
  id: string;
  user_id: string;
  domain_id: string;
  name: string;
  python_version: string;
  app_root: string;
  app_type: "wsgi" | "asgi";
  entrypoint: string;
  base_uri: string;
  loopback_port?: number;
  status: string;
  last_error?: string;
  created_at: string;
};

export type CreatePythonAppInput = {
  domain_id: string;
  name: string;
  python_version: string;
  app_root: string;
  app_type: "wsgi" | "asgi";
  entrypoint: string;
  base_uri: string;
  env?: Record<string, string>;
};

const KEY = ["python-apps"];

export function usePythonApps() {
  return useQuery({
    queryKey: KEY,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: PythonApp[] }>(
        "/python-apps",
      );
      return data.data ?? [];
    },
  });
}

export function useCreatePythonApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreatePythonAppInput) => {
      const { data } = await apiClient.post<{ app: PythonApp }>(
        "/python-apps",
        input,
      );
      return data.app;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}

export function useDeletePythonApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient.delete(`/python-apps/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}

export function useControlPythonApp() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (vars: { id: string; action: string }) => {
      await apiClient.post(`/python-apps/${vars.id}/control`, {
        action: vars.action,
      });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}

export async function fetchPythonAppLogs(id: string): Promise<string> {
  const { data } = await apiClient.get<{ logs: string }>(
    `/python-apps/${id}/logs`,
  );
  return data.logs ?? "";
}
