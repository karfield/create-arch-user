package main

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/codegangsta/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = path.Base(os.Args[0])
	app.Usage = "create archlinux user in btrfs contained '/home'"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "username, user, u",
			Usage: "set username",
		},
		cli.StringFlag{
			Name:  "homedir, home",
			Usage: "set home-dir name",
		},
		cli.BoolFlag{
			Name:  "sudoer, wheel, su",
			Usage: "is a sudoer user which belongs to wheel group",
		},
		cli.StringFlag{
			Name:  "password, pw",
			Usage: "set password",
		},
		cli.StringFlag{
			Name:  "shell",
			Usage: "set login shell",
			Value: "/bin/bash",
		},
	}
	app.Action = func(c *cli.Context) {
		username := c.GlobalString("username")
		if username == "" {
			panic("missing '--username'")
		}
		homedir := c.GlobalString("homedir")
		if !filepath.IsAbs(homedir) {
			if homedir != "" {
				homedir = path.Join("/home", homedir)
			} else {
				homedir = path.Join("/home", username)
			}
		}
		password := c.GlobalString("password")

		// create user
		useradd := []string{
			"--base-dir", "/home",
			"--home-dir", homedir,
			"--no-create-home",
		}
		if password != "" {
			useradd = append(useradd, "--password", password)
		}
		useradd = append(useradd,
			"--user-group",
			"--shell", c.GlobalString("shell"),
			username,
		)
		execute("useradd", useradd...)

		// get gid
		gidPattern := regexp.MustCompile(".*gid=(\\d+)\\(([^\\)]+)\\).*")
		gid := -1
		if ss := gidPattern.FindStringSubmatch(strings.Trim(executeOuput("id", username), " \t\r\n")); len(ss) > 0 {
			gid, _ = strconv.Atoi(ss[1])
		}
		if gid < 0 {
			panic("cannot get user group id")
		}

		// create home-dir and change permission to user
		execute("btrfs", "subvolume", "create", "-i", strconv.Itoa(gid), path.Base(homedir))
		execute("chown", "-R", username, homedir)
		execute("chgrp", "-R", username, homedir)

		if c.GlobalBool("wheel") {
			execute("usermod", "-a", "-G", "wheel", username)
			addToSudoers(username)
		}
	}
	app.Run(os.Args)
}

func execute(cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = "/home"
	c.Run()
}

func executeOuput(cmd string, args ...string) string {
	c := exec.Command(cmd, args...)
	c.Dir = "/home"
	if out, err := c.CombinedOutput(); err != nil {
		return ""
	} else {
		return string(out)
	}
}

const _SUDOER_CONFIG = `
{{.USERNAME}} ALL=(ALL)ALL
`

func addToSudoers(username string) {
	f, err := os.Create(path.Join("/etc/sudoers.d", username))
	if err != nil {
		return
	}
	defer f.Close()
	t, err := template.New("sudoer-config").Parse(_SUDOER_CONFIG)
	if err != nil {
		return
	}
	if err := t.Execute(f, struct {
		USERNAME string
	}{
		USERNAME: username,
	}); err != nil {
	}
}
