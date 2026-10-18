# v16 truth check (2026-10-16, notes only, slides locked)

Covers the speaker notes changed for the committee pass. The slides are v15 and are not edited. Rows marked VERIFY are not settled and must be checked before Monday 2026-10-19. Earlier rows stay in V15_TRUTH_CHECK.md.

| slide | claim | status | evidence / what to verify |
|---|---|---|---|
| 31 (slide image) | "49 patches" sits on the netlab row, labelled "coming" | read as netlab only | Slide 31 image: netlab 49, KNE 80, meshnet-cni 24 (published), koko 15 (published). The notes now say 49 is the netlab series total, not a total across series. |
| 31 (slide image) | "Published now" list: koko 15, meshnet-cni 24, Arista cEOS operator 19, catalystwan 5, catalystwan-sdk 1, nokia-grpc 1 = 65 | matches the slide text; 3 counts need verifying | On disk at commit 2d1d1e4f7 (`patches/*/*.patch`): koko 15, meshnet-cni 24, catalystwan 5, catalystwan-sdk 1, nokia-grpc 1 match. **arista-ceoslab-operator has 20 files, not 19.** |
| 31 (notes) | netlab 49 and KNE 80 | VERIFY | On disk: `patches/netlab` has 61 `.patch` files, `patches/kne` has 82. The 49 and 80 figures do not match a plain file count. They may come from the upstream register in `patches/UPSTREAM.md` (it describes a netlab series re-cut 74 to 29 on 2026-09-27, which is a different scheme). Before Monday: state which count the talk means, and make the slide and the notes say the same thing. Nothing here was changed on the slide. |
| 13 (slide image) | subtitle "Forward 26.9, EOS" | VERIFY, not confirmed | V15_CHANGES.md line 20: the label was changed from "Forward 1.0.0" to "Forward 26.9" on 2026-10-12 and the raw capture was taken 2026-10-04 and not re-captured. Check: (1) which Forward track (primary or release) collected network 5089 and snapshots 13984/13985 on 2026-10-04; (2) that track's build string on that date, from the Forward version recorded in the deploy pins or the lab's run record for those snapshots; (3) whether the capture on the slide shows a build string. The version of the login used for this check does not answer this: it points at a different Forward instance. |
| 1 (slide image) vs README header | venue: slide 1 says "NANOG 98, Miami, October 2026"; README header says "Versailles" | VERIFY | Use the venue on the slide only after confirming it with the NANOG PC. The notes do not name a city. |
| 14 (slide image) | column headers are "Skyforge / observed execution" and "Predict / modeled behavior" | verified | Read on the slide image. The notes now name the columns as shown. The phrase "Skyforge Predict" is not on the slide. |
| 25 (slide image) | "Patches and notes: github.com/forwardnetworks/skyforge" | verified on the slide | The notes now say the repository holds patches only. This check did not open the public repository (not in scope). Verify the repo README says the same before Monday. |
| 1, 4, 13, 14, 25, 31 (notes) | "open source" and "published" wording | verified in the notes | `grep -n -i 'open.source\|github\|published' README.md`: no sentence now says or implies Skyforge is open source. The Skyforge SAY EARLY line says the opposite on purpose. |
| slide 1 (notes) | "Forward" is the twin and the platform | clarified in notes | SAY EARLY on slide 1 names the product. Slide 1 image says "Principal Architect, Forward Networks". |
| 1 (notes) | Predict definition | clarified in notes | SAY EARLY on slide 1, repeated on slide 13. The definition is a sentence that describes what the feature does, from the notes' own description. |
| all | slide text itself | not editable | The slides are locked. If any slide text implies Skyforge is open source, it cannot be changed. None found on slides 1, 4, 14, 25 or 31 at the time of this check. Slides 2 to 12, 15 to 24 and 26 to 34 were not checked as images in this pass. |

## Pass 2 (2026-10-17): reconciled items

Checked against the repo at origin/main be8c5ff5a (fresh clone, 2026-10-17) and the public repo github.com/forwardnetworks/skyforge at dfcf2d20 (read only, nothing pushed). Where a row says "not sourced", the repo does not contain the evidence and this pass did not find it elsewhere.

| item | claim | status | evidence / what to say |
|---|---|---|---|
| 1 venue | slide 1: "NANOG 98, Miami, October 2026" | fixed in README | The README header said "Monday 2026-10-19, 2:00 PM Eastern, Versailles". Versailles appears in no other talk doc (grep of docs/talks/nanog98 and the nanog package). The date and slot time are not on the slide and are not sourced in the repo. Header now gives the venue as on slide 1. Craig confirms the date and slot with the PC. |
| 2 patch counts, netlab | slide 31: 49 | corrected in notes | Disk: `patches/netlab` = 61 `.patch` files at main. The 49 is real but old: docs/drafts/scale-profile (2026-10-09 to 10-11) ran on "all 49 patches" at pinned netlab 9efd2ddb. The series grew to 61 by 2026-10-13 (docs/drafts/netlab-upstream-2026-10-13.md). Netlab is not on GitHub, so 61 is the local count. Say: "It was 49 when I pinned it for the scale runs. It is 61 today." |
| 2 patch counts, KNE | slide 31: 80 | corrected in notes, source not traced | Disk: `patches/kne` = 82. The 80 on the slide has no source in the repo. Notes now say 82. |
| 2 meshnet-cni | 24 | verified | Disk 24, public 24. |
| 2 koko | 15 | verified | Disk 15, public 15. |
| 2 cEOS operator | 19 published | verified, one local extra | Disk 20 (`0020-bringup-throttle-counts-only-the-sandbox-window` is local only). Public 19. Notes say 19 published and one more local. |
| 2 catalystwan / catalystwan-sdk / nokia-grpc | 5 / 1 / 1 | verified | Disk and public match. |
| 2 published total | 65 | verified | Public repo: 15 + 24 + 19 + 5 + 1 + 1 = 65. |
| 3 slide 13 Forward build | "Forward 26.9" | NOT verified | See below. |
| 4 timing | SAY EARLY adds time | cut | See below. |
| 5 Predict harness | if-asked note | added | Slide 27 notes and cue card appendix page. Sources: docs/drafts/predict-harness-v0-first-slice-2026-10-16.md on origin/docs/predict-harness-v0-first-slice-2026-10-16 (not merged to main). |

### Slide 13: which Forward build

Verified build: none. What the record shows:

- The run (network 5089, base snapshot 13984, CHG-752, predicted 13985, 2026-10-04) is not in git history, docs, deploy or the talk package. `git log --all` and a repo grep for 13984, 13985, 5089 and CHG-752 find only the talk notes and an unrelated LLM pilot file.
- The capture is an image. Nothing in the repo records its build string.
- Forward pins on 2026-10-04, from the repo:
  - Stable track: `26.9.1-06` (commit 2a514fcd3, 2026-10-03, "stable Forward 26.9.1-02 -> 26.9.1-06"). Its licpatch build was `26.9.1-06-licpatch-r1` until d2609f8cf (2026-10-05). Next stable move: 26.9.1-06 -> 26.9.2-01 on 2026-10-07 (d9e3198eb).
  - Primary track: `1.0.0-261003-8a36ec49d30`, replaced by `-sched-r1` on 2026-10-05 (4eab35061), then `1.0.0-261005-3f4fdfdd367` (c863d9e84).
- `fwdctl whoami` reports a different Forward (fwd.app, another organization, 26.9.4-02). It cannot answer this. The repo's current stable tag (values file) reads 26.9.3-01; that is not the 10-04 build.
- The label history (V15_CHANGES.md): "Forward 1.0.0" was changed to "Forward 26.9" on 2026-10-12. That suggests the run was on stable, but the record does not show which track collected network 5089.

Unverified: which track, and the exact build. If the track was stable, the build was 26.9.1-06 (licpatch-r1). If primary, it was 1.0.0-261003-8a36ec49d30. Check: the Forward version of the instance that holds network 5089 on 2026-10-04, or the run record for snapshot 13984. Until then the notes say "the 26.9 line, not a build number".

### Timing (150 words per minute, a rough rate)

- SAY EARLY on slide 1: about 112 words (about 45 s on a 30 s slot). Repeats on slides 4 and 13: about 46 and 36 words. Slide 13 also gained a short build line, about 31 words. Added: about 225 words, about 90 s.
- Cuts (net words): slide 1 intro paragraphs (33 + 21), slide 2 "This talk is about..." (21), slide 4 "This is an internal platform..." (13), slide 16 "A model lets me ask..." (18), slide 17 "This is how I run..." (8), slide 18 card description (23 net), slide 21 vendor example (20 net), slide 23 "In chat..." (28), slide 13 "Show the snapshot identity..." (9), slide 24 "The abstract promised..." (16). Total about 230 words, about 92 s.
- Net: about zero (about -2 s). The deck fits 39:00 only if the pace is near 150 wpm. Rehearse slides 1, 4 and 13 with a timer.
- DELIVERY clocks in the notes are not rewritten.
- Slide 1 is about 150 words, and slide 13 about 130 with its SAY EARLY repeat. The rest are under 120.

### Not verified in pass 2

- The undo fix is on main (see below) but the live cluster was not checked. This machine has no kubectl context.
- The date and slot time for NANOG 98.
- The KNE 80 source.

### Predict harness and the undo defect (for the if-asked note)

- The napalm undo defect (2026-10-17 00:01 UTC): a two-device change, the second device failed, the applier rolled back the first, and r1's running config dropped from 116 lines to 54 while the run reported "reverted". Source: the harness doc above.
- Fix: branch `fix/napalm-undo-fidelity-20261016` (65c570cc) is on the private remote. On main, `72756f02a` (server a619e816) pins it, then `6d9784cb8` (server 491943cd). The first roll that carried it, `7184e00d2` (20261016-orgrel-r20), was reverted by `9072707b4` after the node-role hook failed. `49b973aea` re-rolled the same tree. Main's server pin is 491943cd, which includes the fix. Live cluster state not verified from here.
- Predict verdict path: merged to main (`7f4953543`, roll2). The harness run on 2026-10-16 recorded `skipped` with zero probes for the EOS change, because the live pre-flight only stages PAN-OS rules. No EOS Predict verdict exists yet.
