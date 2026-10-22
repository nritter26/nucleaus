package web

import "net/http"

// handleGitLab serves the animated git recovery simulator. The scenarios and
// command engine live in static/gitlab.js so the page stays a single request.
func handleGitLab(w http.ResponseWriter, r *http.Request) {
	Render(w, "gitlab_page", pageData{
		Title: "Git Recovery Lab",
		Page:  "gitlab",
		Nav:   nav(),
	})
}
