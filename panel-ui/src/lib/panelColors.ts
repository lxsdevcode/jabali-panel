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
