# v16 changes (2026-10-16)

Committee feedback from Sergey Myasoedov (NANOG PC), and the response. The slides are locked: no slide image and no PDF changed.

## What the committee said

1. "Skyforge" appears often and is hard to search. It was not clear from the slides what it is.
2. "Predict" is unclear: product, internal tool, or verb?
3. "Forward" as the twin name is confusing, since the speaker works at Forward Networks.
4. The speaker notes are long. Recommended a clean printed copy so the speaker is not relying on AV to match his pace.

## What changed

- **README.md, SAY EARLY lines.** Slide 1 has three SAY EARLY lines (Skyforge, Forward, Predict), in the order the room hears them. Slide 4 repeats the Skyforge line. Slide 13 repeats the Predict line. These are spoken lines, written in the speaker's voice. Items 1 to 3.
- **README.md, slide 14.** A note on the two column headers: left is my lab (Skyforge, observed execution), right is Forward's Predict (modeled behavior). Item 2 and 3.
- **README.md, "open source" and "published".** Slide 25 and slide 31 notes now say the public repository holds patches only, not Skyforge. Skyforge is not described as open source anywhere in the notes. Item 1, plus a correction: Skyforge is internal and not open source. The committee had assumed otherwise.
- **README.md, slide 31.** "49" is named as the netlab series total, not a total across series. The published total (65) is given separately and says it is published now. Nothing is accepted upstream yet, and that stays.
- **NANOG98_CUE_CARD.md and NANOG98_CUE_CARD.pdf (new).** One page per main slide (1 to 26), with the slide number and title, up to three bullets taken from the notes, and the click cue. Page 1 carries the three SAY EARLY lines. The last page lists the appendix titles (27 to 34). Item 4.
- **V16_TRUTH_CHECK.md (new).** Records the checks made in this pass, and what is still to verify before Monday. Includes the patch-count mismatch on slide 31 (see below), the Forward 26.9 label on slide 13, and the venue on slide 1.
- **V15 files.** Not changed.

## Timing

Slide 1 now carries about 40 seconds of SAY EARLY lines on a 30 second slot. The DELIVERY clocks in the notes were not rewritten. The 39:00 deck time moves by about 40 seconds unless the Q&A window (6:00) absorbs it. Decide before the talk, do not rewrite the clocks.

## Not done, and why

- No slide was changed. The committee will not accept slide changes.
- The public GitHub repository was not read or changed.
- Patch counts (netlab 49, KNE 80, cEOS 19) were not reconciled with the files on disk. The disk shows 61, 82 and 20. See V16_TRUTH_CHECK.md.
- Forward version on slide 13 is not confirmed. See V16_TRUTH_CHECK.md.
- The header line of README.md still says the source is the v14 deck. Not changed in this pass.

## PPTX with speaker notes

NANOG98_Digital_Twins_Talk_v16_notes.pptx is a notes-carrying image deck: each of the 34 slides is a full-bleed picture of the matching slides/s-NN.png (slide size matches the PNG, 1067x600), so the slides are pictures, not editable. The speaker notes are the cue card (slides 1 to 26) and the appendix lines (27 to 34). The editable v15 source lives on Craig's laptop.

## Pass 2 (2026-10-17)

Craig's review items, done in notes and the cue card only. The slides are still locked and unchanged. No v17 is needed: the notes pptx and the cue card are regenerated as v16.

- Venue: README header now says "NANOG 98, Miami, October 2026" (from slide 1). Versailles removed. The date and slot are flagged for confirmation with the PC.
- Patch counts: slide 31 notes now carry the correction (netlab 49 was the count when pinned; 61 today; KNE 82). Published total 65 confirmed on the public repo. Details in V16_TRUTH_CHECK.md, pass 2.
- Slide 13 Forward build: not verified. The notes say "26.9 line, not a build number". Candidates are in V16_TRUTH_CHECK.md.
- Timing: SAY EARLY cost is paid back by cuts on slides 1, 2, 4, 13, 16, 17, 18, 21, 23 and 24 (about 230 words). Net about zero at 150 wpm.
- Predict harness: IF ASKED note on appendix slide 27 (notes only).
- Files: README.md, NANOG98_CUE_CARD.md and .pdf (27 pages), NANOG98_Digital_Twins_Talk_v16_notes.pptx (34 slides, notes regenerated), NANOG98_PASTE_NOTES.md (new), V16_TRUTH_CHECK.md (pass 2).
- The header's "source is the v14 deck" line is now "v15" (it was listed as open in pass 1).
