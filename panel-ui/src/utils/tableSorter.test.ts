import { describe, expect, it } from "vitest";

import { sorterToParams } from "./tableSorter";

type Row = { id: string };

describe("sorterToParams", () => {
  it("maps a single sorted column to sort + order", () => {
    expect(
      sorterToParams<Row>({ columnKey: "disk_used_kb", order: "descend" }),
    ).toEqual({ sort: "disk_used_kb", order: "desc" });
  });

  it("maps ascend to asc", () => {
    expect(sorterToParams<Row>({ columnKey: "username", order: "ascend" })).toEqual({
      sort: "username",
      order: "asc",
    });
  });

  // The regression this helper exists for. AntD hands back an ARRAY once
  // more than one column is sorted; the previous inline code took [0], so
  // after the first column was sorted every later click kept sending the
  // OLD column. The header toggled, the URL never moved, and the list
  // stayed in its original order — "clicking it changes nothing".
  it("picks the most recently activated column, not the first in the array", () => {
    expect(
      sorterToParams<Row>([
        { columnKey: "username", order: "ascend" },
        { columnKey: "disk_used_kb", order: "descend" },
      ]),
    ).toEqual({ sort: "disk_used_kb", order: "desc" });
  });

  // Cancelling a sort has to CLEAR the params. Returning the stale column
  // would pin the ORDER BY forever.
  it("clears when nothing is sorted", () => {
    expect(sorterToParams<Row>({ columnKey: "username", order: undefined })).toEqual({
      sort: undefined,
      order: undefined,
    });
    expect(sorterToParams<Row>(undefined)).toEqual({
      sort: undefined,
      order: undefined,
    });
    expect(sorterToParams<Row>([])).toEqual({ sort: undefined, order: undefined });
  });

  // AntD leaves de-activated entries in the array with no order; they must
  // not win over the one the user actually clicked.
  it("ignores array entries that carry no order", () => {
    expect(
      sorterToParams<Row>([
        { columnKey: "username", order: undefined },
        { columnKey: "created_at", order: "ascend" },
      ]),
    ).toEqual({ sort: "created_at", order: "asc" });
  });

  it("falls back to field when columnKey is absent", () => {
    expect(sorterToParams<Row>({ field: "created_at", order: "ascend" })).toEqual({
      sort: "created_at",
      order: "asc",
    });
  });

  // A nested dataIndex cannot be named to the list API's single sort
  // column, and the backend whitelist would reject it — clear instead of
  // sending something guaranteed to be dropped.
  it("clears for an unnameable (array) field", () => {
    expect(
      sorterToParams<Row>({ field: ["a", "b"], order: "ascend" }),
    ).toEqual({ sort: undefined, order: undefined });
  });
});
