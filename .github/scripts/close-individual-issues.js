/**
 * Close individual async daily issues after the report is generated
 *
 * @param {object} github - GitHub API client
 * @param {object} context - GitHub Actions context
 * @param {Array} issues - Array of issues to close
 * @param {string} reportIssueUrl - URL to the generated report issue
 * @returns {Promise<void>}
 */
async function closeIndividualIssues(github, context, issues, reportIssueUrl) {
  console.log(`Closing ${issues.length} individual async daily issues...`);

  const closeComment = `✅ This async daily report has been processed and included in the [Daily Report](${reportIssueUrl}).\n\nClosing this individual issue.`;

  let successCount = 0;
  let errorCount = 0;

  for (const issue of issues) {
    try {
      // Add a comment with the link to the report
      await github.rest.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issue.number,
        body: closeComment
      });

      // Close the issue
      await github.rest.issues.update({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: issue.number,
        state: 'closed',
        state_reason: 'completed'
      });

      console.log(`✓ Closed issue #${issue.number} by @${issue.user.login}`);
      successCount++;
    } catch (error) {
      console.error(`✗ Failed to close issue #${issue.number}:`, error.message);
      errorCount++;
    }
  }

  console.log(`\nSummary: ${successCount} issues closed successfully, ${errorCount} errors`);
}

module.exports = { closeIndividualIssues };
