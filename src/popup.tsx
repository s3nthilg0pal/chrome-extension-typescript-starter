import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";

interface LinkData {
  href: string;
  text: string;
  status?: "pending" | "success" | "error";
}

const API_URL = "http://localhost:9080/api/torrent";
const MEDIA_API_URL = "http://localhost:9080/api/media";

interface ImdbInfo {
  title: string;
  year: string;
  type: string;
  error?: string;
}

const Popup = () => {
  const [count, setCount] = useState(0);
  const [currentURL, setCurrentURL] = useState<string>();
  const [links, setLinks] = useState<LinkData[]>([]);
  const [loading, setLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const [imdbInfo, setImdbInfo] = useState<ImdbInfo | null>(null);
  const [loadingImdb, setLoadingImdb] = useState(false);
  const [addingMedia, setAddingMedia] = useState(false);
  const [mediaStatus, setMediaStatus] = useState<{ success: boolean; message: string } | null>(null);

  useEffect(() => {
    chrome.action.setBadgeText({ text: count.toString() });
  }, [count]);

  useEffect(() => {
    chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
      setCurrentURL(tabs[0].url);
    });
  }, []);

  const changeBackground = () => {
    chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
      const tab = tabs[0];
      if (tab.id) {
        chrome.tabs.sendMessage(
          tab.id,
          {
            color: "#555555",
          },
          (msg) => {
            console.log("result message:", msg);
          }
        );
      }
    });
  };

  const getLinks = () => {
    setLoading(true);
    chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
      const tab = tabs[0];
      console.log("Current tab:", tab);
      if (tab.id) {
        chrome.tabs.sendMessage(
          tab.id,
          { action: "getLinks" },
          (response: { links: LinkData[] } | undefined) => {
            console.log("Response:", response);
            console.log("Chrome error:", chrome.runtime.lastError);
            if (chrome.runtime.lastError) {
              console.error("Error:", chrome.runtime.lastError.message);
              alert("Error: " + chrome.runtime.lastError.message + "\n\nTry refreshing the page.");
            } else if (response && response.links) {
              setLinks(response.links);
            }
            setLoading(false);
          }
        );
      }
    });
  };

  const sendToApi = async (magnetLink: string, index: number) => {
    try {
      const response = await fetch(API_URL, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ magnet_link: magnetLink }),
      });

      if (response.ok) {
        setLinks((prev) =>
          prev.map((link, i) =>
            i === index ? { ...link, status: "success" } : link
          )
        );
      } else {
        throw new Error(`HTTP ${response.status}`);
      }
    } catch (error) {
      console.error("API Error:", error);
      setLinks((prev) =>
        prev.map((link, i) =>
          i === index ? { ...link, status: "error" } : link
        )
      );
    }
  };

  const sendAllToApi = async () => {
    setSending(true);
    for (let i = 0; i < links.length; i++) {
      if (links[i].status !== "success") {
        await sendToApi(links[i].href, i);
      }
    }
    setSending(false);
  };

  const getImdbTitle = () => {
    setLoadingImdb(true);
    setImdbInfo(null);
    chrome.tabs.query({ active: true, currentWindow: true }, function (tabs) {
      const tab = tabs[0];
      if (tab.id) {
        chrome.tabs.sendMessage(
          tab.id,
          { action: "getImdbTitle" },
          (response: ImdbInfo | undefined) => {
            if (chrome.runtime.lastError) {
              setImdbInfo({ title: "", year: "", type: "", error: chrome.runtime.lastError.message });
            } else if (response) {
              setImdbInfo(response);
            }
            setLoadingImdb(false);
          }
        );
      }
    });
  };

  const addToMedia = async () => {
    if (!imdbInfo || imdbInfo.error) return;
    
    setAddingMedia(true);
    setMediaStatus(null);
    
    try {
      const response = await fetch(MEDIA_API_URL, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name: imdbInfo.title,
          type: imdbInfo.type,
          year: imdbInfo.year || undefined,
        }),
      });

      if (response.ok) {
        const data = await response.json().catch(() => ({}));
        setMediaStatus({
          success: true,
          message: `Added "${imdbInfo.title}" to ${imdbInfo.type === "tv" ? "Sonarr" : "Radarr"}!`,
        });
      } else {
        const errorData = await response.json().catch(() => ({ error: `HTTP ${response.status}` }));
        throw new Error(errorData.error || `HTTP ${response.status}`);
      }
    } catch (error) {
      console.error("Media API Error:", error);
      setMediaStatus({
        success: false,
        message: `Failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      });
    }
    
    setAddingMedia(false);
  };

  return (
    <>
      <ul style={{ minWidth: "700px" }}>
        <li>Current URL: {currentURL}</li>
        <li>Current Time: {new Date().toLocaleTimeString()}</li>
      </ul>
      <button
        onClick={() => setCount(count + 1)}
        style={{ marginRight: "5px" }}
      >
        count up
      </button>
      <button onClick={changeBackground} style={{ marginRight: "5px" }}>
        change background
      </button>
      <button onClick={getLinks} disabled={loading} style={{ marginRight: "5px" }}>
        {loading ? "Loading..." : "Get Magnet Links"}
      </button>
      <button onClick={getImdbTitle} disabled={loadingImdb}>
        {loadingImdb ? "Loading..." : "Get IMDB Title"}
      </button>

      {imdbInfo && (
        <div style={{ marginTop: "10px", padding: "10px", backgroundColor: "#f5f5f5", borderRadius: "5px" }}>
          {imdbInfo.error ? (
            <p style={{ color: "red" }}>❌ {imdbInfo.error}</p>
          ) : (
            <>
              <p><strong>Title:</strong> {imdbInfo.title}</p>
              <p><strong>Year:</strong> {imdbInfo.year || "N/A"}</p>
              <p><strong>Type:</strong> {imdbInfo.type === "tv" ? "📺 TV Series" : "🎬 Movie"}</p>
              <button
                onClick={addToMedia}
                disabled={addingMedia}
                style={{
                  marginTop: "10px",
                  padding: "8px 16px",
                  backgroundColor: imdbInfo.type === "tv" ? "#5cad4a" : "#ffc107",
                  color: imdbInfo.type === "tv" ? "white" : "black",
                  border: "none",
                  borderRadius: "4px",
                  cursor: addingMedia ? "not-allowed" : "pointer",
                  fontWeight: "bold",
                }}
              >
                {addingMedia
                  ? "Adding..."
                  : `Add to ${imdbInfo.type === "tv" ? "Sonarr" : "Radarr"}`}
              </button>
              {mediaStatus && (
                <p style={{ marginTop: "10px", color: mediaStatus.success ? "green" : "red" }}>
                  {mediaStatus.success ? "✅" : "❌"} {mediaStatus.message}
                </p>
              )}
            </>
          )}
        </div>
      )}

      {links.length > 0 && (
        <div style={{ marginTop: "10px", maxHeight: "300px", overflowY: "auto" }}>
          <h4>
            Found {links.length} magnet links:
            <button
              onClick={sendAllToApi}
              disabled={sending}
              style={{ marginLeft: "10px", fontSize: "12px" }}
            >
              {sending ? "Sending..." : "Send All to API"}
            </button>
          </h4>
          <ul style={{ paddingLeft: "20px", fontSize: "12px" }}>
            {links.map((link, index) => (
              <li key={index} style={{ marginBottom: "5px" }}>
                <span style={{ marginRight: "5px" }}>
                  {link.status === "success" && "✅"}
                  {link.status === "error" && "❌"}
                  {!link.status && "⏳"}
                </span>
                <button
                  onClick={() => sendToApi(link.href, index)}
                  disabled={link.status === "success"}
                  style={{ marginRight: "5px", fontSize: "10px" }}
                >
                  Send
                </button>
                <span style={{ color: "#333" }}>
                  {link.text.length > 50 ? link.text.substring(0, 50) + "..." : link.text}
                </span>
                <div style={{ fontSize: "10px", color: "#666", marginLeft: "50px" }}>
                  {link.href.length > 80 ? link.href.substring(0, 80) + "..." : link.href}
                </div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </>
  );
};

const root = createRoot(document.getElementById("root")!);

root.render(
  <React.StrictMode>
    <Popup />
  </React.StrictMode>
);
