# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Field and production-line technicians. They are technical but not software developers. They work on a laptop connected to an isolated device network, often on a factory floor or at a customer site, usually without internet access. Their job is to push a known set of files (firmware parts, configs, scripts) to tens or hundreds of embedded Linux devices, then prove the result.

## Product Purpose

DeviceFileUpdater copies chosen local files to chosen paths on embedded Linux devices over SSH or Telnet. It chooses the protocol and upload method per device by itself (SFTP, SCP, FTP, or shell fallbacks). Identical files are left alone, different files are updated, and missing files are created. Every run produces a report.

Success means the operator finishes a fleet update without touching a terminal, sees exactly which device and file failed and why, and can retry only the failures.

## Positioning

It works on devices that have none of the usual tools. When there is no SCP, SFTP, FTP, base64 or even a hash tool, it falls back to shell uploads. It auto-detects all of this per device. A mandatory dry-run preview shows what will change before anything is written.

## Operating Context

- Runs as a single binary on Windows, Linux or macOS. The UI is served on `127.0.0.1` with a random port and an access token, and opens in the default browser.
- Everything lives in a workspace folder:
  - `devices.csv` (ip, username, password)
  - `manifest.csv` (local_path, remote_path, mode)
  - `settings.json`
  - `files/`
  - `reports/`
- Operators often edit CSVs in Excel. A Turkish locale produces `;`-separated files with a BOM, and the engine accepts both.
- A CLI (`devupdater run`) uses the same engine for automation.

## Capabilities and Constraints

- UI language: Turkish. Technical terms stay verbatim (SSH, Telnet, SFTP, SCP, FTP, chmod, dry-run, post-command).
- Fully offline. No CDN, no remote fonts. All assets are embedded in the binary.
- A step-by-step wizard:
  1. Workspace
  2. Devices
  3. Files
  4. Settings
  5. Dry-run preview (mandatory)
  6. Apply (live)
  7. Report
  - Report history is available from the menu.
- Only one run at a time. It can be cancelled, and cancelled devices are reported as FAILED (cancelled).
- Passwords never appear in reports, logs or console output, and are masked in the UI.
- No sudo, no pulling files from devices, no deleting files on devices, and the same manifest for every device.

## Brand Commitments

The product name is "DeviceFileUpdater", and the binary is `devupdater`.

Visual direction (user choice, 2026-09-30): the category standard, played straight. That means a clean shadcn/ui operator dashboard with no novelty world. The craft bar is the Vercel dashboard: restrained monochrome, crisp status indicators, and a legible run/progress log.

## Evidence on Hand

No screenshots, logos or customer material exist. Do not fabricate any.

## Product Principles

1. Never surprise the operator: show what will change before writing, and show exactly what happened afterwards.
2. One device's failure never blocks the others or hides in a summary.
3. Guide, don't gatekeep: each step validates before moving on, but the operator can always go back.
4. Works in the worst environment: offline, flaky networks, minimal devices.

## Accessibility & Inclusion

Must be readable on low-quality laptop screens and in bright factory lighting. Use strong contrast. Never convey status by color alone; pair it with text or an icon. Keyboard operable.
