# MPLS LDP LFIB — IOS-XR (XRd)

The iosxr counterpart of [`../ldp-lfib`](../ldp-lfib) (EOS).

`h1 — pe1 — p1 — p2 — pe2 — h2`, with a **BGP-free core**: `p1`/`p2` run ISIS +
MPLS only and hold no route to the host subnets, so traffic cannot cross by IP
forwarding.

Run it with the `validate` deployment action, which executes the topology's own
`validate:` block against the running lab. The verdict lands on the `Validated`
deployment condition and as a `netlab/validate-results.json` artifact.

**A successful ping is not proof of label switching.** The final test reads
`show mpls forwarding` on `p1` and requires the Bytes Switched counter to have
moved; counters start at zero at boot, so that is an increment. Both the ping
and the counter check must pass.

EOS `MPLS/ldp-lfib` is the known-good control lane for comparison.
