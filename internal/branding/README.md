# Runtime logos

`logo.png` and `logo_dark.png` are 320 × 320 runtime copies of the corresponding
originals in `docs/static/images/`, resized with bicubic interpolation. Keep those
originals for full-resolution use. The dark background is sidebar slate-900
(`#0f172a`); light uses white.

Vite imports these assets through `BrandLogo.vue`; Go embeds the same files for
self-contained Caddy error and parked-domain pages. When updating the source
logos, refresh these compact copies as well.
