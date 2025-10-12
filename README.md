# EKO - Your Private AI Terminal Companion

> **Zero-config local AI chat with complete privacy** 🛡️

EKO is a minimal, keyboard-driven terminal interface for local AI models. Chat with your AI privately without sending data to external servers, with instant model switching and zero configuration required.

## 🎯 Why EKO?

**Privacy First**: Your conversations never leave your machine. No API keys, no data collection, no cloud dependencies.

**Minimal Setup**: Works out-of-the-box with local Ollama installations. Just run and chat.

**Lightning Fast**: Instant model switching, real-time streaming, and responsive interface.

**Developer Friendly**: Clean, distraction-free interface perfect for coding assistance and technical discussions.

## 🚀 Quick Start

### Prerequisites
- Locally hosted AI installed and running

### Installation

```bash
# Clone and build
git clone https://github.com/thebug-lab/eko.git
cd eko
./build.sh

# Run
./eko
```

**That's it!** EKO will automatically detect your local Ollama instance and available models.

## 💬 Usage

### Basic Commands
- **`i`** - Start typing a message
- **`:`** - Command mode (save, config, etc.)
- **`j/k`** - Scroll through conversation
- **`gg`** - Jump to top
- **`G`** - Jump to bottom
- **`y`** - Copy message to clipboard
- **`q`** - Quit

### Model Switching
```
:config
```
Navigate with `j/k`, select with `Enter`. Switch models instantly without restarting.

### Saving Conversations
```
:save my-conversation
```
Exports to `my-conversation.json` for later reference or sharing.

## 🔧 Configuration

### Custom Ollama Server
Create `~/.config/eko/config.json`:
```json
{
  "model": "llama2",
  "url": "127.0.0.1:11434"
}
```

**Supported URL formats:**
- `127.0.0.1:11434` (auto-adds http://)
- `http://localhost:11434`
- `https://your-local-server.com:11434`

### Default Behavior
- **Model**: `dolphin-phi` (if available)
- **Server**: `http://localhost:11434`
- **Auto-detection**: Finds available models automatically

## 🛡️ Privacy & Security

**Complete Local Control**: All processing happens on your machine. No external API calls, no data transmission.

**No Telemetry**: EKO doesn't collect usage data, analytics, or conversation logs.

**Offline Capable**: Works without internet connection once models are downloaded.

**Secure by Design**: No authentication tokens, no cloud dependencies, no data persistence beyond your control.

## 🎨 Features

### Smart Interface
- **Real-time streaming**: Watch responses as they're generated
- **Message history**: Scroll through entire conversation
- **Model metadata**: See model info and timestamps
- **Responsive design**: Adapts to any terminal size

### Developer Workflow
- **Code assistance**: Perfect for debugging and code review
- **Technical discussions**: Ask questions about your codebase
- **Documentation help**: Generate docs and explanations
- **Learning companion**: Understand complex concepts

### Conversation Management
- **TLDR mode**: Collapse long messages for quick overview
- **Export options**: Save conversations in JSON format
- **Message copying**: Copy any message with `y` + message ID

## 🔧 For Developers

### Building from Source
```bash
# Dependencies
go mod tidy

# Build
go build -o eko main.go

# Run tests
go test ./...
```

### Architecture
- **Bubble Tea**: Terminal UI framework
- Local models: Local model communication
- **Lipgloss**: Styling and layout
- **Zero external deps**: Self-contained binary

### Contributing
1. Fork the repository
2. Create feature branch
3. Add tests for new functionality
4. Submit pull request

## 🌟 Use Cases

### Privacy-Conscious Users
- **Journaling**: Private AI for personal thoughts
- **Sensitive work**: Handle confidential information safely
- **Research**: Explore topics without data collection

## 🚀 Advanced Usage

### Custom Models
EKO works with any Ollama-compatible model:
```bash
# Pull different models
ollama pull codellama
ollama pull mistral
ollama pull qwen

# Switch instantly in EKO
:config
```

### Integration
- **Terminal workflow**: Perfect for CLI-heavy development
- **Scripting**: Can be integrated into automation
- **Remote work**: Secure AI assistance anywhere

## 📋 Requirements

- **Go 1.19+** (for building)
- **Ollama** (for AI models)
- **Terminal** (any modern terminal emulator)

## 🤝 Community

- **Issues**: Report bugs and request features
- **Discussions**: Share use cases and tips
- **Contributions**: Help improve EKO

---

**EKO**: Your local AI, your rules. 🚀
