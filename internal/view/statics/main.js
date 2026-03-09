// Simple timezone conversion using Date.toLocaleString()
function convertTimestamps() {
  // Find all elements with timestamp class
  document.querySelectorAll(".timestamp").forEach((element) => {
    const utcTimeStr = element.textContent.trim();

    // Parse the timestamp (format: "2006-01-02 15:04:05")
    if (utcTimeStr.match(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)) {
      try {
        // Convert to ISO format and parse as UTC
        const utcDate = new Date(utcTimeStr.replace(" ", "T") + "Z");

        // Convert to local time string
        const localTime = utcDate.toLocaleString();

        // Update the element
        element.textContent = localTime;
        element.title = `UTC: ${utcTimeStr}`;
      } catch (error) {
        console.warn("Error converting timestamp:", utcTimeStr, error);
      }
    }
  });
}

// Run when page loads
document.addEventListener("DOMContentLoaded", convertTimestamps);

// Also run when new content is added (for dynamic updates)
if (window.MutationObserver) {
  const observer = new MutationObserver(() => {
    setTimeout(convertTimestamps, 100);
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
  });
}
