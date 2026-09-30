---
name: DeviceFileUpdater
description: Offline operator console for pushing one file manifest to a fleet of embedded Linux devices.
colors:
  primary: "#2563eb"
  primary-dark: "#3b82f6"
  on-primary: "#ffffff"
  surface: "#fafafa"
  surface-dark: "#09090b"
  card: "#ffffff"
  card-dark: "#111113"
  ink: "#09090b"
  ink-dark: "#fafafa"
  muted: "#f4f4f5"
  muted-dark: "#1c1c1f"
  muted-ink: "#52525b"
  muted-ink-dark: "#a1a1aa"
  hairline: "#e4e4e7"
  hairline-dark: "#27272a"
  field-stroke: "#d4d4d8"
  field-stroke-dark: "#3f3f46"
  status-ok: "#047857"
  status-ok-dark: "#34d399"
  status-warn: "#b45309"
  status-warn-dark: "#fbbf24"
  status-fail: "#dc2626"
  status-fail-dark: "#f87171"
  status-same: "#52525b"
  status-same-dark: "#a1a1aa"
typography:
  headline:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: 1.33
    letterSpacing: "-0.025em"
  title:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 500
    lineHeight: 1.5
  body:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
    fontFeature: "\"tnum\" 1"
  label:
    fontFamily: "Geist Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.33
  mono:
    fontFamily: "Geist Mono Variable, ui-monospace, SFMono-Regular, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
rounded:
  md: "6px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.md}"
    height: "32px"
    padding: "0 10px"
  button-outline:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    height: "32px"
    padding: "0 10px"
  input:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    height: "32px"
    padding: "4px 10px"
  card:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.md}"
    padding: "16px"
  top-bar:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    height: "56px"
  status-badge:
    backgroundColor: "{colors.card}"
    textColor: "{colors.status-ok}"
    rounded: "{rounded.md}"
    typography: "{typography.label}"
    padding: "2px 8px"
---

# Design System: DeviceFileUpdater

## Overview

**Creative North Star: "The Calibrated Instrument"**

DeviceFileUpdater is a bench instrument, not a dashboard. A technician on a production line opens it on a laptop under bright factory light, walks a seven-step wizard, and must leave with proof of what was written to which device. The interface is the category standard played straight: a restrained shadcn/ui operator console at Vercel-dashboard craft, zinc neutrals, one blue accent, hairline borders, and nothing that competes with the data.

Density is medium. Tables and matrices carry the weight; chrome stays thin. Every screen answers three questions without scrolling: which step am I on, what blocks the next step, and what will happen to which device. Brand lives in precision: tabular numerals everywhere, mono for every machine value, in-place updates with no layout jump.

The system rejects novelty worlds and the generic admin template (sidebar, stat-card mosaics, colour on every surface). It works fully offline, so it ships its own fonts and loads nothing remote.

**Key Characteristics:**
- Zinc surfaces, hairline borders, one accent reserved for the primary action and focus.
- Status always travels as colour + Phosphor icon + Turkish text label, never colour alone.
- Geist for interface, Geist Mono for IPs, paths, file modes and hashes.
- One 6px radius on every shape; no pills.
- Light by default, dark via `prefers-color-scheme`.

## Colors

A cool zinc greyscale with a single saturated blue and four muted status hues. Every light token has a dark twin (the `-dark` keys) applied under `.dark`.

### Primary
- **Signal Blue** (`primary`, dark `primary-dark`): the one "go" colour. Used for the single primary button of a view, focus rings, the current stepper step, progress bars, and inline links. Never for decoration or headings.

### Neutral
- **Bench White / Bench Black** (`surface`, `surface-dark`): page background behind every column.
- **Panel** (`card`, `card-dark`): tables, cards, dialogs, the report summary grid. One step lifted from the surface by tone, not by shadow.
- **Ink** (`ink`, `ink-dark`): body text and headings.
- **Quiet Ink** (`muted-ink`, `muted-ink-dark`): purpose lines, table headers, metadata, placeholders. Holds 4.5:1 on both surface and card.
- **Hairline** (`hairline`, `hairline-dark`): table dividers, card and bar borders.
- **Field Stroke** (`field-stroke`, `field-stroke-dark`): input borders, one step darker than the hairline so fields read as editable.
- **Wash** (`muted`, `muted-dark`): hover rows, skeletons, progress tracks, secondary fills.

### Status
- **Written Green** (`status-ok`): device succeeded, file created or updated, connection OK.
- **Pending Amber** (`status-warn`): something will change (would create, would update), or a soft blocker in the action bar.
- **Fault Red** (`status-fail`): device or file failed, unreachable, destructive confirmation.
- **Unchanged Zinc** (`status-same`): file already identical; deliberately quiet.

### Named Rules
**The One Voice Rule.** Signal Blue appears on at most one filled button per view. When a page body already owns the primary action (for example "Başarısızları tekrar dene" on a report with failures), the action bar's forward button drops to outline.

**The Never Colour Alone Rule.** No status is expressed by hue only. Every status carries its icon (CheckCircle, XCircle, WarningCircle, CircleNotch, Circle) and a Turkish label, and failure counts spell out "başarısız".

## Typography

**Display Font:** none; the system has no display tier.
**Body Font:** Geist Variable (with ui-sans-serif, system-ui)
**Label/Mono Font:** Geist Mono Variable (with ui-monospace, SFMono-Regular)

**Character:** A neutral grotesque paired with its own mono, so machine values sit in the same rhythm as prose. Tabular numerals are on globally (`font-feature-settings: "tnum" 1`), so counters never jitter while a run is live.

### Hierarchy
- **Headline** (600, 1.5rem, tight tracking): one per page, the step title ("Cihazlar", "Önizleme", "Rapor").
- **Title** (500, 1rem): result sentences and section heads inside a step.
- **Body** (400, 0.875rem): everything operational, including table cells, help text and dialogs.
- **Label** (500, 0.75rem): badges, counters inside filter tabs, and stepper numerals.
- **Mono** (400, 0.875rem; 0.75rem in dense cells): IP:port, remote paths, octal modes, report IDs, tool lists.

### Named Rules
**The Machine Values Are Mono Rule.** Anything a technician might copy into a terminal (host, path, mode, hash, report ID) is set in Geist Mono. Human language never is.

**The Turkish Copy Rule.** All UI copy is Turkish, uses no em or en dash, and writes examples as "örn. …" so placeholders never read as entered values.

## Layout

A single work column, `max-width: 72rem`, centred with 24px side padding and 32px top padding. There is no sidebar. The frame is fixed:

1. **Top bar** (56px, panel fill, bottom hairline): product name, current workspace path in mono (truncates to "/.." on narrow screens), and "Geçmiş raporlar".
2. **Stepper** (panel fill, bottom hairline): seven horizontal steps (Çalışma alanı, Cihazlar, Dosyalar, Ayarlar, Önizleme, Uygula, Rapor), each done, current, blocked (lock icon) or reachable. Below 768px it collapses to "Adım N/7: <name>".
3. **Work column**: headline, one-line purpose, then the step's content.
4. **Sticky action bar**: "Geri" on the left; the blocking reason as text plus the primary button on the right.

Spacing runs on a 4px base: 8px inside controls, 16px between related blocks, 24px between sections, 32px page top. Grids that hold wide content are explicitly single-column (`grid-cols-1`) so long error text or matrices can never widen the page on mobile. Wide tables scroll inside their own container, never the page. The Devices table has fixed column widths, so a connection test result never reflows the columns.

## Elevation & Depth

Flat. Depth comes from tone (surface under panel) and hairline borders, not shadows. The only layered surfaces are dialogs and popovers from shadcn, which keep their stock overlay, and the sticky action bar, which uses a 95% surface with backdrop blur and a top hairline so content visibly passes beneath it.

### Named Rules
**The Hairline Not Shadow Rule.** Separate with a 1px hairline or a tonal step. Do not add drop shadows to cards, tables or rows.

## Shapes

One radius (6px) on every shape: buttons, inputs, cards, badges, progress bars and tracks, dialogs. The Tailwind radius scale from sm to 4xl is aliased to this single value, so no utility can produce a pill or a sharp corner by accident. Borders are 1px throughout.

## Components

### Buttons
Quiet and exact; the press is felt, not seen.
- **Shape:** gently squared (6px), 32px tall with 10px side padding; the small size is 28px.
- **Primary:** Signal Blue fill, white text, trailing ArrowRight when it advances the wizard.
- **Outline:** panel fill, field-stroke border, ink text. Used for Geri, secondary toolbar actions ("Cihaz ekle", "Toplu ekle", "CSV dışa aktar"), and the demoted forward action under the One Voice Rule.
- **Press:** `scale(0.98)` over 120ms `cubic-bezier(0.23, 1, 0.32, 1)`; background and colour ease over 150ms. Disabled buttons do not animate. Reduced motion removes both.
- **Focus:** Signal Blue ring.

### Status Badges
- **Style:** 6px radius, a 10% wash of the status hue with a 25% tinted border and status-coloured text, a bold 14px icon plus a Turkish label ("Oluşturulacak", "Güncellenecek", "Aynı", "Başarısız").
- **Use:** preview matrix cells and report file rows.

### Cards / Containers
- **Corner Style:** 6px.
- **Background:** panel on surface.
- **Shadow Strategy:** none (see Elevation & Depth).
- **Border:** 1px hairline.
- **Internal Padding:** 16px; 24px for empty states.

### Inputs / Fields
- **Style:** 32px tall, 6px radius, field-stroke border, transparent fill over the panel; mono for IP and path fields.
- **Focus:** Signal Blue border plus a 3px half-strength ring.
- **Error:** Fault Red border and a one-line reason under the field; the action bar names the blocker.
- **Password:** masked, with an eye toggle; values never leave the device row.

### Navigation
- **Stepper:** numbered squares (6px). Done shows a check on ink, current is Signal Blue with an underline, blocked shows a lock in quiet ink. On mobile it becomes "Adım N/7".
- **Top bar link:** quiet ink with a ClockCounterClockwise icon, ink on hover.

### Live Apply Grid (signature)
A fixed-layout table (host, stage, files, detail) with 44px rows that update in place as events stream in. No row ever changes height or position. The stage cell crossfades opacity over 150ms on each change. The file counter is mono `done/total` with a 4px progress bar. A failed device counts only its succeeded files, so it shows "0/2" with a red bar, never "2/2". The detail cell truncates with the full text in its title. A "Başarısız" filter tab carries the count.

### Preview Matrix
Device × file grid. Each cell holds a status badge; an unreachable device spans the row with its error in Fault Red. It ends in an explicit confirmation checkbox that restates what will happen ("3 cihazda toplam 6 dosya yazılacak").

## Do's and Don'ts

### Do:
- **Do** keep a single filled Signal Blue button per view; demote the other to outline.
- **Do** pair every status with its Phosphor icon and Turkish label.
- **Do** set hosts, paths, modes, hashes and report IDs in Geist Mono.
- **Do** use `grid-cols-1` on any grid that can hold long unbroken text, and scroll wide tables inside their own container.
- **Do** keep motion to transform, opacity or colour, at 120-200ms, on `cubic-bezier(0.23, 1, 0.32, 1)`, and remove it under `prefers-reduced-motion`.
- **Do** use Phosphor icons only.

### Don't:
- **Don't** use pill shapes (`rounded-full`) or any radius other than 6px.
- **Don't** use `transition: all` or animate layout properties.
- **Don't** add a sidebar, stat-card mosaic, gradients, or drop shadows on cards.
- **Don't** use em or en dashes in UI copy, or show example text that looks like an entered value.
- **Don't** convey a status by colour alone.
- **Don't** load fonts, icons or scripts from the network; the tool runs offline.
