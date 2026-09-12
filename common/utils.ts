import { getUserAgent } from "universal-user-agent";
import { createPlankaError } from "./errors.js";
import { VERSION } from "./version.js";

// Global variables to store tokens
let agentToken: string | null = null;
// Dedupes concurrent re-authentication: without this, N requests that all
// see a 401 on the same expired token each trigger their own full
// authenticateAgent() call (a password login + a new Planka session row
// per call), and the request that installs a fresh token can have that
// fresh token immediately cleared by a sibling still reacting to the old one.
let authInFlight: Promise<string> | null = null;

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  skipAuth?: boolean;
};

/**
 * Strip userinfo (username:password@) before a URL reaches a log line or
 * error message. `PLANKA_BASE_URL` is operator-configured, not attacker
 * input, but a URL embedding basic-auth credentials should never round-trip
 * into an error the calling model (or CI logs) can see.
 */
function redactUrlForLogging(url: string): string {
  try {
    const parsed = new URL(url);
    parsed.username = "";
    parsed.password = "";
    return parsed.toString();
  } catch {
    return url;
  }
}

async function parseResponseBody(response: Response): Promise<unknown> {
  const contentType = response.headers.get("content-type");
  if (contentType?.includes("application/json")) {
    return response.json();
  }
  return response.text();
}

export function buildUrl(
  baseUrl: string,
  params: Record<string, string | number | undefined>,
): string {
  const url = new URL(baseUrl);
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined) {
      url.searchParams.append(key, value.toString());
    }
  });
  return url.toString();
}

const USER_AGENT =
  `modelcontextprotocol/servers/planka/v${VERSION} ${getUserAgent()}`;

async function authenticateAgent(): Promise<string> {
  const email = process.env.PLANKA_AGENT_EMAIL;
  const password = process.env.PLANKA_AGENT_PASSWORD;

  if (!email || !password) {
    throw new Error(
      "PLANKA_AGENT_EMAIL and PLANKA_AGENT_PASSWORD environment variables are required",
    );
  }

  const baseUrl = process.env.PLANKA_BASE_URL || "http://localhost:3000";
  // Normalize the base URL to not end with /api
  const normalizedBaseUrl = baseUrl.endsWith("/api")
    ? baseUrl.slice(0, -4)
    : baseUrl;

  const url = new URL("/api/access-tokens", normalizedBaseUrl).toString();

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Accept": "application/json",
        "Content-Type": "application/json",
        "User-Agent": USER_AGENT,
      },
      body: JSON.stringify({
        emailOrUsername: email,
        password: password,
      }),
      credentials: "include",
    });

    const responseBody = await parseResponseBody(response);

    // Planka 2.1.x gates the first login behind terms-of-service acceptance,
    // returning 403 + a pendingToken instead of a token. Surface an actionable
    // error rather than an opaque permission failure.
    if (
      !response.ok &&
      typeof responseBody === "object" &&
      responseBody !== null &&
      (responseBody as { step?: string }).step === "accept-terms"
    ) {
      throw new Error(
        "Planka requires terms-of-service acceptance for this agent account " +
          "before an access token can be issued. This is an operator-side setup " +
          "step, not something the calling model can complete: accept terms once " +
          "in the Planka UI, or ask the operator to run " +
          "`scripts/accept-planka-terms.sh` (it reads the password from the " +
          "PLANKA_AGENT_PASSWORD env var or a stdin prompt, never a command-line " +
          "argument), then retry.",
      );
    }

    if (!response.ok) {
      throw createPlankaError(response.status, responseBody);
    }

    // The token is directly in the item field
    const { item } = responseBody as { item: string };
    agentToken = item;
    return item;
  } catch (error: unknown) {
    // Rethrow with more context
    const errorMessage = error instanceof Error ? error.message : String(error);
    throw new Error(
      `Failed to authenticate agent with Planka: ${errorMessage}`,
    );
  }
}

async function getAuthToken(): Promise<string> {
  if (agentToken) {
    return agentToken;
  }
  // Concurrent callers share one in-flight login instead of each starting
  // their own — see the authInFlight comment above.
  if (!authInFlight) {
    authInFlight = authenticateAgent().finally(() => {
      authInFlight = null;
    });
  }
  return authInFlight;
}

export async function plankaRequest(
  path: string,
  options: RequestOptions = {},
): Promise<unknown> {
  const baseUrl = process.env.PLANKA_BASE_URL || "http://localhost:3000";

  // Normalize the base URL to not end with /api
  const normalizedBaseUrl = baseUrl.endsWith("/api")
    ? baseUrl.slice(0, -4)
    : baseUrl;

  // Ensure path starts with /api/
  const normalizedPath = path.startsWith("/api/") ? path : `/api/${path}`;

  const url = new URL(normalizedPath, normalizedBaseUrl).toString();

  // Two attempts: if a cached token is rejected with 401, clear it and
  // re-authenticate once so a long-lived server self-heals from an expired or
  // revoked token instead of failing every request until restart.
  for (let attempt = 0; attempt < 2; attempt++) {
    const headers: Record<string, string> = {
      "Accept": "application/json",
      "Content-Type": "application/json",
      "User-Agent": USER_AGENT,
      ...options.headers,
    };

    // Remove Content-Type header for FormData
    if (options.body instanceof FormData) {
      delete headers["Content-Type"];
    }

    // Add authentication token if not skipped
    let usedToken: string | undefined;
    if (!options.skipAuth) {
      try {
        const token = await getAuthToken();
        usedToken = token;
        headers["Authorization"] = `Bearer ${token}`;
      } catch (error: unknown) {
        const errorMessage = error instanceof Error
          ? error.message
          : String(error);
        throw new Error(`Failed to get authentication token: ${errorMessage}`);
      }
    }

    let response: Response;
    let responseBody: unknown;
    try {
      response = await fetch(url, {
        method: options.method || "GET",
        headers,
        body: options.body instanceof FormData
          ? options.body
          : options.body
          ? JSON.stringify(options.body)
          : undefined,
        credentials: "include", // Include cookies for Planka authentication
      });

      responseBody = await parseResponseBody(response);
    } catch (error: unknown) {
      const errorMessage = error instanceof Error
        ? error.message
        : String(error);
      throw new Error(
        `Failed to make Planka request to ${redactUrlForLogging(url)}: ${errorMessage}`,
      );
    }

    if (
      response.status === 401 && !options.skipAuth && attempt === 0 &&
      usedToken && agentToken === usedToken
    ) {
      // Cached token rejected — drop it and retry with a fresh login.
      // Only clear when the CURRENT cached token is still the one THIS
      // request used: a sibling request may have already refreshed
      // agentToken to a new, valid value while this one was in flight, and
      // clearing unconditionally would discard that fresh token too.
      agentToken = null;
      continue;
    }

    if (!response.ok) {
      const plankaError = createPlankaError(response.status, responseBody);
      throw new Error(
        `Failed to make Planka request to ${redactUrlForLogging(url)}: ${plankaError.message}`,
      );
    }

    return responseBody;
  }

  // Unreachable: every path inside the loop returns or throws.
  throw new Error(
    `Failed to make Planka request to ${redactUrlForLogging(url)}: authentication retry exhausted`,
  );
}

export function validateProjectName(name: string): string {
  const sanitized = name.trim();
  if (!sanitized) {
    throw new Error("Project name cannot be empty");
  }
  return sanitized;
}

export function validateBoardName(name: string): string {
  const sanitized = name.trim();
  if (!sanitized) {
    throw new Error("Board name cannot be empty");
  }
  return sanitized;
}

export function validateListName(name: string): string {
  const sanitized = name.trim();
  if (!sanitized) {
    throw new Error("List name cannot be empty");
  }
  return sanitized;
}

export function validateCardName(name: string): string {
  const sanitized = name.trim();
  if (!sanitized) {
    throw new Error("Card name cannot be empty");
  }
  return sanitized;
}

/**
 * Looks up a user ID by email
 *
 * @param {string} email - The email of the user to look up
 * @returns {Promise<string | null>} The user ID if found, null otherwise
 */
export async function getUserIdByEmail(email: string): Promise<string | null> {
  // Let request errors propagate so a failed lookup is distinguishable from
  // "no such user" (which legitimately returns null).
  const response = await plankaRequest("/api/users");
  const { items } = response as {
    items: Array<{ id: string; email: string }>;
  };

  const user = items.find((user) => user.email === email);
  return user ? user.id : null;
}

/**
 * Looks up a user ID by username
 *
 * @param {string} username - The username of the user to look up
 * @returns {Promise<string | null>} The user ID if found, null otherwise
 */
export async function getUserIdByUsername(
  username: string,
): Promise<string | null> {
  // Let request errors propagate so a failed lookup is distinguishable from
  // "no such user" (which legitimately returns null).
  const response = await plankaRequest("/api/users");
  const { items } = response as {
    items: Array<{ id: string; username: string }>;
  };

  const user = items.find((user) => user.username === username);
  return user ? user.id : null;
}
