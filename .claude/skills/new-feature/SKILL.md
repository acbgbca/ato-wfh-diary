---
name: new-feature
description: Guided workflow for implementing a new feature
---

Implement a new feature for the ATO WFH Diary: $ARGUMENTS

Follow this workflow:
1. Ask clarifying questions about the feature requirements
2. Plan the implementation (affected files, schema changes, API changes) and present for approval
3. On approval of the plan, create a Github issue with the details of the change. Include enough information that you could pick up the ticket in the future and still know what to do
4. Create a feature branch for the change, including the Github issue number in the name
5. Use a TDD approach by writing or update the tests first, and adding just enough code so the tests compile but fail. Include E2E tests if the UI is affected
6. Implement the changes
7. Update docs/ (features.md for behaviour changes, data_model.md for schema changes)
8. Verify that the tests now pass
9. Commit, push, and create a PR, ensuring to reference the GitHub issue created for this change.
