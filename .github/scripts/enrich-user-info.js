/**
 * Enrich issues with user display names from GitHub API
 * 
 * @param {object} github - GitHub API client
 * @param {Array} issues - Array of GitHub issues
 * @returns {Promise<Array>} Array of issues with enriched user information
 */
async function enrichUserInfo(github, issues) {
  const enrichedIssues = [];
  
  for (const issue of issues) {
    try {
      // Get user details from GitHub API
      const { data: user } = await github.rest.users.getByUsername({
        username: issue.user.login
      });
      
      // Add display name to the issue
      enrichedIssues.push({
        ...issue,
        user: {
          ...issue.user,
          displayName: user.name || issue.user.login,
          avatarUrl: user.avatar_url
        }
      });
    } catch (error) {
      console.warn(`Failed to fetch user info for ${issue.user.login}:`, error.message);
      // Fallback: use login as display name
      enrichedIssues.push({
        ...issue,
        user: {
          ...issue.user,
          displayName: issue.user.login,
          avatarUrl: issue.user.avatar_url
        }
      });
    }
  }
  
  return enrichedIssues;
}

module.exports = { enrichUserInfo };
