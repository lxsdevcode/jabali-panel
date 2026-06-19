// LibravatarAvatar — shows a mailbox's Libravatar photo (federated,
// privacy-friendly Gravatar alternative), resolved client-side from the
// public CDN by the email's SHA-256. When the address has no avatar
// (CDN returns 404 via d=404) it falls back to a generic icon. Clicking
// always opens libravatar.org so the user can add or change their photo
// (Libravatar has no upload API; the user registers it on their own
// account there).
import { useEffect, useState } from "react";
import { Avatar, Tooltip } from "antd";
import { UserOutlined } from "@icons";

const LIBRAVATAR_SITE = "https://www.libravatar.org/";

async function sha256Hex(input: string): Promise<string> {
  const bytes = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

interface LibravatarAvatarProps {
  email: string;
  size?: number;
}

export function LibravatarAvatar({ email, size = 24 }: LibravatarAvatarProps) {
  const [src, setSrc] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setFailed(false);
    setSrc(null);
    const normalized = email.trim().toLowerCase();
    if (!normalized || !normalized.includes("@")) return;
    // s = 2x for retina; d=404 so a missing avatar surfaces as an error
    // and we fall through to the generic icon.
    sha256Hex(normalized)
      .then((hash) => {
        if (!cancelled) {
          setSrc(
            `https://seccdn.libravatar.org/avatar/${hash}?d=404&s=${size * 2}`,
          );
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [email, size]);

  const hasAvatar = src && !failed;
  // Native <img> (via AntD Avatar's ReactNode src) so we can set
  // referrerPolicy=no-referrer — otherwise the panel URL leaks to
  // Libravatar/its CDN in the Referer header on every avatar fetch.
  const avatar = hasAvatar ? (
    <Avatar
      size={size}
      src={
        <img
          src={src}
          alt=""
          referrerPolicy="no-referrer"
          onError={() => setFailed(true)}
        />
      }
    />
  ) : (
    <Avatar size={size} icon={<UserOutlined />} />
  );

  return (
    <Tooltip
      title={
        hasAvatar
          ? "Photo from libravatar.org — click to manage it"
          : "Looks up a photo on libravatar.org (your address hash is sent there) — click to add one"
      }
    >
      <a
        href={LIBRAVATAR_SITE}
        target="_blank"
        rel="noopener noreferrer"
        style={{ display: "inline-flex", flexShrink: 0 }}
        aria-label="Manage avatar on libravatar.org"
      >
        {avatar}
      </a>
    </Tooltip>
  );
}
