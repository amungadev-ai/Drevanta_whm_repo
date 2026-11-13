# Drevanta theme consistency — v1.9

- Original Drevanta teal/slate is the first-visit default across the entire build.
- Onboarding and authentication now use the same `drevanta-palette` and `hc-theme` preferences as all four workspaces.
- The default light mode is consistent across workspaces; the existing dark mode remains available.
- Changing the palette or light/dark mode in onboarding carries into dashboards, and vice versa (same browser origin).
- Existing alternate palettes remain available; no dashboard layouts, navigation, or flows have been redesigned.
- For a clean first-visit check, use a fresh private window or clear prior Drevanta localStorage preferences.
- Browser file:// origins may isolate localStorage differently. Use a local HTTP server to test cross-page persistence.
- This remains a front-end simulation.
