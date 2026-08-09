// Projection from AntD's onChange sorter payload to the list API's
// ?sort=&order= params.
//
// Extracted and hardened after the admin Users list shipped a Disk usage
// sorter that visibly toggled but changed nothing: the previous inline
// version took `sorter[0]`, and AntD only hands back an ARRAY when more
// than one column is sorted at once. Once any column was already sorted,
// clicking a second one appended to that array, `[0]` stayed the OLD
// column, and the URL — and therefore the query — never moved.
//
// The list API takes a single sort column (repository.ListOptions.Sort is
// one string), so the correct reading of a multi-entry payload is "the
// one the user just activated", not "the first one".
import type { SorterResult } from "antd/es/table/interface";

export type SorterParams = {
  sort: string | undefined;
  order: "asc" | "desc" | undefined;
};

/**
 * sorterToParams reduces AntD's sorter payload to the single (sort, order)
 * pair the list API understands.
 *
 * Undefined for both means "no active sort" — useTableURL.setParams deletes
 * those keys, which is what clears the ORDER BY rather than freezing it on
 * a stale column.
 */
export function sorterToParams<T>(
  sorter: SorterResult<T> | SorterResult<T>[] | undefined,
): SorterParams {
  const list = (Array.isArray(sorter) ? sorter : sorter ? [sorter] : []).filter(
    Boolean,
  ) as SorterResult<T>[];

  // Only entries carrying an order are actually sorted; AntD leaves the
  // others in the payload with order undefined after a cancel click.
  const active = list.filter((s) => s.order === "ascend" || s.order === "descend");
  if (active.length === 0) {
    return { sort: undefined, order: undefined };
  }

  // Last active entry = the most recently activated column. With a
  // single-column sorter there is only ever one, so this is exact; with a
  // multi-column table it is the closest thing to user intent that a
  // one-column API can honour.
  const picked = active[active.length - 1];
  const key = picked.columnKey ?? picked.field;
  if (key === undefined || key === null || Array.isArray(key)) {
    // A column with neither key nor a plain dataIndex cannot be named to
    // the API. Clear rather than send something the whitelist will reject.
    return { sort: undefined, order: undefined };
  }
  return {
    sort: String(key),
    order: picked.order === "ascend" ? "asc" : "desc",
  };
}
