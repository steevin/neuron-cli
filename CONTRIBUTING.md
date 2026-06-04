# Contributing to Neuron CLI

Hey! Thanks for looking into contributing to Neuron CLI. Whether you want to fix a typo, report a bug, or write a new feature, we really appreciate the help.

Just a quick heads-up: since Neuron CLI is licensed under GPL v3, any code you submit will also be open-sourced under the same license.

## How you can help

* **Found a bug or have an idea?** Check if someone already talked about it in the [issues](https://github.com/steevin/neuron-cli/issues). If not, feel free to open a new one.
* **Want to write code?** For small bugs or typos, just submit a Pull Request. For larger features, please open an issue first to discuss the design so we're on the same page.

## Pull Request Workflow

1. Fork this repo and clone it to your machine:
   ```bash
   git clone https://github.com/YOUR_USERNAME/neuron-cli.git
   cd neuron-cli
   ```
2. Create a new branch for your changes:
   ```bash
   git checkout -b my-feature-branch
   ```
3. Make your changes. Please keep commit messages straightforward and clear.
4. Verify your code formats correctly and tests pass:
   ```bash
   go fmt ./...
   go test ./...
   ```
5. Push to your fork and open a Pull Request (PR) against the `main` branch.

## Quick Dev Guide

You'll need Go (1.21+) installed.

* **Build the binary:** `make build` (or `go build -o bin/neuron cmd/neuron/*.go`)
* **Run unit tests:** `go test ./...`

We try to stick to standard Go idioms and formatting. If you're unsure about how to implement something, open a draft PR and we can work through it together in the comments!

Thanks again! 🧠
