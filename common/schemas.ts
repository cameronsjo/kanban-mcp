import { z } from "zod";

/**
 * Planka entity IDs are numeric snowflakes. Validating client-supplied IDs at
 * the tool boundary rejects path-traversal / cross-resource injection (e.g. an
 * id like "1/../../users") before it is ever interpolated into an API path.
 *
 * Lives in its own module (rather than inline in index.ts) so it can be
 * imported by tests without triggering index.ts's module-level
 * `runServer()` side effect.
 */
export const plankaId = z.string().regex(/^\d+$/, "must be a numeric Planka ID");
