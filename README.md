<div align="center">

# 📊 Perfdeck

**A modern, lightweight, and customizable TUI performance monitor for your terminal.**

[![Go Reference](https://pkg.go.dev/badge/github.com/sumant1122/perfdeck.svg)](https://pkg.go.dev/github.com/sumant1122/perfdeck)
[![Go Report Card](https://goreportcard.com/badge/github.com/sumant1122/perfdeck)](https://goreportcard.com/report/github.com/sumant1122/perfdeck)
[![CI](https://github.com/sumant1122/perfdeck/actions/workflows/ci.yml/badge.svg)](https://github.com/sumant1122/perfdeck/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/sumant1122/perfdeck)](https://github.com/sumant1122/perfdeck/releases)

[Features](#-features) • [Installation](#-installation) • [Usage](#-usage) • [Configuration](#-configuration) • [Contributing](#-contributing)

</div>

---

## 💡 Why Perfdeck?

Traditional performance monitors often overwhelm users with information or lack the flexibility to show exactly what you need. **Perfdeck** solves this by providing:

-   **Consolidation**: View output from multiple diagnostic tools (like `top`, `vmstat`, `netstat`) in one place.
-   **Focus**: A clean, tabbed interface lets you switch between different metrics without terminal clutter.
-   **Visibility**: Real-time sparklines provide an immediate "at-a-glance" health check of your system's core resources.
-   **Flexibility**: Don't like the defaults? Bring your own shell commands via a simple TOML file.

## ✨ Features

-   🚀 **Blazing Fast**: Written in Go with minimal CPU and memory overhead.
-   📂 **Tabbed Navigation**: Organize your monitoring tools into logical, navigable views.
-   📈 **Live Sparklines**: Visual summaries for Load, CPU, Memory, and Network.
-   🎨 **Adaptive Themes**: Seamlessly toggle between Light and Dark modes.
-   ⚙️ **Deeply Configurable**: Custom commands, refresh intervals, and environment-specific settings.
-   🐧 **Cross-Platform**: Intelligent defaults for both Linux and macOS.

## 📸 Screenshots

### Dark Mode (Default)
![Perfdeck Dark Mode](https://github.com/user-attachments/assets/7a94f63d-02ee-4992-b66d-9adf42a16603)

### Light Mode
![Perfdeck Light Mode](https://github.com/user-attachments/assets/19025abf-63e0-49c3-b5a5-c5c07f62468b)

## 🚀 Installation

### 📦 Pre-built Binaries
Download the latest pre-compiled binaries from the [Releases page](https://github.com/sumant1122/perfdeck/releases).

### 🛠️ Using `go install`
```bash
go install github.com/sumant1122/perfdeck@latest
```
> [!NOTE]
> Ensure your `GOPATH/bin` is in your `PATH` (e.g., `export PATH=$PATH:$(go env GOPATH)/bin`) so you can run the `perfdeck` command from any directory.

### 🔨 From Source
```bash
git clone https://github.com/sumant1122/perfdeck.git
cd perfdeck
make build
# Binary will be in the project root
```

## 📖 Usage

Simply run the command to start monitoring with default system tools:
```bash
perfdeck
```

### ⌨️ Key Bindings
| Key | Action |
|:---|:---|
| `Tab` / `Shift+Tab` | Next / Previous Tab |
| `j` / `k` (or `↓`/`↑`) | Scroll through command output |
| `t` | Toggle Light/Dark theme |
| `v` | Display version information |
| `q` / `Esc` / `Ctrl+C` | Exit Perfdeck |

## ⚙️ Configuration

Perfdeck is designed to be personalized. It looks for `perfdeck.toml` in:
1.  `$PERFDECK_CONFIG`
2.  `~/.config/perfdeck/config.toml`
3.  Current working directory

### 📝 Configuration Schema
```toml
# Interval for updating the sparklines and default tabs
global_refresh_interval = "5s"

[[tab]]
title = "Process Explorer"
cmd = ["top", "-b", "-n", "1"]
refresh_interval = "2s" # Specific interval for this tab

[[tab]]
title = "Network Connections"
cmd = ["ss", "-tulpn"]
```

## 🛠 Development

We utilize a simple `Makefile` for a streamlined development experience:

-   `make run`: Start the application in development mode.
-   `make build`: Compile the binary.
-   `make test`: Execute the test suite.
-   `make lint`: Run the golangci-lint (if installed).

## 🤝 Contributing

We love contributions! Whether it's a bug report, a new feature idea, or a documentation improvement, please feel free to:
1.  Check out the [Contributing Guidelines](CONTRIBUTING.md).
2.  Open an [Issue](https://github.com/sumant1122/perfdeck/issues).
3.  Submit a Pull Request.

## 📜 License

Distributed under the **MIT License**. See `LICENSE` for details.

---

<div align="center">
Built with ❤️ using <a href="https://github.com/charmbracelet/bubbletea">Bubble Tea</a>
</div>
