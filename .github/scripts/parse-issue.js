/**
 * Parse issue body to extract async daily information
 * 
 * @param {string} body - The issue body text
 * @returns {object} Object containing yesterday, today, blockers, parkingLotDetails, additionalComments sections and parkingLot flag
 */
function parseIssueBody(body) {
  const sections = {
    yesterday: '',
    today: '',
    blockers: '',
    parkingLot: false,
    parkingLotDetails: '',
    additionalComments: ''
  };
  
  if (!body) return sections;
  
  // Helper function to clean empty responses
  const cleanEmptyResponse = (text) => {
    if (!text) return '';
    const trimmed = text.trim();
    if (trimmed === '_No response_' || trimmed.toLowerCase() === 'none' || trimmed === '_None._') {
      return '';
    }
    return trimmed;
  };
  
  // Handle both GitHub form format (###) and rendered markdown (##)
  const yesterdayMatch = body.match(/### ✅ What did you do yesterday\?\s*([\s\S]*?)(?=###|##|$)/) ||
                         body.match(/## ✅ What did you do yesterday\?\s*([\s\S]*?)(?=##|$)/);
  const todayMatch = body.match(/### 🎯 What will you do today\?\s*([\s\S]*?)(?=###|##|$)/) ||
                     body.match(/## 🎯 What will you do today\?\s*([\s\S]*?)(?=##|$)/);
  const blockersMatch = body.match(/### 🚧 Any blockers\?\s*([\s\S]*?)(?=###|##|$)/) ||
                      body.match(/## 🚧 Any blockers\?\s*([\s\S]*?)(?=##|$)/);
  const parkingLotMatch = body.match(/- \[x\].*Parking Lot/i) || 
                          body.match(/- ✅ Yes, I need a Parking Lot/i);
  const parkingLotDetailsMatch = body.match(/### 📝 Parking Lot Details\s*([\s\S]*?)(?=###|##|$)/) ||
                                 body.match(/## 📝 Parking Lot Details\s*([\s\S]*?)(?=##|$)/);
  const additionalCommentsMatch = body.match(/### 💬 Additional Comments\s*([\s\S]*?)(?=###|##|$)/) ||
                                  body.match(/## 💬 Additional Comments\s*([\s\S]*?)(?=##|$)/);
  
  if (yesterdayMatch) sections.yesterday = yesterdayMatch[1].trim();
  if (todayMatch) sections.today = todayMatch[1].trim();
  if (blockersMatch) sections.blockers = cleanEmptyResponse(blockersMatch[1]);
  if (parkingLotDetailsMatch) sections.parkingLotDetails = cleanEmptyResponse(parkingLotDetailsMatch[1]);
  if (additionalCommentsMatch) sections.additionalComments = cleanEmptyResponse(additionalCommentsMatch[1]);
  
  sections.parkingLot = !!parkingLotMatch;
  
  return sections;
}

module.exports = { parseIssueBody };
