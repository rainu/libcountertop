# Contributing: settings completeness

Every supported user-configurable setting and associated configuration/lifecycle
action must have appropriate device UI, usable owner web UI and a documented
owner-authenticated API in the consuming application. Share validation,
canonical catalogs, build/hardware availability and persisted state. Verify
refresh/restart behavior, localization, mobile layout, invalid input and
unauthorized access. Keep credential reads/exports protected and never broaden
restricted Node authority to implement an owner configuration flow.

Update maintained coverage inventories and omission-detecting regression tests
when adding fields/actions. For EchoLocal, see
[the owner configuration coverage](https://github.com/rainu/echolocal/blob/rainu/integration/docs/openclaw-owner-settings.md).
Experimental Avatar Lab is the only narrow temporary device/CLI-only exception;
reassess web/API coverage before its configuration becomes a supported feature.

For web changes, run `npm --prefix web ci`, `npm --prefix web run build`,
`npm --prefix web test`, install the matching Playwright Chromium, and run
`npm --prefix web run test:e2e`. Commit rebuilt embedded assets. The CI web job
checks for stale generated frontend files. Synthetic browser tests do not prove
real-device browser or physical interaction.
