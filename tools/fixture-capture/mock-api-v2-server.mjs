#!/usr/bin/env node
import http from "node:http";

const jsonHeaders = {
  "content-type": "application/json",
  "x-request-id": "mock-request-id",
};

const secretEchoFields = new Set([
  "authorization",
  "apikey",
  "key",
  "plaintextkey",
  "clientsecret",
  "client_secret",
  "access_token",
  "accesstoken",
  "refresh_token",
  "refreshtoken",
  "credential",
  "credentials",
  "providertoken",
  "webhooksecret",
  "password",
  "newpassword",
  "currentpassword",
  "secret",
]);

function sendJson(res, status, body) {
  res.writeHead(status, jsonHeaders);
  res.end(`${JSON.stringify(body)}\n`);
}

function bearerToken(req) {
  const header = req.headers.authorization ?? "";
  return header.startsWith("Bearer ") ? header.slice("Bearer ".length) : "";
}

function routePath(req) {
  return new URL(req.url ?? "/", "http://localhost").pathname;
}

async function readJsonBody(req) {
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  if (chunks.length === 0) return {};
  const raw = Buffer.concat(chunks).toString("utf8");
  return raw ? JSON.parse(raw) : {};
}

function bookingPayload(overrides = {}) {
  return {
    uid: "mock-booking-personal-basic",
    id: 987,
    title: "Fixture Event",
    status: "accepted",
    start: "2026-05-01T15:00:00.000Z",
    end: "2026-05-01T15:30:00.000Z",
    eventTypeId: 1001,
    attendees: [
      {
        id: 321,
        name: "Fixture Attendee",
        email: "fixture-attendee@example.test",
        timeZone: "America/Chicago",
      },
    ],
    responses: {
      name: "Fixture Attendee",
      email: "fixture-attendee@example.test",
    },
    metadata: {
      fixture: "personal-basic",
    },
    createdAt: "2026-01-01T00:00:00.000Z",
    updatedAt: "2026-01-01T00:00:00.000Z",
    requestId: "mock-request-id",
    ...overrides,
  };
}

function slotsPayload(overrides = {}) {
  return {
    eventTypeId: 1001,
    timeZone: "America/Chicago",
    start: "2026-05-01T00:00:00.000Z",
    end: "2026-05-02T00:00:00.000Z",
    slots: {
      "2026-05-01": [
        {
          time: "2026-05-01T15:00:00.000Z",
          duration: 30,
        },
      ],
    },
    requestId: "mock-request-id",
    ...overrides,
  };
}

function hasSecretEchoField(value) {
  if (Array.isArray(value)) return value.some((item) => hasSecretEchoField(item));
  if (!value || typeof value !== "object") return false;
  return Object.entries(value).some(([key, child]) => {
    const normalizedKey = key.toLowerCase();
    return secretEchoFields.has(normalizedKey) || hasSecretEchoField(child);
  });
}

function authorized(req) {
  return bearerToken(req) === "cal_test_valid_mock";
}

function authenticatedBookingPrincipal(req) {
  const token = bearerToken(req);
  return token === "cal_test_valid_mock" || token === "cal_test_wrong_owner_mock";
}

function ownsFixtureBooking(req) {
  return bearerToken(req) === "cal_test_valid_mock";
}

export function createMockApiV2Server() {
  const bookings = new Map();
  const idempotency = new Map();

  const ensureBooking = () => {
    if (!bookings.has("mock-booking-personal-basic")) {
      bookings.set("mock-booking-personal-basic", bookingPayload());
    }
    return bookings.get("mock-booking-personal-basic");
  };
  const ensurePendingBooking = (uid) => {
    if (!bookings.has(uid)) {
      bookings.set(
        uid,
        bookingPayload({
          uid,
          id: uid === "mock-booking-pending-decline" ? 989 : 988,
          status: "pending",
          start: "2026-05-03T15:00:00.000Z",
          end: "2026-05-03T15:30:00.000Z",
          metadata: {
            fixture: "pending-host-action",
          },
        })
      );
    }
    return bookings.get(uid);
  };

  return http.createServer(async (req, res) => {
    const path = routePath(req);

    if (req.method === "GET" && path === "/health") {
      res.writeHead(200, { "content-type": "text/plain", "x-request-id": "mock-request-id" });
      res.end("OK");
      return;
    }

    if (req.method === "GET" && path === "/v2/me") {
      if (!authorized(req)) {
        sendJson(res, 401, {
          status: "error",
          error: {
            code: "UNAUTHORIZED",
            message: "Invalid credentials",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      sendJson(res, 200, {
        status: "success",
        data: {
          id: 123,
          uuid: "00000000-0000-4000-8000-000000000123",
          username: "fixture-user",
          email: "fixture-user@example.test",
          createdAt: "2026-01-01T00:00:00.000Z",
          updatedAt: "2026-01-01T00:00:00.000Z",
          requestId: "mock-request-id",
        },
      });
      return;
    }

    if (req.method === "GET" && path === "/v2/slots") {
      const url = new URL(req.url ?? "/", "http://localhost");
      const eventTypeId = Number(url.searchParams.get("eventTypeId") ?? 0);
      if (eventTypeId !== 1001) {
        sendJson(res, 404, {
          status: "error",
          error: {
            code: "NOT_FOUND",
            message: "Slots not found",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      sendJson(res, 200, {
        status: "success",
        data: slotsPayload({
          start: url.searchParams.get("start") ?? "2026-05-01T00:00:00.000Z",
          end: url.searchParams.get("end") ?? "2026-05-02T00:00:00.000Z",
          timeZone: url.searchParams.get("timeZone") ?? "America/Chicago",
        }),
      });
      return;
    }

    if (req.method === "POST" && path === "/v2/bookings") {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 403, {
          status: "error",
          error: {
            code: "FORBIDDEN",
            message: "Insufficient permissions",
            requestId: "mock-request-id",
          },
        });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, {
          status: "error",
          error: {
            code: "FORBIDDEN",
            message: "Insufficient permissions",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      const body = await readJsonBody(req);
      if (hasSecretEchoField(body.responses) || hasSecretEchoField(body.metadata)) {
        sendJson(res, 400, {
          status: "error",
          error: {
            code: "SECRET_FIELD_NOT_ALLOWED",
            message: "Secret-bearing fields are not allowed in booking responses or metadata",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      const requestedStart = body.start ?? "2026-05-01T15:00:00.000Z";
      const requestedTimeZone = body.attendee?.timeZone ?? "America/Chicago";
      if (body.eventTypeId === 1001 && (requestedStart !== "2026-05-01T15:00:00.000Z" || requestedTimeZone !== "America/Chicago")) {
        sendJson(res, 400, {
          status: "error",
          error: {
            code: "SLOT_UNAVAILABLE",
            message: "Requested slot is unavailable",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      const idempotencyKey = body.idempotencyKey;
      if (idempotencyKey && idempotency.has(idempotencyKey)) {
        sendJson(res, 200, {
          status: "success",
          data: idempotency.get(idempotencyKey),
        });
        return;
      }

      const booking = bookingPayload({
        start: requestedStart,
        attendees: [
          {
            id: 321,
            name: body.attendee?.name ?? "Fixture Attendee",
            email: body.attendee?.email ?? "fixture-attendee@example.test",
            timeZone: body.attendee?.timeZone ?? "America/Chicago",
          },
        ],
        responses: body.responses ?? {},
        metadata: body.metadata ?? {},
      });
      bookings.set(booking.uid, booking);
      if (idempotencyKey) idempotency.set(idempotencyKey, booking);

      sendJson(res, 201, {
        status: "success",
        data: booking,
      });
      return;
    }

    const bookingByUid = path.match(/^\/v2\/bookings\/([^/]+)$/);
    if (req.method === "GET" && bookingByUid) {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 401, { status: "error", error: { code: "UNAUTHORIZED", requestId: "mock-request-id" } });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, {
          status: "error",
          error: { code: "FORBIDDEN", message: "Insufficient permissions", requestId: "mock-request-id" },
        });
        return;
      }

      const uid = decodeURIComponent(bookingByUid[1]);
      const booking = uid === "mock-booking-personal-basic" ? ensureBooking() : bookings.get(uid);
      if (!booking) {
        sendJson(res, 404, {
          status: "error",
          error: { code: "NOT_FOUND", message: "Booking not found", requestId: "mock-request-id" },
        });
        return;
      }

      sendJson(res, 200, { status: "success", data: booking });
      return;
    }

    const bookingCancel = path.match(/^\/v2\/bookings\/([^/]+)\/cancel$/);
    if (req.method === "POST" && bookingCancel) {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }

      await readJsonBody(req);
      const uid = decodeURIComponent(bookingCancel[1]);
      const existing = uid === "mock-booking-personal-basic" ? ensureBooking() : bookings.get(uid);
      if (!existing) {
        sendJson(res, 404, { status: "error", error: { code: "NOT_FOUND", requestId: "mock-request-id" } });
        return;
      }

      const cancelled = bookingPayload({
        ...existing,
        status: "cancelled",
        updatedAt: "2026-01-01T00:05:00.000Z",
      });
      bookings.set(uid, cancelled);
      sendJson(res, 200, {
        status: "success",
        data: cancelled,
        sideEffects: ["calendar.cancelled", "email.cancelled", "webhook.booking.cancelled"],
      });
      return;
    }

    const bookingReschedule = path.match(/^\/v2\/bookings\/([^/]+)\/reschedule$/);
    if (req.method === "POST" && bookingReschedule) {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }

      const body = await readJsonBody(req);
      const oldUid = decodeURIComponent(bookingReschedule[1]);
      const existing = oldUid === "mock-booking-personal-basic" ? ensureBooking() : bookings.get(oldUid);
      if (!existing) {
        sendJson(res, 404, { status: "error", error: { code: "NOT_FOUND", requestId: "mock-request-id" } });
        return;
      }

      const oldBooking = bookingPayload({
        ...existing,
        status: "cancelled",
        updatedAt: "2026-01-01T00:10:00.000Z",
      });
      const newBooking = bookingPayload({
        ...existing,
        uid: "mock-booking-rescheduled",
        status: "accepted",
        start: body.start ?? "2026-05-02T15:00:00.000Z",
        end: "2026-05-02T15:30:00.000Z",
        updatedAt: "2026-01-01T00:10:00.000Z",
      });
      bookings.set(oldUid, oldBooking);
      bookings.set(newBooking.uid, newBooking);
      sendJson(res, 200, {
        status: "success",
        data: {
          oldBooking,
          newBooking,
          oldBookingUid: oldBooking.uid,
          newBookingUid: newBooking.uid,
        },
        sideEffects: ["calendar.rescheduled", "email.rescheduled", "webhook.booking.rescheduled"],
      });
      return;
    }

    const bookingConfirm = path.match(/^\/v2\/bookings\/([^/]+)\/confirm$/);
    if (req.method === "POST" && bookingConfirm) {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }

      await readJsonBody(req);
      const uid = decodeURIComponent(bookingConfirm[1]);
      const existing = uid === "mock-booking-pending-confirm" ? ensurePendingBooking(uid) : bookings.get(uid);
      if (!existing) {
        sendJson(res, 404, { status: "error", error: { code: "NOT_FOUND", requestId: "mock-request-id" } });
        return;
      }

      const confirmed = bookingPayload({
        ...existing,
        status: "accepted",
        updatedAt: "2026-01-01T00:15:00.000Z",
      });
      bookings.set(uid, confirmed);
      sendJson(res, 200, {
        status: "success",
        data: confirmed,
        sideEffects: ["email.confirmed", "webhook.booking.confirmed"],
      });
      return;
    }

    const bookingDecline = path.match(/^\/v2\/bookings\/([^/]+)\/decline$/);
    if (req.method === "POST" && bookingDecline) {
      if (!authenticatedBookingPrincipal(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }
      if (!ownsFixtureBooking(req)) {
        sendJson(res, 403, { status: "error", error: { code: "FORBIDDEN", requestId: "mock-request-id" } });
        return;
      }

      await readJsonBody(req);
      const uid = decodeURIComponent(bookingDecline[1]);
      const existing = uid === "mock-booking-pending-decline" ? ensurePendingBooking(uid) : bookings.get(uid);
      if (!existing) {
        sendJson(res, 404, { status: "error", error: { code: "NOT_FOUND", requestId: "mock-request-id" } });
        return;
      }

      const declined = bookingPayload({
        ...existing,
        status: "rejected",
        updatedAt: "2026-01-01T00:20:00.000Z",
      });
      bookings.set(uid, declined);
      sendJson(res, 200, {
        status: "success",
        data: declined,
        sideEffects: ["email.declined", "webhook.booking.declined"],
      });
      return;
    }

    const oauthClientMetadata = path.match(/^\/v2\/auth\/oauth2\/clients\/([^/]+)$/);
    if (req.method === "GET" && oauthClientMetadata) {
      if (bearerToken(req) !== "cal_test_valid_mock") {
        sendJson(res, 401, { status: "error", error: { code: "UNAUTHORIZED" } });
        return;
      }

      const clientId = decodeURIComponent(oauthClientMetadata[1]);
      if (clientId !== "mock-oauth-client") {
        sendJson(res, 404, {
          status: "error",
          error: {
            code: "NOT_FOUND",
            message: "OAuth client not found",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      sendJson(res, 200, {
        status: "success",
        data: {
          clientId,
          name: "Fixture OAuth Client",
          redirectUris: ["https://fixture.example.test/callback"],
          createdAt: "2026-01-01T00:00:00.000Z",
          updatedAt: "2026-01-01T00:00:00.000Z",
          requestId: "mock-request-id",
        },
      });
      return;
    }

    const platformClient = path.match(/^\/v2\/oauth-clients\/([^/]+)$/);
    if (req.method === "GET" && platformClient) {
      const clientId = decodeURIComponent(platformClient[1]);
      const headerClientId = req.headers["x-cal-client-id"];
      const secret = req.headers["x-cal-secret-key"];
      if (
        clientId !== "mock-platform-client" ||
        headerClientId !== "mock-platform-client" ||
        secret !== "mock-platform-secret"
      ) {
        sendJson(res, 401, {
          status: "error",
          error: {
            code: "UNAUTHORIZED",
            message: "Invalid platform client credentials",
            requestId: "mock-request-id",
          },
        });
        return;
      }

      sendJson(res, 200, {
        status: "success",
        data: {
          id: "mock-platform-client",
          name: "Fixture Platform Client",
          organizationId: 456,
          permissions: ["booking:read", "booking:write"],
          createdAt: "2026-01-01T00:00:00.000Z",
          updatedAt: "2026-01-01T00:00:00.000Z",
          requestId: "mock-request-id",
        },
      });
      return;
    }

    sendJson(res, 404, {
      status: "error",
      error: {
        code: "NOT_FOUND",
        message: `No mock route for ${req.method} ${path}`,
        requestId: "mock-request-id",
      },
    });
  });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const port = Number(process.env.PORT ?? "5555");
  const server = createMockApiV2Server();
  server.listen(port, () => {
    console.log(`Mock API v2 server listening on http://127.0.0.1:${port}`);
  });
}
