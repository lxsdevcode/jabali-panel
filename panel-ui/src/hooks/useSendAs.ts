// useSendAs.ts — GH #347 send-as delegation hooks.
// A delegate mailbox may send FROM each grantor's address without receiving the
// grantor's mail. Backed by /mailboxes/:mbid/send-as (see mailbox_sendas.go).

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "../apiClient";

export interface SendAsGrantor {
  id: string; // delegation row id
  grantor_mailbox_id: string;
  grantor_email: string;
  delegate_mailbox_id: string;
  delegate_email: string;
  created_at: string;
}

const qk = (mailboxId: string) => ["sendas", mailboxId];

export function useSendAs(mailboxId: string | undefined) {
  return useQuery({
    queryKey: qk(mailboxId ?? ""),
    enabled: !!mailboxId,
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: SendAsGrantor[] }>(
        `/mailboxes/${mailboxId}/send-as`,
      );
      return data.data ?? [];
    },
  });
}

export function useAddSendAs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      mailboxId,
      grantorMailboxId,
    }: {
      mailboxId: string;
      grantorMailboxId: string;
    }) => {
      const { data } = await apiClient.post<SendAsGrantor>(
        `/mailboxes/${mailboxId}/send-as`,
        { grantor_mailbox_id: grantorMailboxId },
      );
      return data;
    },
    onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: qk(v.mailboxId) }),
  });
}

export function useDeleteSendAs() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      mailboxId,
      grantorMailboxId,
    }: {
      mailboxId: string;
      grantorMailboxId: string;
    }) => {
      await apiClient.delete(`/mailboxes/${mailboxId}/send-as/${grantorMailboxId}`);
    },
    onSuccess: (_d, v) => qc.invalidateQueries({ queryKey: qk(v.mailboxId) }),
  });
}
