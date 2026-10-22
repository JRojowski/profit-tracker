# 04: Re-import and Import Conflicts

**What to build:** The user imports a newer full-history export, or the same file again. The tool skips the known Cash Operations and adds only the new ones. When a known XTB ID has different data, the tool shows an Import Conflict in the preview. The user selects "keep existing" or "use new" for each conflict. The tool saves the Import only when all conflicts have a decision. The user can cancel an Import in preview.

**Blocked by:** 02 (Import an export into a new Account).

**Status:** done

- [x] The same file imported two times gives zero new Cash Operations and no change to the data.
- [x] A newer export that overlaps an older one adds only the new Cash Operations.
- [x] A known XTB ID with a different amount, time, type or Comment is an Import Conflict of kind "changed data".
- [x] An Import with an open conflict has the state needs-review. `POST /api/imports/{id}/confirm` fails until each conflict has a decision.
- [x] `POST /api/imports/{id}/conflicts/{conflictId}` records the decision.
- [x] `POST /api/imports/{id}/cancel` cancels the Import. Nothing changes.
- [x] The Import page shows the conflicts with the stored data, the new data and the two buttons.
- [x] API tests cover: the same file twice, an overlapping newer file, a conflict with each decision, a cancel.
