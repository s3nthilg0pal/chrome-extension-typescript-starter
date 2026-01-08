console.log("Content script loaded on:", window.location.href);

chrome.runtime.onMessage.addListener(function (msg, sender, sendResponse) {
  console.log("Message received:", msg);
  if (msg.color) {
    console.log("Receive color = " + msg.color);
    document.body.style.backgroundColor = msg.color;
    sendResponse("Change color to " + msg.color);
  } else if (msg.action === "getLinks") {
    const links = document.querySelectorAll("a");
    console.log("Found links:", links.length);
    const linkData = Array.from(links)
      .filter((link) => link.href && link.href.startsWith("magnet:?"))
      .map((link) => ({
        href: link.href,
        text: link.textContent?.trim() || "(no text)",
      }))
      .slice(0, 100); // Limit to 100 links
    console.log("Sending magnet links:", linkData);
    sendResponse({ links: linkData });
  } else if (msg.action === "getImdbTitle") {
    // Check if we're on IMDB
    if (!window.location.hostname.includes("imdb.com")) {
      sendResponse({ error: "Not on IMDB page" });
      return true;
    }
    
    // Try to get the title from various IMDB selectors
    let title = "";
    let year = "";
    let type = "movie"; // default to movie
    
    // Main title (works for both movies and TV shows)
    const titleElement = document.querySelector('[data-testid="hero__pageTitle"]') ||
                         document.querySelector('h1[data-testid="hero-title-block__title"]') ||
                         document.querySelector("h1");
    
    if (titleElement) {
      title = titleElement.textContent?.trim() || "";
    }
    
    // Try to get year from multiple sources
    // Method 1: metadata area links
    const metadataLinks = document.querySelectorAll('[data-testid="hero-title-block__metadata"] a, [data-testid="hero-title-block__metadata"] span, [data-testid="hero-title-block__metadata"] li');
    for (const el of metadataLinks) {
      const text = el.textContent || "";
      const yearMatch = text.match(/^(\d{4})(?:–|$)/);
      if (yearMatch) {
        year = yearMatch[1];
        break;
      }
    }
    
    // Method 2: Look in release info link
    if (!year) {
      const releaseLink = document.querySelector('a[href*="/releaseinfo"]');
      if (releaseLink) {
        const yearMatch = releaseLink.textContent?.match(/\d{4}/);
        if (yearMatch) {
          year = yearMatch[0];
        }
      }
    }
    
    // Method 3: Schema.org JSON-LD
    if (!year) {
      const schemaScript = document.querySelector('script[type="application/ld+json"]');
      if (schemaScript) {
        try {
          const schema = JSON.parse(schemaScript.textContent || "");
          if (schema.datePublished) {
            const yearMatch = schema.datePublished.match(/\d{4}/);
            if (yearMatch) {
              year = yearMatch[0];
            }
          }
        } catch (e) {
          // ignore
        }
      }
    }
    
    // Method 4: Look anywhere in metadata for 4-digit year
    if (!year) {
      const metaArea = document.querySelector('[data-testid="hero-title-block__metadata"]');
      if (metaArea) {
        const yearMatch = metaArea.textContent?.match(/\b(19|20)\d{2}\b/);
        if (yearMatch) {
          year = yearMatch[0];
        }
      }
    }
    
    console.log("Year extraction debug:", { year });
    
    // Determine if it's a TV series - check multiple indicators
    const metadataArea = document.querySelector('[data-testid="hero-title-block__metadata"]');
    const pageContent = document.body.innerText.toLowerCase();
    
    // Check for TV indicators in metadata
    if (metadataArea) {
      const metaText = metadataArea.textContent?.toLowerCase() || "";
      if (metaText.includes("tv series") || 
          metaText.includes("tv mini series") || 
          metaText.includes("episode") ||
          metaText.includes("tv special")) {
        type = "tv";
      }
    }
    
    // Check for episode guide link (only TV shows have this)
    const episodeGuide = document.querySelector('[data-testid="episodes-header"]') ||
                         document.querySelector('a[href*="/episodes"]') ||
                         document.querySelector('[data-testid="tm-box-episodes-702560"]');
    if (episodeGuide) {
      type = "tv";
    }
    
    // Check URL pattern - TV shows often have episode info
    if (window.location.pathname.includes("/episodes")) {
      type = "tv";
    }
    
    // Check for "Seasons" or "Episodes" section
    const seasonsSection = document.querySelector('[data-testid="episodes-browse-episodes"]') ||
                           document.querySelector('section[data-testid="Episodes"]');
    if (seasonsSection) {
      type = "tv";
    }

    // Look for schema.org type in page
    const schemaScript = document.querySelector('script[type="application/ld+json"]');
    if (schemaScript) {
      try {
        const schema = JSON.parse(schemaScript.textContent || "");
        if (schema["@type"] === "TVSeries" || schema["@type"] === "TVEpisode") {
          type = "tv";
        }
      } catch (e) {
        // ignore parse errors
      }
    }
    
    console.log("IMDB Title:", { title, year, type });
    sendResponse({ title, year, type });
  } else {
    sendResponse("Color message is none.");
  }
  return true; // Required for async sendResponse
});
