# Embedded scripts

Scripts in `scripts/` are compiled into the cb-tumblebug binary with `go:embed` and run on VMs that
CB-Tumblebug provisions. They are not operator tooling (that lives in the repository-level `scripts/`).

| Script | Used by | Runs on | Inputs |
|---|---|---|---|
| `init-script.sh` | `core/infra` (`initScript` of a NodeGroup) | every Linux node, first boot, as user data; reports each step start and the end | `TB_INIT_ACCESS`/`TB_INIT_USER`/`TB_INIT_PUBKEY`, `TB_INIT_SCRIPT_COUNT` + `TB_INIT_SCRIPT_<i>_NAME`/`_B64`, node identity `TB_NS_ID`/`TB_INFRA_ID`/`TB_NODEGROUP_ID`/`TB_NODE_ID`/`TB_CSP`/`TB_REGION`, `TB_NOTIFY_*` |
| `install-ntfy.sh` | `core/notifier` (`POST /tumblebug/notifier/ntfy`) | the ntfy VM, over remote command | `TB_NTFY_*` |

Conventions:
- Each script reads its inputs from environment variables only; CB-Tumblebug prepends `export K='v'` lines.
- Add a new script here and expose it with one accessor in `embedded.go`. Changing a script requires a rebuild.
- `install-ntfy.sh` can be overridden on disk at `$TB_ROOT_PATH/scripts/notifier/install-ntfy.sh` for trials
  without a rebuild. `init-script.sh` has no override because its report format must match the binary.
