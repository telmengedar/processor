package process

import "os/exec"

func configureChild(cmd *exec.Cmd) func() {
	kill := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	cmd.Cancel = func() error { kill(); return nil }
	return kill
}
