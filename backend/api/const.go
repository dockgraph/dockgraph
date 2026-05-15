package api

// fieldName is the JSON response field key for resource identifiers,
// reused across containers, networks, and volumes inspect payloads.
const fieldName = "name"

// stateRunning is the Docker container state value indicating the
// container is currently running. Doubles as the JSON field key in
// inspect responses where the field is named after the state.
const stateRunning = "running"

// untaggedImageTag is Docker's marker for images without a real tag,
// used to identify dangling/reclaimable images.
const untaggedImageTag = "<none>:<none>"

// indexHTMLPath is the SPA fallback served when no static file matches the URL.
const indexHTMLPath = "index.html"
