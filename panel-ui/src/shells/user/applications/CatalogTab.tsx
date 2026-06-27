// CatalogTab — the "browse & install" half of the Applications page,
// modelled on the admin Docker-Apps catalog: a masonry of cards, one per
// registered app descriptor (GET /applications/registry). Clicking a card
// opens the Install drawer pre-targeted to that app_type.
import { Alert, Button, Card, Empty, Space, Spin, Tag } from "antd";
import { useMemo, useState } from "react";
import { DatabaseOutlined } from "@icons";

import { useAppRegistry } from "./appRegistry";
import { CmsIcon } from "./CmsIcon";

type Props = {
  onInstall: (appType: string) => void;
};

export const CatalogTab = ({ onInstall }: Props) => {
  const { data: apps = [], isLoading, isError } = useAppRegistry();
  const [selectedTags, setSelectedTags] = useState<string[]>([]);

  // All tags present in the catalog, sorted, for the filter bar.
  const allTags = useMemo(() => {
    const set = new Set<string>();
    for (const a of apps) for (const t of a.tags ?? []) set.add(t);
    return Array.from(set).sort();
  }, [apps]);

  // OR-match: an app shows if it carries ANY selected tag (best for browsing).
  const visibleApps = useMemo(() => {
    if (selectedTags.length === 0) return apps;
    return apps.filter((a) => (a.tags ?? []).some((t) => selectedTags.includes(t)));
  }, [apps, selectedTags]);

  const toggleTag = (t: string) =>
    setSelectedTags((cur) =>
      cur.includes(t) ? cur.filter((x) => x !== t) : [...cur, t],
    );

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: 48 }}>
        <Spin />
      </div>
    );
  }

  if (isError) {
    return (
      <Alert
        type="error"
        showIcon
        message="Failed to load the application catalog"
        description="Try reloading the page."
      />
    );
  }

  if (apps.length === 0) {
    return (
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="No applications available"
      />
    );
  }

  return (
    <>
      {allTags.length > 0 && (
        <div style={{ marginBottom: 16 }}>
          <Space size={[8, 8]} wrap>
            {allTags.map((t) => (
              <Tag.CheckableTag
                key={t}
                checked={selectedTags.includes(t)}
                onChange={() => toggleTag(t)}
              >
                {t}
              </Tag.CheckableTag>
            ))}
            {selectedTags.length > 0 && (
              <Button type="link" size="small" onClick={() => setSelectedTags([])}>
                Clear
              </Button>
            )}
          </Space>
        </div>
      )}
      {visibleApps.length === 0 ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description="No applications match the selected tags"
        />
      ) : (
        <div
          style={{
            columnGap: 16,
            // As many >=320px columns as the viewport fits: 1 col mobile,
            // 2 tablet, 3+ desktop, no media queries. Same trick as the
            // docker-apps catalog masonry.
            columns: "320px",
          }}
        >
          {visibleApps.map((app) => (
        <div
          key={app.name}
          style={{
            breakInside: "avoid",
            marginBottom: 16,
            display: "inline-block",
            width: "100%",
          }}
        >
          <Card
            hoverable
            onClick={() => onInstall(app.name)}
            styles={{ body: { padding: 16 } }}
            actions={[
              <Button type="link" key="install">
                Install
              </Button>,
            ]}
          >
            <Card.Meta
              avatar={
                <div
                  style={{
                    width: 44,
                    height: 44,
                    borderRadius: 10,
                    background: "rgba(0, 0, 0, 0.03)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    flexShrink: 0,
                  }}
                >
                  <CmsIcon appType={app.name} size={26} />
                </div>
              }
              title={
                <Space size={6} wrap>
                  <span>{app.display_name}</span>
                  {app.requires_db && (
                    <Tag
                      icon={<DatabaseOutlined />}
                      style={{ marginInlineEnd: 0 }}
                    >
                      Database
                    </Tag>
                  )}
                </Space>
              }
              description={
                <div style={{ whiteSpace: "pre-line" }}>{app.description}</div>
              }
            />
          </Card>
            </div>
          ))}
        </div>
      )}
    </>
  );
};
