# Native iOS application instructions

These instructions apply to the planned native iOS application under `ios/`.
The directory currently defines the boundary; do not scaffold an Xcode project
unless the task explicitly requests native implementation.

## Product boundary

- Share the Go HTTP API and domain vocabulary with the web application; do not
  duplicate routing policy in Swift.
- Use native iOS only for capabilities that materially benefit from it, including
  reliable background location, locked-screen guidance, continuous ride
  recording, and platform navigation integration.
- Keep the foreground web app as a supported client. Native work must not make
  server contracts iOS-specific.

## Stack

- Prefer SwiftUI for application UI and Core Location for navigation tracking.
- Choose MapLibre Native or MapKit behind a map adapter; record a binding choice
  that affects licensing or server data in an ADR.
- Use structured concurrency and isolate mutable navigation state. UI updates
  belong on the main actor.

## Location, privacy, and safety

- Request the least powerful location authorization that satisfies the active
  feature and explain the benefit before the system prompt.
- Background location, ride recording, and exploration history require explicit
  start/stop controls, visible state, retention, export, and deletion behavior.
- Never log precise coordinates, route history, authorization tokens, or imported
  tracks in production diagnostics.
- Navigation UI must acknowledge GPS accuracy and stale fixes and must not imply
  a route is legally or physically safe.

## Testing

- Keep pure route-progress and navigation-state logic independent of Core
  Location so it can be unit tested deterministically.
- Use injected location streams and clocks; never make tests depend on live GPS,
  wall-clock sleeps, or a particular simulator location.
- UI tests use accessibility identifiers/names and follow the same 44x44-point
  minimum interaction target as the web layout policy.

