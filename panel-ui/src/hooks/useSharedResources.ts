// useSharedResources.ts — M52 (ADR-0133) shared resources (standalone shared
// calendars / address books / file folders) + their grants. DB is truth; the
// panel reconciler converges to Stalwart shareWith.
import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";

import { apiClient } from "../apiClient";

export type SharedResourceKind = "mailbox" | "calendar" | "addressbook" | "files";
export type GrantLevel = "read" | "readwrite" | "admin";
export type GranteeKind = "mailbox" | "group";

export interface SharedResource {
  id: string;
  domain_id: string;
  kind: SharedResourceKind;
  email?: string | null;
  display_name: string;
  host_account_id: string;
  collection_id: string;
  created_at: string;
  updated_at: string;
}

export interface SharedResourceGrant {
  resource_id: string;
  grantee_kind: GranteeKind;
  grantee_id: string;
  rights: GrantLevel;
}

export interface SharedResourceDetail extends SharedResource {
  grants: SharedResourceGrant[];
}

export interface CreateSharedResourceInput {
  domainId: string;
  name: string;
  kind: SharedResourceKind;
  display_name: string;
}

const KEY = "shared_resources";

export function useCreateSharedResource(): UseMutationResult<
  SharedResource,
  unknown,
  CreateSharedResourceInput
> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ domainId, name, kind, display_name }) => {
      const { data } = await apiClient.post<SharedResource>(
        `/domains/${domainId}/shared-resources`,
        { name, kind, display_name },
      );
      return data;
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: [KEY] }),
  });
}

export function useDeleteSharedResource(): UseMutationResult<void, unknown, { id: string }> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }) => {
      await apiClient.delete(`/shared-resources/${id}`);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: [KEY] }),
  });
}

export async function fetchSharedResourceDetail(id: string): Promise<SharedResourceDetail> {
  const { data } = await apiClient.get<SharedResourceDetail>(`/shared-resources/${id}`);
  return data;
}

export function useSetSharedResourceGrants(): UseMutationResult<
  void,
  unknown,
  { id: string; grants: Omit<SharedResourceGrant, "resource_id">[] }
> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, grants }) => {
      await apiClient.put(`/shared-resources/${id}/grants`, { grants });
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: [KEY] }),
  });
}
