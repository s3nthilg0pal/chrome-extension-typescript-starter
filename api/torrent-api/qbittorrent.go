package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type QBittorrentClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	loggedIn   bool
}

func NewQBittorrentClient(baseURL, username, password string) *QBittorrentClient {
	jar, _ := cookiejar.New(nil)
	return &QBittorrentClient{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		loggedIn: false,
	}
}

// Login authenticates with qBittorrent
func (c *QBittorrentClient) Login() error {
	loginURL := fmt.Sprintf("%s/api/v2/auth/login", c.baseURL)

	data := url.Values{}
	data.Set("username", c.username)
	data.Set("password", c.password)

	resp, err := c.httpClient.PostForm(loginURL, data)
	if err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Ok." {
		return fmt.Errorf("login failed: %s", string(body))
	}

	c.loggedIn = true
	return nil
}

// AddTorrent adds a torrent to qBittorrent with the specified category
func (c *QBittorrentClient) AddTorrent(magnetLink, category string) error {
	if !c.loggedIn {
		if err := c.Login(); err != nil {
			return err
		}
	}

	addURL := fmt.Sprintf("%s/api/v2/torrents/add", c.baseURL)

	data := url.Values{}
	data.Set("urls", magnetLink)
	data.Set("category", category)

	resp, err := c.httpClient.PostForm(addURL, data)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add torrent: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// EnsureCategory creates a category if it doesn't exist
func (c *QBittorrentClient) EnsureCategory(category string) error {
	if !c.loggedIn {
		if err := c.Login(); err != nil {
			return err
		}
	}

	createURL := fmt.Sprintf("%s/api/v2/torrents/createCategory", c.baseURL)

	data := url.Values{}
	data.Set("category", category)

	// We don't care if this fails (category might already exist)
	c.httpClient.PostForm(createURL, data)

	return nil
}
