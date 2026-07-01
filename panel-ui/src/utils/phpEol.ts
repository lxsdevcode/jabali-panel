// php.net end-of-life dates (date security support ends). A version is EOL
// once its date has passed — running it, especially on a public site, means no
// upstream security patches (GH #329). Versions absent from this map are
// treated as still-supported (newer than every EOL entry).
export const PHP_EOL_DATES: Record<string, string> = {
  "5.6": "2018-12-31",
  "7.0": "2019-01-10",
  "7.1": "2019-12-01",
  "7.2": "2020-11-30",
  "7.3": "2021-12-06",
  "7.4": "2022-11-28",
  "8.0": "2023-11-26",
  "8.1": "2025-12-31",
  "8.2": "2026-12-31",
  "8.3": "2027-12-31",
  "8.4": "2028-12-31",
};

export const isPHPEOL = (version: string): boolean => {
  const eol = PHP_EOL_DATES[version];
  return eol !== undefined && new Date(eol) < new Date();
};
