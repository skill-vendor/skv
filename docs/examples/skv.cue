// Example skv.cue configuration.

skv: {
	tools: {
		exclude: ["opencode"]
	}
	skills: [
		{
			name: "release-notes"
			repo: "https://github.com/acme/skill-pack"
			path: "skills/release-notes"
			ref:  "v1.2.3"
		},
		{
			name:  "local-helper"
			local: "./.skv/skills/local-helper"
		},
	]
}
