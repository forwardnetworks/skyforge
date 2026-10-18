# v14 truth check (2026-10-10)

Carries every row of V13_TRUTH_CHECK.md (same evidence, same labels) for the claims that moved unchanged: the loop, three questions, demo 3 and 4 numbers, firewalls, intent (designed and prototyped, 59 of 60, 83 of 88), the eight defects, AI labels, slide 7 scale numbers, patch counts (netlab 49, KNE 80, meshnet-cni 24, koko 15; published vs coming), QR repo, PR comment (run 8cec7c86), pre-flight (run b9a03a53).
Below: what is new or reworded in v14.

| v14 slide | Claim | Status | Evidence |
|---|---|---|---|
| 1 | Rule A deny inside 10.10.101.0/24 to outside 10.10.102.128/25 is the intended retired range; B (10.10.102.0/25) is the one-digit fault; both look reasonable | measured (demo 3) | `docs/drafts/napalm` change CHG-PREDICT-3-FIREWALL-DETOUR; intent example in `change-intent-examples.yaml` (fix dst 102.128/25, break dst 102.0/25); explainer html. The notes call B "the fault I planted", not a real typo |
| 1 | "Both pass review" | rhetorical | the slide's claim is that neither is obviously wrong to a reader; no review record exists |
| 17 | lab proves the machinery works, twin judges the change | the talk's argument | evidence: slide 22 (eight defects, found on the real PA-VM; Forward judged correctly) |
| 17 | "napalm pushes it with an automatic undo armed" | built; PAN-OS undo is platform-side (slide 20) | V13 rows 19, 22 |
| 17 | "Not on production" | true | no production run in the repo |
| 18 | "BGP sessions 88 to 88" is the ring demo (run 187a8c25, explainer page); "118 of 118 checks, 0 regressions, reach 2 of 2, sentinels 2 of 2" is the firewall fix run 49d789f3 | measured | `light-6-fix-evidence.png` (device-state families are not recorded on the demo 3 card, so 88 to 88 stays attributed to the ring) |
| 19 | the 55 s light clip `demo3-fault-held-and-fix-light.mp4`, assembled from real screenshots of the live PA-VM run cards: break run 18210ea7 (HELD, then REVERTED, baseline 14824, after 14825); fix run 49d789f3 (HELD, then CONFIRMED, baseline 14828, after 14829, 118 of 118 existing checks, 0 regressions, reachability 2 of 2, sentinels 2 of 2) | measured | owner brief 2026-10-10; `evidence/screenshots/v14/demo3-fault-held-and-fix-light.mp4`, `light-slide-1..6.png`, `light-1..6-*.png`. It is an assembled animation of real screenshots, not a screen recording; the slide says so. Clip timing measured by frame change |
| 19 | 'Why Forward held it': regression outside the blast radius (pd3_fwin_h_to_fwout_h PASS to FAIL), segmentation pass, IP uniqueness 1 of 1, sentinels 1 of 2 (failing sn_pd3_fwin_to_fwout_via_pa_fw) | measured | owner brief; the card text in the clip |
| 19 | Run fix / Run break buttons start the run on their own (server image r46) | built, live | owner brief 2026-10-10; not re-checked |
| 19 | PR comment: REVERTED not applied, three-part judgment, labelled AI summary | measured, run 8cec7c86 | V13 row 21; screenshot `evidence/screenshots/v13/orig-9-forgejo-pr-comment.jpg` |
| 20 | cEOS timed commit, IOL timed rollback, PAN-OS saved config plus deadline plus reaper | in use / built | V13 row 22 |
| 21 | "The same intent would drive the lab and the twin" | designed | change-intent-design.md sec. 1: compilers produce the napalm delta and the Forward Predict input and derive the checks |
| 22 | "Our test fake could not raise any of them" (the eight bugs) | measured | `memory/reference-panos-napalm-live-lessons...`: every one was invisible to the unit-test fake until the real PA-VM answered. (Reworded 2026-10-10 from "a simulator would have found none".) |
| 7 | "100 cEOS Ready in 374 s; 250 stalled at 220; 500 of 500 in 925 s, 13% CPU" | measured 2026-10-09 | scale-profile results (as V13 row 8) |
| 7 | 136-node lab with the PA-VM runtime-ready in 19 min 25 s with the four apply-phase switches off, 15 min 28 s with them on, one run each; ARP resolved and ping across the firewall worked | measured (one run each) | owner brief 2026-10-10; no file found. Replaces v13's "about 20 minutes, 9.5 of it the PA-VM" (owner-brief only) |
| 7 | "The lab is where cost and scale live. The twin is where the network's real size lives." | the talk's argument | no measurement; the twin models the collected network, the lab costs pods (slide 7 numbers) |
| 4 | clabernetes to KNE March 2026, k3s to Talos July 2026 (notes), "wrong twice" | in use | V12_TRUTH_CHECK.md (dated cutovers) |
| 24 | "A check that looks at nothing is a failure, not a pass" | built | V13 row 20 |
| 25 notes | upstream series published, rest coming | true | V13 row 6 |

| 21 | a compiler for two changes reproduces the pack byte for byte; intents are not in the pack | built as a compiler, verified today | owner brief 2026-10-10 (no file read by me). The slide does not claim intents are in the pack or in use |
| 23 | the loop controller core exists and is simulated, not run live | built core, simulated | owner brief 2026-10-10; agentic-loop-design.md for the design. The slide does not claim it runs live |
| 19 | the Run fix / Run break buttons execute the run themselves | built, live (server image r46) | owner brief 2026-10-10 (notes only) |
| 1, 19, 22, 7 | light jokes ("Ask me how I know.", "tells you something about reviews", "it has opinions", the capacity planning spreadsheet) | not claims | style only |

| 19, 18, appendix 30 | break run b6f0435e (held, then reverted; baseline 14840, after 14841; 118 checks, 2 regressions; reachability 1 of 2, sentinels 1 of 2); fix run 2aa18f9f (held, then CONFIRMED; baseline 14844, after 14845; 118 of 118, 0 regressions, reach 2 of 2, sentinels 2 of 2) | measured | owner brief 2026-10-10; `evidence/screenshots/v14/final-*.png`, `demo3-fault-held-and-fix-light.mp4` (68 s, assembled from real screenshots, not a screen recording). Supersedes the 18210ea7 / 49d789f3 rows above |
| 19, appendix 30 | the pull request comment is short: an 'At a glance' list, tables and Forward's full judgment folded; break comment 'REVERTED: not applied' (status check red, 2 failing checks), fix comment 'APPLIED and CONFIRMED' (status check green, sweep 118/118, 0 regressions); both on pull request #27 | measured | `final-break-4-PR-comment.png`, `final-fix-4-PR-comment.png`. The fix comment is the first live capture. Both show 'Predict pre-flight has not run for this change yet' (off by default), which is true |

| all presented slides | click animations (Fade, on click) | presentation feature | no new facts; see V14_ANIMATIONS.md. Slide 1's reveal order changed (answer, then the joke) and slides 21 and 22 text was split into boxes; the wording is unchanged |

| 23 | the loop ran end to end on the demo firewall with a model-drafted rework: fault held and reverted, model rule `DENY-INSIDE-TO-RETIRED-RANGE` (inside 10.10.101.0/24 to outside 10.10.102.128/25, deny, top), a person approved, Forward confirmed (`forwardChangeLanding: confirmed`) | measured 2026-10-11 | loop-440302c1, child run 56e552df. The loop had parked on its 30 min approval timeout before the approval, so the approved run was queued through the admin execute endpoint, not by the loop; the slide does not claim an unattended loop |

| v15: 21, 29 | an intent form of the firewall change ran live: break held by the pre-flight before any device was touched (run 3139d84f), fix applied and confirmed by Forward (run 394f25f2: 118/118, 0 regressions, 6/6 own checks, reach 2/2, sentinels 2/2); facts read from the device's running config at create (read runs 833a63b6, 6d119190) | measured 2026-10-12 | run records on the deployment; evidence in docs/drafts/napalm/open-items-2026-10-10.md. The run used a temporary deployment-own pack, since deleted; the shared pack still has the old form; the loop refuses intent changes. |
