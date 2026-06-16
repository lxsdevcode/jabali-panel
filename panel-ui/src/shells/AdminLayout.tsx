// AdminLayout.tsx — chrome for the admin shell.
//
// Full-width Header on top (brand + search + user menu), then either
// a persistent <Sider> (≥lg / 992px) or an off-canvas <Drawer> (<lg)
// that the header's hamburger button opens. See ADR-0046.
import { useEffect, useState } from "react";
import { LeftOutlined, RightOutlined } from "@icons";
import { Drawer, Grid, Layout, Menu, theme } from "antd";
import { Outlet, useLocation, useNavigate } from "react-router";

import { apiClient } from "../apiClient";
import { JabaliFooter } from "../components/JabaliFooter";
import { JabaliHeader } from "../components/JabaliHeader";
import { JabaliTitle } from "../components/JabaliTitle";
import { adminNav, selectedNavKey } from "../nav";
import { useThemeMode } from "../theme/ThemeModeContext";
import { QuickStartModal } from "./admin/QuickStartModal";

const { Sider, Content } = Layout;

export function AdminLayout() {
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const location = useLocation();
  const navigate = useNavigate();
  const { mode } = useThemeMode();
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  // screens.lg is undefined on the first render before AntD measures the
  // viewport. Fall back to window.innerWidth so mobile users see the
  // hamburger on initial paint rather than the desktop Sider.
  const isDesktop = screens.lg ?? (typeof window !== "undefined" ? window.innerWidth >= 992 : true);

  // M48: hide the Docker Apps nav entry until the operator opts in
  // via Server Settings -> Apps. /me/server-capabilities is cached
  // per-session; UI ergonomics outweigh staleness here.
  const [dockerEnabled, setDockerEnabled] = useState<boolean>(false);
  useEffect(() => {
    apiClient
      .get<{ docker_marketplace_enabled?: boolean }>("/me/server-capabilities")
      .then((r) => setDockerEnabled(!!r.data.docker_marketplace_enabled))
      .catch(() => setDockerEnabled(false));
  }, []);
  const visibleNav = dockerEnabled
    ? adminNav
    : adminNav.filter((n) => n.key !== "docker-apps");

  const selected = selectedNavKey(visibleNav, location.pathname);

  // Light mode: explicit Tailwind gray-50 / gray-100 per operator request
  // so the sidebar sits a shade paler than the main card surface and the
  // active menu row reads slightly darker than the sidebar body. Dark
  // mode keeps the layout-bg token (it already pairs well with the
  // algorithm-derived itemSelectedBg).
  const siderBg = mode === "dark" ? token.colorBgLayout : "#f9fafb";

  // Single source of truth for the menu items — used by both <Sider>
  // and <Drawer> so the two shell variants stay in lock-step.
  const menu = (
    <Menu
      mode="inline"
      theme={mode}
      selectedKeys={selected ? [selected] : []}
      style={{ border: "none", background: siderBg }}
      items={visibleNav.map((n) => ({
        key: n.key,
        icon: n.icon,
        label: n.label,
        onClick: () => {
          navigate(n.path);
          setDrawerOpen(false);
        },
      }))}
    />
  );

  // Close the drawer on every route change — covers not just menu
  // clicks but also back-button / programmatic navigation.
  useEffect(() => {
    setDrawerOpen(false);
  }, [location.pathname]);

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <JabaliHeader
        showMenuButton={!isDesktop}
        onMenuClick={() => setDrawerOpen(true)}
      />
      <Layout>
        {isDesktop ? (
          <Sider
            theme={mode}
            width={256}
            breakpoint="lg"
            collapsedWidth="64"
            collapsible
            collapsed={collapsed}
            onCollapse={setCollapsed}
            trigger={
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  width: "100%",
                  height: "100%",
                  color: token.colorTextSecondary,
                  background: siderBg,
                }}
              >
                {collapsed ? <RightOutlined /> : <LeftOutlined />}
              </div>
            }
            style={{ background: siderBg, paddingTop: 16, paddingInline: 8 }}
          >
            {menu}
          </Sider>
        ) : (
          <Drawer
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
            placement="left"
            width={256}
            closable
            title={<JabaliTitle />}
            styles={{
              body: { padding: 8, background: siderBg },
              header: { background: siderBg },
            }}
          >
            {menu}
          </Drawer>
        )}
        <Layout>
          <Content
            style={{
              // Extra top gap so the page heading breathes away from
              // the header's bottom border. Horizontal + bottom stay
              // at the baseline gutter.
              padding: screens.md ? "32px 24px 24px" : "20px 12px 12px",
              // minWidth:0 lets this flex child shrink instead of forcing
              // the page wider than the viewport; overflowX hidden is the
              // backstop so a single wide element can't sideways-scroll the
              // whole page on mobile (tables keep their own inner scroll).
              minWidth: 0,
              overflowX: "hidden",
            }}
          >
            <Outlet />
            <QuickStartModal />
          </Content>
          <JabaliFooter />
        </Layout>
      </Layout>
    </Layout>
  );
}
