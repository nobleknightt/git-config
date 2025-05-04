# Git Config

A CLI tool based on the [Multi-Account Git Setup with SSH and Commit Signing](https://ajaydandge.dev/blog/multi-account-git-config) guide, written in Go.

## Install it from GitHub Releases

You can download the latest release from the GitHub releases page.

1. Go to the [GitHub Releases](https://github.com/nobleknightt/git-config/releases) page.
2. Download the appropriate version for your OS:

   * For **Windows**, download the `.zip` file.
   * For **Linux/macOS**, download the `.tar.gz` file.
3. Extract the archive and place the `git-config` binary in a directory included in your `PATH`.

### Example installation for Linux:

```sh
curl -LO https://github.com/nobleknightt/git-config/releases/download/v0.1.0/git-config-linux-amd64.tar.gz
tar -xvf git-config-linux-amd64.tar.gz
sudo mv git-config /usr/local/bin/
```

### Example installation for Windows:

1. Download the `.zip` file for Windows from the [Releases page](https://github.com/nobleknightt/git-config/releases).
2. Extract the contents of the `.zip` file.
3. Move the `git-config.exe` binary to a directory included in your `PATH`, for example `C:\Program Files\git-config`.

## Install using `go install`

If you have Go installed, you can also install the tool using `go install`:

1. Run the following command to install `git-config`:

```sh
go install github.com/nobleknightt/git-config@latest
```

2. After installation, the `git-config` binary will be placed in your Go workspace's `bin` directory:

   * On **Linux/macOS**, this is usually located at `$HOME/go/bin`.
   * On **Windows**, it will be placed in the Go workspace’s `bin` directory, typically located at `C:\Users\<YourUser>\go\bin`.

## Set up Git Config

Once installed, navigate to the directory where you want to set up your git configuration.

```sh
cd /path/to/your/git-config-directory
git-config
```

This will initiate the setup process, allowing you to configure your multi-account Git setup with SSH keys and commit signing.

---

### Notes for Windows Users:

* Ensure that you have added the path to the folder containing `git-config.exe` to your system's `PATH` environment variable for easy access from the command line.
* You can check if `git-config` is installed correctly by running `git-config version` from your terminal.
