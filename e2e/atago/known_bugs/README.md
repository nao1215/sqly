# Known dialect limitations

Dialect behavior sqly cannot currently reproduce. Every scenario here is
expected to fail: it asserts what the source dialect does, so it fails today and
would start passing if the obstacle ever went away.

These specs sit outside the `e2e/atago/*.atago.yaml` glob that
`scripts/run_e2e.sh` uses, so CI stays green.

```sh
sh scripts/run_known_bugs.sh
```

This directory started as a set of reproductions for the SQL-dialect defects
found by probing sqly's CLI. filesql v0.20.0 fixed all but one of them, and
those scenarios moved into the active suite as `e2e/atago/dialect_*.atago.yaml`.
What remains is the one case that needs more than a translation rule.

When a scenario here starts passing, move it into the matching
`e2e/atago/*.atago.yaml` suite so CI protects it from regressing.
