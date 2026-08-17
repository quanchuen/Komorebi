# ADR 0007: Support foreground navigation as an installable web app

- Status: Accepted
- Date: 2026-08-04

## Context

An iPhone Home Screen web app can provide a focused, standalone cycling UI
without an App Store release. Web APIs can support foreground geolocation, map
updates, route progress, off-route detection, rerouting, and spoken cues. A web
manifest and service worker can provide installation metadata and cache the
application shell.

Reliable turn-by-turn cycling navigation also needs timely location updates and
cues while the screen is locked or the application is backgrounded. Apple notes
that iOS normally suspends background applications and documents continuous
background location as a native Core Location application capability. See
[Apple's background location documentation](https://developer.apple.com/documentation/corelocation/handling-location-updates-in-the-background).

## Decision

Deliver the first navigation milestone as an installable, foreground-only web
app:

- add a web app manifest, install icons, standalone presentation, and an app-shell
  service worker;
- request geolocation only after a user starts navigation;
- use `watchPosition` while the page is visible;
- show route progress, next instruction, GPS accuracy, off-route state, and a
  clear reroute action;
- optionally use speech synthesis for cues while the app is active;
- keep the planned route and essential instructions available through transient
  network loss;
- detect visibility changes and clearly warn when guidance may have paused.

Do not promise reliable background or locked-screen tracking, continuous spoken
cues, or ride recording from the web milestone. Web push is for visible remote
notifications, not invisible continuous navigation work; Apple requires received
web pushes to be presented. See [Apple's Web Push documentation](https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers).

If validated product requirements include locked-screen navigation, reliable
background rerouting, or continuous ride recording, build a native iOS shell or
application using Core Location and record that distribution decision separately.

## Consequences

- The project can validate navigation UX using the existing SvelteKit/MapLibre
  application.
- Foreground navigation still requires HTTPS, permission handling, battery-aware
  sampling, real-device tests, and safe failure behavior.
- Service workers improve application/offline asset availability but do not make
  foreground JavaScript a background navigation service.
- The UI must never imply guidance continues after iOS suspends it.
- Native iOS remains a deliberate later step rather than an accidental promise.

