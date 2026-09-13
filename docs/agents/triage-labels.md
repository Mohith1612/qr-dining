# Intended triage labels

The agent workflow uses five canonical triage roles. This file records the
intended label strings; it is not evidence that every label is provisioned in
GitHub. Run `gh label list --repo Mohith1612/qr-dining` before applying one.

| Workflow role | Intended label | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer needs to evaluate this issue  |
| `needs-info`               | `needs-info`         | Waiting on reporter for more information |
| `ready-for-agent`          | `ready-for-agent`    | Fully specified, ready for an AFK agent  |
| `ready-for-human`          | `ready-for-human`    | Requires human implementation            |
| `wontfix`                  | `wontfix`            | Will not be actioned                     |

When a skill names a role, use the intended string only if it exists in the live
tracker. Adding or renaming GitHub labels is an external repository change and is
outside documentation-only work.
