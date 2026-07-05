import { afterEach, describe, expect, it } from "vitest";
import { filesDownloadURL } from "./filesApi";
import { ACT_AS_KEY } from "../../../impersonation";

// filesDownloadURL feeds window.open() (Download) and an <img src> (inline
// media preview). Both are full-navigation GETs that can't carry the
// X-Jabali-Act-As header the apiClient interceptor injects, so an admin
// impersonating a tenant must pass the grant as ?act_as or the request runs
// as the admin and 403s with not_in_scope. These tests lock that in.
describe("filesDownloadURL", () => {
  afterEach(() => {
    sessionStorage.removeItem(ACT_AS_KEY);
  });

  it("omits act_as when not impersonating", () => {
    const url = filesDownloadURL("/home/user/a.txt");
    expect(url).toBe("/api/v1/files/download?path=%2Fhome%2Fuser%2Fa.txt");
    expect(url).not.toContain("act_as");
  });

  it("appends the active grant as &act_as when impersonating", () => {
    sessionStorage.setItem(
      ACT_AS_KEY,
      JSON.stringify({ id: "grant_123", username: "taxrefound" }),
    );
    const url = filesDownloadURL("/home/taxrefound/domains/x/public_html/f.php");
    expect(url).toContain(
      "path=%2Fhome%2Ftaxrefound%2Fdomains%2Fx%2Fpublic_html%2Ff.php",
    );
    expect(url).toContain("&act_as=grant_123");
  });

  it("url-encodes the grant id", () => {
    sessionStorage.setItem(
      ACT_AS_KEY,
      JSON.stringify({ id: "a b/c", username: "u" }),
    );
    expect(filesDownloadURL("/x")).toContain("&act_as=a%20b%2Fc");
  });
});
