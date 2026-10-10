---
name: maintain-npm-package-page
description: Create or refresh a project-owned npm package README and npmjs.com landing page for a CLI package. Use when install commands, npx/pnpm/Yarn/Bun usage, executable names, architecture, modes, platform support, security boundaries, or registry links need clearer documentation.
---

# Maintain an npm package page

Start from the optional [package-page template](../../assets/package-page.md)
only when it saves time. Copy useful pieces into the package's own README; the
project immediately owns the result and may depart from the template.

## Write for a first successful command

1. State the package's purpose in one sentence.
2. Show the canonical global install and executable command.
3. Show a no-install invocation with `npx` when supported.
4. Add the shortest real usage example before architecture detail.
5. Explain architecture, modes, platform limits, privilege requirements,
   downloads, install scripts, and security boundaries only as far as users
   need them.

Distinguish package name from executable name. Add alternatives only after
testing them against the published package:

- npm: `npm install --global`, `npx`, or `npm exec`;
- pnpm: `pnpm add --global`, `pnpm dlx`, or `pnpx`;
- Yarn: `yarn dlx`;
- Bun: `bun add --global` or `bunx`;

Do not imply that every JavaScript runtime supports Node APIs, lifecycle
scripts, native binaries, or the same cache/permission model. Keep only commands
that execute the package's real CLI successfully.

## Validate ownership and rendering

Confirm `package.json` points npm at the intended README and that package
metadata, links, binary names, version examples, and platform claims are
current. Render or inspect the packed README where practical. Lasso does not
generate, synchronize, lint, publish, or retain state for this page.
