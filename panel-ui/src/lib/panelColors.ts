// panelColors — single source of truth for the operator-customizable "Look and
// feel" panel colors (#433 follow-up). Used by LookAndFeelCard (render) and
// App.tsx (apply via the antd ConfigProvider). Each maps to a server_settings
// column / public-branding field; `token` is the antd seed token it drives
// (null = the component-level "accent" handled directly in muiTheme).
export type PanelColorDef = {
  field: string;
  token: string | null;
  label: string;
  tip: string;
};

export const PANEL_COLORS: PanelColorDef[] = [
  {
    field: "panel_primary_color",
    token: "colorPrimary",
    label: "Primary",
    tip: "Buttons, links, primary actions and selected toggles. Clear = default blue.",
  },
  {
    field: "panel_accent_color",
    token: null,
    label: "Accent",
    tip: "Selected sidebar row and active tab. Clear = default.",
  },
  {
    field: "panel_success_color",
    token: "colorSuccess",
    label: "Success",
    tip: "Success messages, tags and OK states. Clear = default green.",
  },
  {
    field: "panel_warning_color",
    token: "colorWarning",
    label: "Warning",
    tip: "Warnings and caution states. Clear = default gold.",
  },
  {
    field: "panel_error_color",
    token: "colorError",
    label: "Error / danger",
    tip: "Errors and destructive actions like delete. Clear = default red.",
  },
  {
    field: "panel_info_color",
    token: "colorInfo",
    label: "Info",
    tip: "Informational highlights and badges. Clear = default blue.",
  },
  {
    field: "panel_link_color",
    token: "colorLink",
    label: "Link",
    tip: "Hyperlink text colour. Clear = follows primary.",
  },
];


// PANEL_CHROME_COLORS — per-theme (light + dark) chrome colors (GH #435).
// Unlike the brand colors above (single value, read fine in both themes), each
// chrome surface needs separate light + dark values. Applied in App.tsx /
// muiTheme.ts: bg -> colorBgLayout, container -> colorBgContainer, text ->
// colorTextBase, topbar -> the --jabali-header-bg CSS var. Empty = default.
export type PanelChromeColorDef = {
  field: string;
  mode: "light" | "dark";
  label: string;
  tip: string;
};

export const PANEL_CHROME_COLORS: PanelChromeColorDef[] = [
  { field: "panel_light_topbar_color", mode: "light", label: "Top bar (light)", tip: "Header bar background in light mode. Clear = default." },
  { field: "panel_dark_topbar_color", mode: "dark", label: "Top bar (dark)", tip: "Header bar background in dark mode. Clear = default." },
  { field: "panel_light_bg_color", mode: "light", label: "Page / sidebar (light)", tip: "Page + sidebar background in light mode. Clear = default." },
  { field: "panel_dark_bg_color", mode: "dark", label: "Page / sidebar (dark)", tip: "Page + sidebar background in dark mode. Clear = default." },
  { field: "panel_light_container_color", mode: "light", label: "Card (light)", tip: "Card / container background in light mode. Clear = default." },
  { field: "panel_dark_container_color", mode: "dark", label: "Card (dark)", tip: "Card / container background in dark mode. Clear = default." },
  { field: "panel_light_text_color", mode: "light", label: "Text (light)", tip: "Base text color in light mode. Clear = default." },
  { field: "panel_dark_text_color", mode: "dark", label: "Text (dark)", tip: "Base text color in dark mode. Clear = default." },
];
