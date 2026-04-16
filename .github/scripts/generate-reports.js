const fs = require('fs');
const path = require('path');
const { parseIssueBody } = require('./parse-issue');

/**
 * Convert basic markdown to HTML
 * 
 * @param {string} text - Markdown text
 * @returns {string} HTML text
 */
function markdownToHtml(text) {
  if (!text) return text;
  
  // Convert lists with nested support
  const lines = text.split('\n');
  let html = '';
  let listStack = [];
  
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmedLine = line.trim();
    
    const listMatch = line.match(/^(\s*)[-*]\s+(.+)$/);
    
    if (listMatch) {
      const indent = listMatch[1].length;
      const listContent = listMatch[2];
      const level = Math.floor(indent / 2);
      
      while (listStack.length > level + 1) {
        html += '</ul>\n';
        listStack.pop();
      }
      
      if (listStack.length === level) {
        html += '<ul>\n';
        listStack.push(level);
      }
      
      html += `<li>${escapeHtml(listContent)}</li>\n`;
    } else {
      while (listStack.length > 0) {
        html += '</ul>\n';
        listStack.pop();
      }
      
      if (trimmedLine) {
        html += escapeHtml(line) + '\n';
      } else {
        html += '\n';
      }
    }
  }
  
  while (listStack.length > 0) {
    html += '</ul>\n';
    listStack.pop();
  }
  
  return html;
}

function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

function generateReports(dailyIssues, titleDateStr, context, reportIssueUrl = null) {
  const parkingLotItems = [];
  const markdownBody = generateMarkdownReport(dailyIssues, titleDateStr, context, parkingLotItems);
  
  return {
    markdownBody,
    parkingLotItems
  };
}

function generateMarkdownReport(dailyIssues, titleDateStr, context, parkingLotItems) {
  let markdownBody = `# 📅 Async Daily Summary - ${titleDateStr}\n\n`;
  markdownBody += `**Team Updates:** ${dailyIssues.length} member(s) reported today\n\n`;
  markdownBody += `---\n\n`;
  
  for (const issue of dailyIssues) {
    const author = issue.user.login;
    const displayName = issue.user.displayName || author;
    const avatarUrl = issue.user.avatarUrl || issue.user.avatar_url;
    const issueNumber = issue.number;
    const issueUrl = issue.html_url;
    const sections = parseIssueBody(issue.body);
    
    markdownBody += `## <img src="${avatarUrl}" width="24" height="24" style="border-radius: 50%; vertical-align: middle;"> ${displayName} (@${author}) ${sections.parkingLot ? '🚨 **PARKING LOT**' : ''} | #${issueNumber}\n\n`;
    
    markdownBody += `### ✅ What did you do yesterday?\n\n`;
    markdownBody += sections.yesterday ? sections.yesterday + '\n\n' : '_No information provided_\n\n';
    
    markdownBody += `### 🎯 What will you do today?\n\n`;
    markdownBody += sections.today ? sections.today + '\n\n' : '_No information provided_\n\n';
    
    if (sections.blockers) {
      markdownBody += `### 🚧 Any blockers?\n\n`;
      markdownBody += sections.blockers + '\n\n';
    }
    
    if (sections.parkingLotDetails) {
      markdownBody += `### 📝 Parking Lot Details\n\n`;
      markdownBody += sections.parkingLotDetails + '\n\n';
    }
    
    if (sections.additionalComments) {
      markdownBody += `### 💬 Additional Comments\n\n`;
      markdownBody += sections.additionalComments + '\n\n';
    }
    
    markdownBody += `---\n\n`;
    
    if (sections.parkingLot) {
      parkingLotItems.push({ author, displayName, avatarUrl, issueUrl, issueNumber });
    }
  }
  
  if (parkingLotItems.length > 0) {
    markdownBody += `## 🚨 Parking Lot Items (${parkingLotItems.length})\n\n`;
    markdownBody += `The following team members have requested a parking lot discussion or escalation:\n\n`;
    
    for (const item of parkingLotItems) {
      markdownBody += `- <img src="${item.avatarUrl}" width="20" height="20" style="border-radius: 50%; vertical-align: middle;"> **${item.displayName} (@${item.author})** - [Issue #${item.issueNumber}](${item.issueUrl})\n`;
    }
    
    markdownBody += `\n`;
  }
  
  markdownBody += `---\n\n`;
  markdownBody += `_This is an automated async daily report generated from GitHub issues._\n`;
  markdownBody += `_Repository: ${context.repo.owner}/${context.repo.repo}_\n`;
  
  return markdownBody;
}

module.exports = { generateReports };
