// unstorage (pulled in by Astro) declares an optional peer for every storage
// backend it can talk to. This site uses none of them, so keep those
// never-installed backends out of the dependency graph and the lockfile.
function readPackage(pkg) {
  if (pkg.name === "unstorage") {
    pkg.peerDependencies = {};
    pkg.peerDependenciesMeta = {};
  }
  return pkg;
}

module.exports = { hooks: { readPackage } };
