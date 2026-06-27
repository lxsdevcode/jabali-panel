// useServerCapabilities — shared, cached read of the opt-in feature flags the
// panel gates UI on (GH #229 + gap-audit #1). Backed by GET
// /me/server-capabilities. Used by the layouts to filter the sidebar AND by
// CapabilityRoute to gate the routes themselves, so a deep-link to a disabled
// feature redirects instead of rendering a page that immediately 403s.
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "../apiClient";

export interface ServerCapabilities {
  postgres_enabled: boolean;
  docker_marketplace_enabled: boolean;
  docker_apps_user_enabled: boolean;
  python_apps_enabled: boolean;
  tenant_domain_options_enabled: boolean;
}

export function useServerCapabilities() {
  return useQuery<ServerCapabilities>({
    queryKey: ["server-capabilities"],
    queryFn: async () => {
      const { data } = await apiClient.get<Partial<ServerCapabilities>>("/me/server-capabilities");
      return {
        postgres_enabled: !!data.postgres_enabled,
        docker_marketplace_enabled: !!data.docker_marketplace_enabled,
        docker_apps_user_enabled: !!data.docker_apps_user_enabled,
        python_apps_enabled: !!data.python_apps_enabled,
        tenant_domain_options_enabled: !!data.tenant_domain_options_enabled,
      };
    },
    staleTime: 60_000,
  });
}
