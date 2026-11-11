# Notifier (ntfy) scripts

The ntfy install script that `POST /tumblebug/notifier/ntfy` runs on the provisioned VM lives in the embedded-scripts
package: [`src/core/embedded/scripts/install-ntfy.sh`](../../src/core/embedded/scripts/install-ntfy.sh).
It is compiled into the cb-tumblebug binary with `//go:embed`, so no network access to this repository is needed at runtime.

To try a modified copy without rebuilding, place it at `$TB_ROOT_PATH/scripts/notifier/install-ntfy.sh`
(`/app/scripts/notifier/install-ntfy.sh` in the container); cb-tumblebug prefers that file when it exists.

Manual run on an Ubuntu VM (as root):

```bash
export TB_NTFY_IP=1.2.3.4 TB_NTFY_TOPIC=tb-demo \
  TB_NTFY_WRITE_TOKEN=tk_... TB_NTFY_READ_TOKEN=tk_... \
  TB_NTFY_WRITER_HASH='$2a$10$...' TB_NTFY_READER_HASH='$2a$10$...' \
  TB_NTFY_FQDN=ntfy.example.com   # optional: Let's Encrypt instead of self-signed
bash install-ntfy.sh
```
