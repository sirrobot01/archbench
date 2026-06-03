# Security Model

ArchBench executes arbitrary commands locally, on configured SSH hosts, and
inside Docker containers. Only run specs you trust.

## SSH

The SSH runner delegates connection handling to the system `ssh` client. It
inherits the user's SSH config, agent, identities, ProxyJump settings, and
known_hosts behavior.

By default, host-key checking uses `StrictHostKeyChecking=accept-new`. Setting
`ARCHBENCH_SSH_INSECURE=1` disables host-key verification and prints a warning.

## Docker

The Docker runner delegates to the system `docker` CLI and runs the suite inside
a container created from the target's `image`. The container is created with a
fixed label, used only as a keep-alive process between run groups, and
force-removed on cleanup. Commands and a pinned `platform` are the only image
inputs ArchBench controls; the image itself is whatever the spec names, so treat
images the same way you treat the commands you run.

## Project Sync

When the project directory is a Git worktree, ArchBench syncs files from:

```sh
git ls-files --cached --others --exclude-standard
```

That includes tracked files and untracked non-ignored files, while excluding
ignored files and directories such as local build outputs or editor state. If
Git is unavailable, ArchBench falls back to a conservative directory walk that
skips VCS metadata and result artifacts.

## Environment Variables

Custom `runs[].env` values may contain secrets. SSH and Docker execution write
them to a 0600 file in the work directory over stdin and source that file. They
are not placed on the command line, where they would be visible via `ps` or
`docker inspect`.
