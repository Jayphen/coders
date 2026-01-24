# Homebrew Tap for Coders

This directory contains the Homebrew formula for installing the `coders` CLI tool on macOS.

## Installation

```bash
# Tap this repository
brew tap Jayphen/coders

# Install coders
brew install coders
```

Or install directly without tapping:

```bash
brew install Jayphen/coders/coders
```

## Upgrade

```bash
brew upgrade coders
```

## Uninstall

```bash
brew uninstall coders
brew untap Jayphen/coders  # Optional: remove the tap
```

## Formula Details

The formula automatically:
- Detects your Mac architecture (Apple Silicon or Intel)
- Downloads the appropriate binary from GitHub releases
- Verifies the download with SHA256 checksums
- Installs the `coders` command to your PATH

## Supported Platforms

- macOS (Apple Silicon / ARM64)
- macOS (Intel / AMD64)

For Linux users, please use the [install script](https://github.com/Jayphen/coders#install-via-script-linuxmacos) instead.

## Version

Current version: 1.0.0

## Links

- [Main Repository](https://github.com/Jayphen/coders)
- [GitHub Releases](https://github.com/Jayphen/coders/releases)
- [Documentation](https://github.com/Jayphen/coders#readme)
