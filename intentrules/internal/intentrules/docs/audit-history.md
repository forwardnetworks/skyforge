# Audit history of the rules layer

What independent audits found, and where each fix is proven. Hand-written, unlike `rules.md`.

## First audit: five blockers

1. **The switch did not reach validate.** `Netlab.DisableIntentRules` was read from a request field nobody set, so turning the layer off
   only worked in the designer. It is now read from `req.Config.Netlab` (every caller carries the config). Test:
   `TestDesignerPolicyIncludesIntentRuleWarnings`; loaders: `TestDisableIntentRulesIsCarriedByBothLoaders`.
2. **Exponential recursion in group membership.** `members()` had a depth cap but no visited set, so mutually recursive groups with fan-out
   cost 2^n. A visited set cuts the cycle. Test: `TestCyclicGroupsDoNotBlowUp`.
3. **Findings netlab could have owned survived a clean compile.** After a successful compile, a finding for a code netlab checks
   exhaustively is the loader being wrong, so it is dropped (`NetlabChecksCode`, `MergeWithCompile(..., compileSucceeded)`). Tests:
   `TestAGoodCompileDropsCodesNetlabCouldHaveOwned`, `TestIntentRulesAGoodCompileOverridesTheModelWhereNetlabChecks`.
4. **Loader false positives on hand-written topologies.** Device-named groups, `_auto_create` on list-form groups and in `defaults`,
   `~regex`/glob members, `vlan.mode` on a node or group, `evpn.vlans`/`vxlan.vlans` restrictions, the `svi_ospf` warning opt-out, mixed-AS
   VLANs, `role: external`, `_include`, unknown plugins. One fixture each in `testdata/`, each compared with the real compile.
5. **Double report on the list form of a link.** netlab words the unknown-node error differently for `[a, b]` links; `textOwned` maps
   it. Fixture: `link-unknown-node-list-form`.

## Second audit: one blocker, four follow-ups

- **Fail open and log.** Both call sites run the layer under `Guard` (recover) and log a load error by class; a panic or a load error leaves
  validate and the designer exactly as without the layer. Tests: `TestDesignerPolicyFailsOpenWhenTheRulesLayerBreaks`,
  `TestIntentRulesFailOpenWhenTheLayerBreaks`, and the positive control `TestInjectedFaultsReallyBreakEvaluationAndLoading` (without the
  recover the injected panic does fail).
- **Chart payload.** `skyforge.netlab.disableIntentRules` is rendered into the server, cloudcredentials and worker Encore config only when set;
  the worker loader now carries the key.
- **Licence.** The NOTICE and the Apache-2.0 text are embedded beside the schema (`ThirdPartyNotices`). `go.mod` lists `jsonschema-go`
  as the direct dependency it is.
- **Race.** The plugin-owner table is built under `sync.Once` (`TestPluginOwnersAreSafeForConcurrentFirstUse`, run with `-race`).
- **Severity and dedupe.** A repeated node in a node list is `node-name-duplicate`, likely-wrong (netlab accepts it); a repeated link id
  stays `schema-unique`, will-not-work. An IRB VLAN with no address (`pool: l2only`, `prefix: {ipv4: false}`) carries no OSPF; netlab
  ignores `prefix: false` alone, so that form is still flagged. A group listing an undefined member is not also reported as empty.

## Third pass: the designer IRB rule's false-warning gaps (Roll N)

- **A pool that addresses nothing.** A custom pool that defines neither an IPv4 nor an IPv6 prefix (for example only `prefix: 24`) gives the SVI
  no address and no OSPF, like `l2only` (measured with the pinned, patched netlab; fixture `evpn-svi-custom-pool-no-address`). A pool with
  IPv6 only is NOT suppressed: netlab runs OSPFv3 on that SVI (fixture `evpn-svi-custom-pool-ipv6-only-still-warns`).
- **`vlan.mode` on an interface or link.** An interface (or link, or `vlan.trunk` entry) that carries the VLAN routed has no SVI at that
  node, so the node is not counted for the VLAN (`evpn-svi-interface-mode-route`). Only that node is dropped: a VLAN still carried by two
  other IRB nodes still warns (`evpn-svi-interface-mode-route-third-leaf-still-warns`). netlab accepts only `route` at that level; `bridge`
  is read too. A node that has the VLAN through two interfaces, one of them routed, is treated as having no SVI (a missed warning, never a
  false one).
