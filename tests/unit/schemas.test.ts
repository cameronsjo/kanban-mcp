/**
 * Regression lock for the `plankaId` tool-boundary validator.
 *
 * This schema exists specifically to reject a client-supplied ID before it
 * is interpolated into a Planka API URL path (path-traversal / cross-resource
 * injection). Every field that reaches a URL path segment must use it.
 */
import { describe, expect, it } from "@jest/globals";

import { plankaId } from "../../common/schemas.js";

describe("plankaId", () => {
  it("accepts a numeric Planka snowflake ID", () => {
    expect(plankaId.safeParse("1234567890123456789").success).toBe(true);
  });

  it("rejects a path-traversal payload", () => {
    expect(plankaId.safeParse("1/../../users").success).toBe(false);
  });

  it("rejects a non-numeric string", () => {
    expect(plankaId.safeParse("abc").success).toBe(false);
  });

  it("rejects an empty string", () => {
    expect(plankaId.safeParse("").success).toBe(false);
  });

  it("rejects a numeric ID with a trailing path segment", () => {
    expect(plankaId.safeParse("123/labelId:456").success).toBe(false);
  });
});
