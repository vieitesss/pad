/**
 * Collects async daily issues for a given date and returns the data
 *
 * @param {object} github - GitHub API client
 * @param {object} context - GitHub Actions context
 * @param {string} reportDate - Date string in YYYY-MM-DD format (optional)
 * @param {string} label - Label to filter issues (default: 'async-daily')
 * @returns {Promise<object>} Object containing asyncDailyIssues array and date information
 */
async function collectAsyncDailyIssues(github, context, reportDate, label = 'async-daily') {
  // Determine the date for the report
  let date;
  if (reportDate) {
    date = new Date(reportDate);
  } else {
    date = new Date();
  }

  // Format date as YYYY-MM-DD
  const dateStr = date.toISOString().split('T')[0];

  // Format date as YYYY/MM/DD for issue title matching
  const [year, month, day] = dateStr.split('-');
  const titleDateStr = `${year}/${month}/${day}`;

  console.log(`Collecting async daily issues for: ${dateStr} (${titleDateStr})`);
  console.log(`Using label: ${label}`);

  // Get all issues with the specified label
  const issues = await github.rest.issues.listForRepo({
    owner: context.repo.owner,
    repo: context.repo.repo,
    labels: label,
    state: 'all',
    sort: 'created',
    direction: 'desc',
    per_page: 100
  });

  // Filter issues by title pattern only (allows creating issues in advance)
  const asyncDailyIssues = issues.data.filter(issue => {
    const titleMatch = issue.title.includes(titleDateStr);
    return titleMatch;
  });

  console.log(`Found ${asyncDailyIssues.length} async daily issues`);

  return {
    asyncDailyIssues,
    dateStr,
    titleDateStr
  };
}

module.exports = { collectAsyncDailyIssues };
