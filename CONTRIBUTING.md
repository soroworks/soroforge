# Contributing to SoroForge

Thanks for considering a contribution. SoroForge is deliberately a small core
with clear extension points rather than a feature-complete tool, so most useful
contributions are additive and land in one place.

## Getting set up

You need Go 1.25+ (required by the Stellar SDK) and Docker for Postgres.

```bash
git clone https://github.com/soroworks/soroforge.git
cd soroforge
make build
make test
```

`make test` requires neither a network nor a database. If it needs either,
something has regressed — please report it.

**Example:** After cloning the repo, build and run the unit tests:

```bash
make build
make test
```

### Dev container (optional)

The repository ships a [dev container](.devcontainer/) with Go 1.26, Postgres
and golangci-lint already wired together. In VS Code, open the folder and
choose **Reopen in Container**; in GitHub Codespaces, create a codespace.
Then:

```bash
make check
make test-integration
```

`DATABASE_URL` and `TEST_DATABASE_URL` point at the container's Postgres, so
the integration suite runs without any other setup. A local Go install and
Docker compose still work exactly as described below — the dev container is a
convenience, not a requirement.

**Example:** Common dev container workflow — run checks and unit tests:

```bash
make check
make test
```

For the full suite:

```bash
docker compose up -d
make migrate-up
make test-integration   # adds the Postgres suite
make test-live          # read-only smoke tests against live testnet
```

Before opening a pull request:

```bash
make check    # fmt, vet, test
make lint     # golangci-lint at the version CI pins
```

## How the code is organised

```
cmd/soroforge/      cobra CLI
internal/config/    soroforge.yaml + env loading, validation, ScVal encoding
internal/stellar/   RPC client, transaction assembly, signing
internal/deploy/    deploy/upgrade/status orchestration
internal/store/     Postgres history + embedded migrations
internal/api/       chi HTTP handlers mirroring the CLI
```

**Example: adding a new constructor argument type**

1. Add a case to `Arg.ToScVal` in `internal/config/args.go`
2. Add the type name to `SupportedArgTypes` in the same file
3. Add a row to the table test in `internal/config/args_test.go`
4. No other packages need to change — the new type works through the existing `Arg` plumbing

Three interfaces are the seams the whole design rests on:

| Interface | Production | Test double |
|---|---|---|
| `stellar.Client` | `RPCClient` | `MockClient` |
| `stellar.Signer` | `KeypairSigner` | `MockSigner` |
| `store.Store` | `Postgres` | `MemoryStore` |

For the deployment flow, package boundaries, and invariants to preserve, see
the [architecture overview](ARCHITECTURE.md).

The test doubles live in the packages themselves rather than in `_test.go`
files, because other packages need them too. If you add a feature, you should be
able to test it without provisioning anything.

Two rules that keep this working:

- **Lifecycle logic belongs in `internal/deploy`.** The CLI and the HTTP API are
  both thin layers over the same `deploy.Service`. Putting behaviour in one of
  them means the two surfaces drift apart.
- **Only `internal/stellar/rpc.go` touches the Stellar SDK's RPC types.**
  Everything else works against the `stellar.Client` interface, which speaks
  base64 XDR strings. An SDK upgrade should be containable to that one file.

## Common contributions

### Adding a constructor argument type

Everything is in one switch. Add a case to `Arg.ToScVal` in
[`internal/config/args.go`](internal/config/args.go), add the name to
`SupportedArgTypes`, and add a row to the table test in `args_test.go`. Nothing
else needs to change — and note that `{type: xdr}` already means no type is ever
a hard blocker for a user.

### Adding a Signer

Implement `stellar.Signer` (`Address` and `Sign`). Hardware wallets, remote
signing services, and multisig collection all fit behind it without any
orchestration change. See the `TODO(extension)` note in
[`internal/stellar/signer.go`](internal/stellar/signer.go).

If your implementation holds key material, follow the existing rules: keep it
unexported, never log it, never serialise it, and never include it in an error
message.

### Adding a store backend

Implement `store.Store` and run it against `runStoreSuite` in
`internal/store/store_test.go` — the same suite both existing implementations
pass. That's what keeps their semantics aligned.

**Example:** start from `internal/store/memory.go`, which implements every
method with no dependencies. The compile-time assertion catches a missing
method before the suite runs:

```go
// internal/store/mystore.go
package store

// MyStore is a Store backed by <whatever>.
type MyStore struct{ /* ... */ }

var _ Store = (*MyStore)(nil)

func NewMyStore(dsn string) (*MyStore, error) { /* ... */ }
```

Then point the shared suite at it. The factory returns a fresh, empty store per
test, so use a throwaway database or `t.TempDir()`:

```go
// internal/store/mystore_test.go
package store_test

import (
	"testing"

	"github.com/soroworks/soroforge/internal/store"
	"github.com/stretchr/testify/require"
)

func TestMyStore(t *testing.T) {
	runStoreSuite(t, func(t *testing.T) store.Store {
		s, err := store.NewMyStore(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, s.Close()) })
		return s
	})
}
```

Finally, teach `store.Open` (and `NewMigrator`, if the backend brings its own
schema) which `DATABASE_URL` belongs to it, and add the backend to the table
above so the production column stays accurate.

### Adding a CLI command

Add a `newXxxCmd()` in [`cmd/soroforge/root.go`](cmd/soroforge/root.go)
and register it in `newRootCmd`. If it does anything a CI pipeline would want,
add the matching HTTP handler too, and have both call the same service method.

**Example:** the shape [`cmd/soroforge/status.go`](cmd/soroforge/status.go)
uses — one file per command, constructed by a `newXxxCmd()`:

```go
// cmd/soroforge/drift.go
package main

import (
	"github.com/soroworks/soroforge/internal/deploy"
	"github.com/spf13/cobra"
)

func newDriftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift <alias>",
		Short: "Report drift between the ledger and the recorded state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp(cmd.Context(), appOptions{})
			if err != nil {
				return err
			}
			defer a.Close()

			result, err := a.service.Status(cmd.Context(), deploy.StatusRequest{
				Alias:   args[0],
				Network: flags.network,
			})
			if err != nil {
				return err
			}
			return printStatus(result)
		},
	}
	attachAliasCompletion(cmd)
	return cmd
}
```

Then add `newDriftCmd(),` to the `cmd.AddCommand(...)` list in `newRootCmd`.
If CI should be able to call it, give it a route in `internal/api/api.go` and a
handler that calls the same `a.service` method — neither surface owns logic of
its own.

### Adding a migration

Add a numbered pair in `internal/store/migrations/`
(`000002_thing.up.sql` and `000002_thing.down.sql`). They're embedded into the
binary, so nothing else needs wiring. Write a real `down` — reversibility is
tested.

**Example: adding a new `status` command**

```bash
# 1. Create the command file
touch cmd/soroforge/status.go

# 2. Implement newStatusCmd() in cmd/soroforge/commands.go
# 3. Register it in newRootCmd()
# 4. Add the matching HTTP handler in internal/api/handlers.go
# 5. Both call the same deploy.Service method
```

## Deliberately out of scope

These are not oversights. Please open an issue to discuss before building one:

- A web UI
- Key generation, storage, or rotation beyond reading a provided key
- Multisig and threshold signing
- CI plugin integrations

The interfaces above exist so these can be built *on* SoroForge rather than in
it.

## Testing expectations

- `go test ./...` must never require a network or a database. Anything that
  does belongs behind an environment-variable skip, like the Postgres and live
  suites.
- Test behaviour, not implementation. The most valuable tests here assert what
  SoroForge *does* — that a dry run submits nothing, that a failed transaction
  is never recorded, that drift is detected against the ledger rather than the
  database.
- Error messages are part of the interface. If an error tells a user how to fix
  their problem, assert on that.

## Style

- Standard Go. `gofmt` is enforced by `make check`.
- `make lint` runs golangci-lint with `.golangci.yml`, at the same pinned
  version as CI (via `go run`, so nothing needs installing). Fix findings rather
  than suppressing them; when a finding is a false positive, suppress only that
  line with `//nolint:<linter> // reason`. A bare or unexplained `//nolint` is
  itself a lint error.
- Use `log/slog` for logging. Never log key material.
- Comments should explain *why*, not restate the code. The tricky parts of this
  codebase — manual transaction assembly, the two-transaction deploy, comparing
  against the ledger rather than the database — are commented that way, and
  matching that style helps the next reader.

## Verifying against a changed SDK

Soroban's RPC evolves. After bumping `github.com/stellar/go-stellar-sdk`:

```bash
make test        # catches compile-level breaks
make test-live   # catches response-shape changes the mocks cannot
```

Then update the version table in the README. The mocks will happily keep passing
against an API that no longer exists — that's exactly what the live tests are
for.

## Pull requests

Keep them focused, explain the "why" in the description, and make sure
`make check` passes. If you're changing deploy or upgrade behaviour, say how you
verified it — a testnet dry run counts.

## License

Contributions are licensed under [Apache-2.0](LICENSE).
