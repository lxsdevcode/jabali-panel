// MailboxGroupsSection — view and edit a mailbox's distribution/resource
// group membership inline from the Edit Mailbox modal (GH #238). A multi-select
// preloaded with the mailbox's current groups; toggling applies immediately via
// add/remove member endpoints.
import { useMemo } from "react";
import { App, Divider, Select, Typography } from "antd";
import { useQuery } from "@tanstack/react-query";

import { apiClient } from "../../apiClient";
import {
  useMailGroups,
  useAddMailboxToGroup,
  useRemoveMailboxFromGroup,
} from "../../hooks/useMailGroups";

type Membership = { group_id: string; group_name: string; group_email: string };

interface MailboxGroupsSectionProps {
  mailboxId: string;
  domainId: string;
}

export function MailboxGroupsSection({ mailboxId, domainId }: MailboxGroupsSectionProps) {
  const { message } = App.useApp();
  const groups = useMailGroups(domainId);
  const add = useAddMailboxToGroup();
  const remove = useRemoveMailboxFromGroup();

  // Reuses the same query key the Mailboxes tab uses, so add/remove
  // invalidations refresh both the column and this control.
  const memberships = useQuery({
    queryKey: ["list", "mailbox-group-memberships", domainId],
    queryFn: async () => {
      const { data } = await apiClient.get<{ data: Record<string, Membership[]> }>(
        `/domains/${domainId}/mailbox-group-memberships`,
      );
      return data.data;
    },
    enabled: !!domainId,
  });

  const current = useMemo(
    () => (memberships.data?.[mailboxId] ?? []).map((g) => g.group_id),
    [memberships.data, mailboxId],
  );
  const options = useMemo(
    () => (groups.data ?? []).map((g) => ({ label: `${g.display_name} <${g.email}>`, value: g.id })),
    [groups.data],
  );

  const onChange = async (next: string[]) => {
    const toAdd = next.filter((id) => !current.includes(id));
    const toRemove = current.filter((id) => !next.includes(id));
    if (toAdd.length === 0 && toRemove.length === 0) return;
    try {
      for (const groupId of toAdd) {
        await add.mutateAsync({ groupId, mailboxId, domainId });
      }
      for (const groupId of toRemove) {
        await remove.mutateAsync({ groupId, mailboxId, domainId });
      }
      message.success("Groups updated");
    } catch {
      message.error("Failed to update groups");
    }
  };

  return (
    <>
      <Divider style={{ margin: "12px 0" }} />
      <Typography.Text strong>Groups</Typography.Text>
      <Select
        mode="multiple"
        style={{ width: "100%", marginTop: 8 }}
        placeholder={
          options.length ? "Add to distribution / resource groups" : "No groups on this domain yet"
        }
        value={current}
        options={options}
        onChange={onChange}
        loading={groups.isLoading || memberships.isLoading}
        disabled={add.isPending || remove.isPending}
        optionFilterProp="label"
        showSearch
        notFoundContent={options.length ? undefined : "Create groups in the Groups tab first"}
      />
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        Distribution and resource groups this mailbox belongs to. Changes apply immediately.
      </Typography.Text>
    </>
  );
}
