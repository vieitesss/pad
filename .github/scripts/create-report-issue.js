/**
 * Create a GitHub issue with the daily report
 * 
 * @param {object} github - GitHub API client
 * @param {object} context - GitHub Actions context
 * @param {string} titleDateStr - Date string in YYYY/MM/DD format
 * @param {string} markdownBody - Report content in markdown format
 * @param {string} reportLabel - Label for report issues (default: 'async-daily/report')
 * @returns {Promise<object>} Created issue data
 */
async function createReportIssue(github, context, titleDateStr, markdownBody, reportLabel = 'async-daily/report') {
  console.log('Creating GitHub issue with the report...');
  
  const reportIssue = await github.rest.issues.create({
    owner: context.repo.owner,
    repo: context.repo.repo,
    title: `[Daily Report] ${titleDateStr}`,
    body: markdownBody,
    labels: [reportLabel]
  });
  
  console.log(`Report issue created: #${reportIssue.data.number} - ${reportIssue.data.html_url}`);
  
  return reportIssue.data;
}

module.exports = { createReportIssue };
