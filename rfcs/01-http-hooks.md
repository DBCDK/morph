# HTTP hooks for integrating morph deployments into existing infrastructure

## Summary

We need a way of triggering external systems during deployments. We also want this to be generic, and promote a healthy way of collaborating on deployments (=> do not introduce any local state).

Examples:
- Trigger draining a kubelet prior to deployment
- Mark hosts in maintenance before deployment, and mark as ready after health checks are OK
- Provide a way of skipping certain hosts for deployment
- Provide locking to away multiple people deploying the same host (or group of hosts)
- ...

Hooks should be composable, so that one hook point can call out to multiple hooks.


## Overview of implementation

Hooks can be split into two types:
1.  Multi-host hooks
1.  Single-host hooks

Multi-host hooks would be applicable when selecting hosts to work on, while single-host hooks are relevant when dealing with the actual deployment of a host.

The suggested implementation of the hooks are exactly the same, only differing in the actual payload passed around:

Serialize the payload as JSON, and send it to the configured endpoint. The endpoint then transforms it, and returns a payload in the same format. Non `200 OK` responses causes deployments to stop, unless the user opts into ignoring the result of endpoints using a command line option.

### Why not execute commands instead?

The most straightforward way of implementing hooks would be by executing a command and sending all required state as an argument to the command.

Currently morph can't execute anything on the user's machine (unless nix sandboxing is disabled), so this feature would be the first instance of morph executing arbitrary scripts during deployments. By implementing hooks as calls to HTTP endpoints we avoid many security related issues.

## Protocol

The only difference with single- and multi-host payloads is that multi-host is a list of single-host payloads.

Sing-host payload format:
```json
{
  "host": "http.example.com",
  "actions": [ "push", "switch-boot", "reboot", .... ]
}
```

Actions can only be things that express an actual operation on the remote host. Operations like `build` thus can't be an action, since that's a requirement of actual actions, but `push` _is_ an action, since it changes the host, but removing `push` while preserving `deploy/boot` will not cause any chance in practice, since `deploy/boot` depends on `push`.

Actions are repeatable and the order significant. Morph will not collapse the list of actions in any way, but will perform other required actions transparently. One such example is ensuring `push` is performed before `switch`. Actions are performed left to right, and after each action it is removed from the list. Later hooks will not see actions that has already been performed.

### List of actions

- `skip`: Ignore the remaining actions on the host
- `push`: Push the derivation and outputs
- `reboot`: Perform a reboot of the host
- `switch-boot`: Switch system on next boot
- `switch`: Switch system now
- `test`: Switch system now but don't create a new generation
- `dry-activate`: Run dry-activate on the host
- `upload-secrets`: Upload secrets to the host
- `run-secret-triggers`: Run the triggers defined by the secrets

(List might be incomplete)


## List of hook points

What follows is a list of steps that can be hooked into:

| step | type | notes |
|---|---|---|
| select | multi | This is the step where the select flags (`--limit`, `--skip`, `--every`, `...`) are evaluated. Could be used to modify the list before and after. |
| deploy | single | Deployments step that actually change the host, so _not_ `dry-active`. |
| reboot | single | Only run if a reboot is actually requested. |
| healthchecks | single | |

FIXME: This list needs to be fleshed out. Should steps be able to overlap (like making `reboot` an inner step of `deploy`).

The hook-points are named `{pre|post}-stepname`, e.g. `pre-select` and `post-deploy`. Not all combinations might make sense, but it's probably easier to implement it anyways for sanity's sake.


## Future work

- Limit the changes a hook can perform (e.g. `read only` (for triggering external actions without modifying the execution plan), `filter only` (only allow removing parts of the plan), `do anything` (no restrictions for modifying the plan)).
