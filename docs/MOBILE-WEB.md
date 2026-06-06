# Mobile access via browser



Swoop desktop stays the full native peer (discovery, TCP push, chat). Phones

and tablets without an app reach the **same control plane** over HTTPS and use

alternate **data planes** where the browser cannot do raw TCP.



## Principles



- **One protocol, multiple data planes.** Control (`prepare-upload`, accept/

  decline, TOFU TLS) is shared. Native desktop peers keep parallel TCP push;

  browser clients use HTTP upload (phase 1) or HTTP pull (phase 2).

- **Desktop logic unchanged** for desktop-to-desktop. Capability negotiation

  picks the path per peer.

- **LAN-only.** The phone opens `https://<desktop-ip>:<controlPort>/` on the

  same Wi‑Fi. User confirms the TLS fingerprint (TOFU) once.

- **1:1 pairing.** A browser tab is tied to exactly one desktop host (QR or

  manual URL). There is no shared device pool or cloud relay. The phone does

  not run a server; the desktop is always the HTTPS endpoint. This keeps

  zero-config behaviour predictable on home Wi‑Fi.



## Phase 1 — Phone → desktop (HTTP upload)



**Goal:** Send files from a mobile browser to a running Swoop desktop. No app

store, no native mobile build.



### Desktop



| Item | Detail |

|------|--------|

| Web UI | `GET /` — minimal page (pick files, send, progress) |

| Info | `GET /api/v1/info` — name, fingerprint, `capabilities` |

| Handshake | `POST /api/v1/prepare-upload` — same as native; blocks until Accept |

| Data | `POST /api/v1/upload/{sessionId}` + `X-Swoop-Token` — `multipart/form-data` |

| Native path | Unchanged: `mode=tcp-push`, `dataPort`, parallel TCP |



### Browser flow



1. Open `https://<desktop-ip>:53317/` (accept self-signed cert warning).

2. Page loads `/api/v1/info`, shows desktop name, address, short fingerprint.

3. Heartbeat `POST /api/v1/presence` — phone appears in the desktop grid
   (response includes an HMAC `token` for later API calls).

4. **Chat** (optional, same session): browser `POST /api/v1/message` with
   `platform: "web"` + `X-Swoop-Web-Token`; desktop replies via outbox polled at
   `GET /api/v1/chat?clientId&since`. Read receipts use `POST /api/v1/read` on
   both sides. The mobile page has a «Сообщения» card; the desktop shows chat
   when a web peer is selected.

5. User picks files → `prepare-upload` with `platform: "web"`.

6. Desktop shows the usual incoming-offer modal → Accept / Decline.

7. On 200, browser POSTs multipart upload to `uploadPath` from the response.

8. Desktop saves to Downloads; progress in Wails UI + web page.



### Done criteria



- [x] iOS Safari and Android Chrome can upload to Windows/macOS/Linux desktop.

- [x] Desktop-to-desktop speed and UX unchanged.

- [x] Incoming modal works for `platform: web`.



---



## Phase 2 — Desktop → phone (HTTP pull)



**Goal:** Send files from the desktop to an open browser tab without a native

mobile app.



### UX (who clicks what)



1. Phone opens the desktop URL (or QR) — **connection is established**, phone

   tile appears on the desktop grid.

2. On the **desktop**, user opens the phone tile, stages files/folders the usual

   way, and clicks **Отправить**.

3. The **phone** page polls the desktop and shows an incoming-offer card with

   the full file list (names, sizes, directory layout as `relPath`).

4. User taps **Принять и скачать** on the phone (or **Отклонить**).

5. Browser downloads via HTTP GET. **One loose file** downloads directly; **folders
   and multiple files** are packed into a temporary `.zip` on the desktop before
   the phone is answered (mobile browsers block several programmatic saves in a
   row). The zip is deleted when the session ends.



The list is visible on the phone **after** the desktop clicks Send (when the

offer is registered), not while merely staging on the desktop. This mirrors

phase 1: the initiator’s Send creates the offer; the receiver confirms.



### APIs



| Item | Detail |

|------|--------|

| Poll | `GET /api/v1/pull-offer?clientId=web-…` — 204 if none, else offer JSON |

| Respond | `POST /api/v1/pull-offer/{sessionId}/respond` — `{clientId, accept}` |

| Data | `GET /api/v1/download/{sessionId}/{fileId}` or `…/archive` (zip) + `X-Swoop-Token` |



### Desktop



- Web peers are **clickable** in the device grid (same staging / Send UI).

- `SendTo` uses HTTP pull instead of `postPrepare` (browser has no listener).

- Outgoing state: “Ожидание подтверждения на телефоне…”, then progress.



### Out of scope (phase 2 MVP)



- Resumable `Range` downloads.

- WebSocket live progress on the phone.

- Chat to browser clients.



### Done criteria



- [x] Desktop can push files to an open browser tab.

- [x] Phase 1 flows still work; native TCP push still default for desktop peers.



---



## Protocol sketch



```json

// GET /api/v1/info (desktop)

{

  "capabilities": ["tcp-push", "http-upload", "http-pull"]

}



// prepare-upload response — native sender

{ "mode": "tcp-push", "sessionId": "…", "dataPort": 53319, "token": "…" }



// prepare-upload response — web sender (phase 1)

{ "mode": "http-upload", "sessionId": "…", "uploadPath": "/api/v1/upload/…", "token": "…" }



// pull-offer poll (phase 2)

{

  "sessionId": "…",

  "sender": { "name": "MyPC", "platform": "windows", … },

  "files": [{ "id": "…", "name": "doc.pdf", "relPath": "project/doc.pdf", "size": 12345 }],

  "totalSize": 12345,

  "count": 1

}



// pull respond (accept) — phase 2

{

  "mode": "http-pull",

  "sessionId": "…",

  "token": "…",

  "files": [{ "id": "…", "downloadPath": "/api/v1/download/…/…" }]

}

```


