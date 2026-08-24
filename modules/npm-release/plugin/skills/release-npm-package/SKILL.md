---
name: release-npm-package
description: Release or configure an npm package with native npm and GitHub tooling. Use for first publication, versioned npm releases, npm Trusted Publisher setup, OIDC workflow review, tarball inspection, provenance, token retirement, or fresh npm/npx/pnpm/Yarn/Bun/Deno acceptance.
---

# Release an npm package

Keep the owning project in control. Use its versioning, build, test, changelog,
artifact, GitHub Release, and acceptance commands. The bundled helper exists
only for the one-time npm Trusted Publisher relationship.

## Establish the release contract

1. Read the project guidance, `package.json`, lockfile, publishing workflow,
   changelog, and release scripts.
2. Identify the package name, executable names, version source of truth,
   supported runtimes/platforms, public files, and whether installation runs
   lifecycle scripts or downloads external binaries.
3. Preserve the project's artifact boundary. If it builds release archives
   locally, publish those exact checksummed archives; do not silently rebuild
   different bytes in CI.
4. Treat version publication as immutable. Verify that the target version is
   absent before uploading and that source, tag, package metadata, and release
   notes agree.

## Configure trusted publishing

npm requires an existing package before its Trusted Publisher can be attached.
For a first release, use the account's ordinary interactive npm flow, publish
only the bootstrap version, then retire that credential.

Run the bundled [setup helper](../../scripts/setup-trusted-publisher) with
`--print` first. Show the exact repository, workflow, package, and allowed
publish action. Apply it only after the user authorizes the account-level
change. npm owns login, browser state, 2FA, and the stored trust relationship.

The project workflow must:

- run on a GitHub-hosted runner supported by npm Trusted Publishing;
- grant `id-token: write` only to the publish job;
- check out or download the immutable release input;
- pin npm 12.x (or a later project-approved version) instead of relying on the
  runner's bundled npm; and
- invoke native `npm publish` directly without a long-lived token.

npm Trusted Publishing automatically emits provenance. Do not add a permanent
`NODE_AUTH_TOKEN`, npm token, browser cookie, or Lasso command to the routine
workflow. A package has one trusted publisher, so inspect and explicitly remove
an obsolete binding before replacement.

## Package and publish

Run the project's full gate, then inspect the package rather than trusting a
successful build:

```bash
npm pack --dry-run
npm pack --json
```

Check included files, executable mappings, bundled native artifacts, archive
size, lifecycle scripts, license, README, repository links, and version. Publish
with the project's native command or release workflow. Never dispatch or
approve a release merely because setup succeeded.

If bootstrap credentials were used, remove the local login and revoke the
server-side token separately. A local logout does not prove server revocation.

## Accept from the registry

Wait for the exact version to appear, then query npm independently and install
into a fresh temporary prefix or cache. Exercise the real executable, not only
`npm view`:

```bash
npm view PACKAGE@VERSION name version dist.integrity dist.tarball
npx --yes --package=PACKAGE@VERSION -- COMMAND --version
```

When claimed by the project, also smoke-test pnpm, Yarn, Bun, or Deno through
their native one-shot command. Verify the registry tarball checksum/integrity
against the released artifact where the project promises byte identity. Report
build, publication, registry visibility, fresh installation, and execution as
separate outcomes.
