# Security Model

ArchBench executes arbitrary commands locally and on configured SSH hosts.
Only run specs you trust.

## SSH

The SSH runner delegates connection handling to the system `ssh` client. It
inherits the user's SSH config, agent, identities, ProxyJump settings, and
known_hosts behavior.

By default, host-key checking uses `StrictHostKeyChecking=accept-new`. Setting
`ARCHBENCH_SSH_INSECURE=1` disables host-key verification and prints a warning.

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

Custom `runs[].env` values may contain secrets. SSH execution writes them to a
0600 file in the remote work directory over stdin and sources that file. They
are not placed on the SSH command line.
