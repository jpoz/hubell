package notify

import "os/exec"

// Say uses the macOS say command to speak the given text aloud.
func Say(text string) {
	cmd := exec.Command("say", text)
	_ = cmd.Start()
}
