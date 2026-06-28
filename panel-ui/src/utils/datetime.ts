// datetime — short, human-readable date/time formatters for table cells etc.
// Keeps timestamps compact ("25 Jun 2026, 04:04") instead of raw ISO
// ("2026-06-25T04:04:32.589695Z"). Invalid/empty input renders an em dash.
import dayjs from "dayjs";

const DASH = "—";

// shortDateTime: date + 24h time, no seconds. e.g. "25 Jun 2026, 04:04".
export function shortDateTime(ts: string | null | undefined): string {
  if (!ts) return DASH;
  const d = dayjs(ts);
  return d.isValid() ? d.format("DD MMM YYYY, HH:mm") : DASH;
}

// shortDate: date only. e.g. "25 Jun 2026".
export function shortDate(ts: string | null | undefined): string {
  if (!ts) return DASH;
  const d = dayjs(ts);
  return d.isValid() ? d.format("DD MMM YYYY") : DASH;
}
