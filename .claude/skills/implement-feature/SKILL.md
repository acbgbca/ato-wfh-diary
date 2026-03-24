---
name: implement-feature
description: Guided workflow for implementing a new feature
---

Implement a new feature for the ATO WFH Diary:

Follow this workflow:
1. Get a list of open issues in GitHub. Display the summaries to the user, and ask them which issue we should be implementing
2. Load the full description of the issue. Ensure that you understand what is being built, and the changes that need to be made. Ask questions if anything isn't clear
3. Explore the local repository, and verify that the proposed solution will work and that there are no gaps. If anything isn't clear, ask the user.
4. Once everything is confirmed, make sure we are on the main branch and run `git pull` to ensure we have the latest changs. Then create a feature branch for the change, including the Github issue number in the name
5. Use a TDD approach by writing or update the tests first, and adding just enough code so the tests compile but fail. Include E2E tests if the UI is affected
6. Implement the changes
7. Update docs/ (features.md for behaviour changes, data_model.md for schema changes)
8. Verify that the tests now pass
9. Commit the change, ensuring that the commit references the issue number
10. Ask the user to verify the change. If they have any feedback or changes, make the necessary updates to the application. After each change make a new commit to the branch with the change. All commits should also reference the issue number
11. Once the user is happy with the change push the branch and create a PR, ensuring to reference the GitHub issue created for this change.

Your work is now complete. Do not offer to start anything else, that will happen in a new session
