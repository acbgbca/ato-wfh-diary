// Commitlint configuration — enforces Conventional Commits format.
// See https://conventionalcommits.org for the full specification.
//
// Enforced by the commit-lint GitHub Actions workflow on every PR.
// release-please reads these commits on main to determine version bumps:
//   fix:   → patch bump   (1.0.x)
//   feat:  → minor bump   (1.x.0)
//   feat!: → major bump   (x.0.0)  (or any type with BREAKING CHANGE in footer)
//
// Other allowed types (chore, docs, test, refactor, ci, style, perf, build)
// produce no version bump but are valid and encouraged.

export default {
  extends: ["@commitlint/config-conventional"],
};
