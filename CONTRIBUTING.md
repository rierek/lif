# Contributing to Lif

## Where to go

- **Bug or concrete feature request:** open an [issue](https://github.com/xRiErOS/lif/issues). Include observed behavior, expected behavior, `lif version`, and the smallest store shape that reproduces it.
- **Questions, ideas, and projects built around Lif:** use [Discussions](https://github.com/xRiErOS/lif/discussions).
- **Code or docs change:** open a pull request. For anything larger than a small fix, start with an issue or discussion so the direction is agreed before you build it.

## Branches

`main` only carries released states. Work lands on the open release line first and reaches `main` with the next release.

- Target your pull request at the current `milestone/0.N` branch (the highest one), not at `main`.
- Branch from that line, or from `main` if you already have; the maintainer can retarget the pull request.
- Pull requests against `main` are retargeted before merging.

## Before you open a pull request

- One concern per pull request.
- `just test` passes; `just test-web` and `just test-e2e` too if you touched `frontend/`.
- Behavior changes come with a test that fails without them.
- User-visible changes update `README.md` or the matching page under `wiki/`.
- Commit titles follow Conventional Commits: `<type>(<scope>): <subject>`, for example `fix(cli): reject empty titles`.

CI runs on every pull request. For first-time contributors GitHub holds it until the maintainer approves the run.

## License

Lif is licensed under Apache-2.0. By submitting a contribution you agree that it is licensed under the same terms.
