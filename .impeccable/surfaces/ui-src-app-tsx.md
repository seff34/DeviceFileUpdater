---
version: 1
slug: "ui-src-app-tsx"
primary_target: "ui/src/App.tsx"
related_targets: []
---

# Operator wizard (devupdater ui)

Scope: the whole embedded web UI, which is the 7-step wizard plus report history. Visitor mode: Operate.
Audience and job: a field or production-line technician pushes one manifest of files to a device fleet. They must validate every step, preview the dry-run, apply it, and prove the result. The UI is in Turkish and works fully offline. It is used on laptops under bright factory light.

## Direction contract

THESIS: this is the category standard played straight, which the user chose as canon. It is a restrained shadcn/ui operator console at Vercel-dashboard craft. It refuses novelty worlds, and it refuses the generic admin template with a sidebar, stat cards and colour everywhere.

OWN-WORLD:
- Palette: off-white zinc surface, zinc-950 ink, hairline zinc-200 borders, and one accent (blue-600) reserved for the primary action and focus. Status colours are emerald, amber, red and zinc. Status is always paired with a text label and a Phosphor icon.
- Type: Geist for UI and Geist Mono for IPs, paths, modes and hashes.
- Shape: 6px radius on everything; no pills.
- Theme: light by default and dark via prefers-color-scheme.

STORY:
- The operator always knows three things: which step they are on, what blocks the next step, and what will happen to which device.
- Before anything is written, they see the device × file matrix and confirm it explicitly.

FIRST VIEWPORT:
- A 56px top bar: product name, workspace path in mono, and a "Geçmiş raporlar" link.
- Below it, a horizontal 7-step stepper. Each step is shown as done, current, blocked or reachable.
- Below that, one max-w-6xl work column with the step title and a one-line purpose.
- A sticky bottom action bar: Geri on the left, and on the right the blocking reason in text plus a primary "Devam" button.

FORM: canon, the category standard. Seed key 908cd16c.

Signature interaction: the live Apply grid. Rows update in place with no reflow jump, the stage text crossfades in 150ms, and a failed row expands its error inline.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
