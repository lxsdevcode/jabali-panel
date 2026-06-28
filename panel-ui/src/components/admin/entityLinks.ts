// entityLinks — single source of truth for admin entity URLs + breadcrumb
// crumb derivation (#483, ADR-0152).
//
// Keep ALL admin entity hrefs here so a route rename is one edit, and so the
// owner-scoped query param (?user_id=) is built one way everywhere. Only
// resources that have a real admin page are linkable: domains, mailboxes
// (AdminMailPage), docker apps, backups, plus the package detail. Databases and
// python apps are user-shell only — no admin page — so they are intentionally
// absent here.

const ADMIN = "/jabali-admin";

const withOwner = (base: string, userId?: string): string =>
  userId ? `${base}?user_id=${encodeURIComponent(userId)}` : base;

export const adminLinks = {
  users: () => `${ADMIN}/users`,
  user: (id: string) => `${ADMIN}/users/${encodeURIComponent(id)}`,
  domains: (userId?: string) => withOwner(`${ADMIN}/domains`, userId),
  domainEdit: (id: string) => `${ADMIN}/domains/edit/${encodeURIComponent(id)}`,
  mailboxes: (userId?: string) => withOwner(`${ADMIN}/mail`, userId),
  dockerApps: (userId?: string) => withOwner(`${ADMIN}/docker-apps`, userId),
  backups: (userId?: string) => withOwner(`${ADMIN}/backups`, userId),
  packageEdit: (id: string) => `${ADMIN}/packages/edit/${encodeURIComponent(id)}`,
} as const;

export type CrumbMenuItem = { key: string; label: string; href: string };
export type Crumb = { title: string; href?: string; menu?: CrumbMenuItem[] };

export type OwnerRef = { id: string; username?: string | null };

// ownerLabel is the display name for a user crumb — username when known, else a
// short id prefix so the trail is never blank.
export const ownerLabel = (user: OwnerRef): string =>
  user.username && user.username.trim() !== "" ? user.username : user.id.slice(0, 8);

// The owner-scoped resource types that have an admin page. Used to build the
// sibling dropdown so a user's "Domains" crumb can pivot to their other
// resources (the bidirectional navigation the issue asks for).
const OWNER_RESOURCES: { key: string; label: string; href: (userId: string) => string }[] = [
  { key: "domains", label: "Domains", href: (id) => adminLinks.domains(id) },
  { key: "mailboxes", label: "Mailboxes", href: (id) => adminLinks.mailboxes(id) },
  { key: "docker-apps", label: "Docker Apps", href: (id) => adminLinks.dockerApps(id) },
  { key: "backups", label: "Backups", href: (id) => adminLinks.backups(id) },
];

// userResourceSiblings returns the dropdown menu for an owner-scoped resource
// crumb: links to that same owner's OTHER resource types (current excluded).
export function userResourceSiblings(userId: string, currentKey?: string): CrumbMenuItem[] {
  return OWNER_RESOURCES.filter((r) => r.key !== currentKey).map((r) => ({
    key: r.key,
    label: r.label,
    href: r.href(userId),
  }));
}

// ownerCrumbs: Users › <name>. The user crumb is the link to the hub.
export function ownerCrumbs(user: OwnerRef): Crumb[] {
  return [
    { title: "Users", href: adminLinks.users() },
    { title: ownerLabel(user) },
  ];
}

// ownerResourceCrumbs: Users › <name> › <Resource>, with the resource crumb
// carrying a sibling dropdown to pivot across the same owner's resources.
export function ownerResourceCrumbs(
  user: OwnerRef,
  resource: { key: string; label: string },
): Crumb[] {
  return [
    { title: "Users", href: adminLinks.users() },
    { title: ownerLabel(user), href: adminLinks.user(user.id) },
    { title: resource.label, menu: userResourceSiblings(user.id, resource.key) },
  ];
}

// resourceListCrumbs: a non-owner-scoped list page (e.g. all Domains).
export function resourceListCrumbs(label: string): Crumb[] {
  return [{ title: label }];
}
