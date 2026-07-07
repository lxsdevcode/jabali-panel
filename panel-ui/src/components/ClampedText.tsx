// ClampedText — Codeberg #8. Clamp long banner/alert text to `lines` lines on
// mobile (< md) with an inline "Read more" / "Show less" toggle; desktop always
// shows the full text. Shared so every panel banner/notice behaves the same and
// long copy (e.g. the impersonation strip) never eats the viewport on 320-390px.
//
// The toggle inherits the surrounding text color (currentColor + underline) so
// it stays legible on any banner background without per-callsite styling.
import { useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { Grid } from "antd";

const { useBreakpoint } = Grid;

interface ClampedTextProps {
  children: ReactNode;
  /** Lines to show before clamping on mobile. Default 2. */
  lines?: number;
  moreLabel?: string;
  lessLabel?: string;
  /** Style applied to the text span (color/weight of the banner copy). */
  style?: CSSProperties;
}

export function ClampedText({
  children,
  lines = 2,
  moreLabel = "Read more",
  lessLabel = "Show less",
  style,
}: ClampedTextProps) {
  const screens = useBreakpoint();
  const isMobile = !screens.md;
  const ref = useRef<HTMLSpanElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [overflowing, setOverflowing] = useState(false);

  const clamp = isMobile && !expanded;

  // Measure overflow only while clamped: clientHeight is the clamped box (N
  // lines), scrollHeight is the full text. Re-run when breakpoint, expanded
  // state, or content changes.
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el || !clamp) {
      setOverflowing(false);
      return;
    }
    setOverflowing(el.scrollHeight > el.clientHeight + 1);
  }, [clamp, children]);

  const clampStyle: CSSProperties = clamp
    ? {
        display: "-webkit-box",
        WebkitLineClamp: lines,
        WebkitBoxOrient: "vertical",
        overflow: "hidden",
      }
    : {};

  return (
    <>
      <span ref={ref} style={{ ...clampStyle, ...style }}>
        {children}
      </span>
      {isMobile && (overflowing || expanded) && (
        <span
          role="button"
          tabIndex={0}
          onClick={() => setExpanded((v) => !v)}
          onKeyDown={(e) => {
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              setExpanded((v) => !v);
            }
          }}
          style={{
            display: "inline-block",
            marginTop: 2,
            textDecoration: "underline",
            cursor: "pointer",
            fontWeight: 600,
            opacity: 0.9,
          }}
        >
          {expanded ? lessLabel : moreLabel}
        </span>
      )}
    </>
  );
}
