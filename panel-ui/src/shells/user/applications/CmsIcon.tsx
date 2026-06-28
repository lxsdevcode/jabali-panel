import { SiWordpress, SiWikipedia, SiDrupal, SiJoomla, SiPhpbb, SiPrestashop, SiMoodle } from "react-icons/si";
import { FaOpencart, FaRegComments, FaBriefcase } from "react-icons/fa6";
import { BookOutlined } from "@icons";
import type { CSSProperties } from "react";

interface CmsIconProps {
  appType: string | undefined;
  size?: number;
}

// Brand colors for the CMS logos. Falling back to currentColor for the
// generic icon so themed dark/light backgrounds keep contrast without a
// per-mode override.
const BRAND_COLOR: Record<string, string> = {
  wordpress: "#21759B",
  mediawiki: "#990000",
  drupal: "#0678BE",
  joomla: "#F44321",
  phpbb: "#26477F",
  opencart: "#2AC2EF",
  prestashop: "#DF0067",
  flarum: "#5d61d6",
  itflow: "#0d6efd",
  moodle: "#f98012",
};

// DokuWikiMark — react-icons has no DokuWiki brand glyph, so render its logo
// inline (mirrors the docker-app catalog icon.svg).
function DokuWikiMark({ size }: { size: number }) {
  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      style={{ flexShrink: 0 }}
      role="img"
      aria-label="DokuWiki"
    >
      <rect width="64" height="64" rx="12" fill="#88bb11" />
      <path
        fill="#fff"
        d="M18 16h17a13 13 0 0 1 0 26H18a2 2 0 0 1-2-2V18a2 2 0 0 1 2-2zm6 6v14h11a7 7 0 0 0 0-14H24z"
      />
      <rect x="16" y="46" width="32" height="4" rx="2" fill="#fff" opacity="0.85" />
    </svg>
  );
}

// CmsIcon renders the brand logo for an app_type. Unknown app_type falls
// back to the AntD BookOutlined generic icon.
export function CmsIcon({ appType, size = 18 }: CmsIconProps) {
  const key = appType || "wordpress";
  const color = BRAND_COLOR[key];
  const style: CSSProperties = {
    fontSize: size,
    width: size,
    height: size,
    color,
    flexShrink: 0,
  };

  if (key === "wordpress") {
    return <SiWordpress style={style} />;
  }
  if (key === "mediawiki") {
    return <SiWikipedia style={style} />;
  }
  if (key === "drupal") {
    return <SiDrupal style={style} />;
  }
  if (key === "joomla") {
    return <SiJoomla style={style} />;
  }
  if (key === "phpbb") {
    return <SiPhpbb style={style} />;
  }
  if (key === "opencart") {
    return <FaOpencart style={style} />;
  }
  if (key === "prestashop") {
    return <SiPrestashop style={style} />;
  }
  if (key === "flarum") {
    return <FaRegComments style={style} />;
  }
  if (key === "itflow") {
    return <FaBriefcase style={style} />;
  }
  if (key === "moodle") {
    return <SiMoodle style={style} />;
  }
  if (key === "dokuwiki") {
    return <DokuWikiMark size={size} />;
  }
  return <BookOutlined style={style} />;
}
