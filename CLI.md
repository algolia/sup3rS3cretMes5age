# Command Line Interface (CLI) Usage

## Overview

The sup3rS3cretMes5age CLI integration allows you to quickly create secure, self-destructing message links directly from your terminal. This is particularly useful for:

- **Sharing sensitive configuration files** with team members
- **Sending API keys or passwords** securely
- **Sharing command outputs** that contain sensitive information
- **Quick secure file sharing** without leaving the terminal
- **Automating secure message creation** in scripts and workflows

## Example Usage

```bash
# Share a secret file
$ o secret-config.json
https://your-domain.com/getmsg?token=abc123def456

# Share command output
$ kubectl get secrets -o yaml | o
https://your-domain.com/getmsg?token=xyz789uvw012

# Share multiple files
$ o database.env api-keys.txt
https://your-domain.com/getmsg?token=mno345pqr678
```

The generated URL can only be accessed **once** and will self-destruct after being viewed, ensuring your sensitive data remains secure.

## Shell Integration

### Prerequisites

Before using any of the shell functions below, ensure you have:
- `curl` installed
- `jq` installed (for JSON parsing)
- Access to a sup3rS3cretMes5age deployment

Replace `https://your-domain.com` in all examples with your actual sup3rS3cretMes5age deployment URL.

---

### Bash

Add this function to your `~/.bashrc`:

```bash
o() {
    if [ $# -eq 0 ]; then
        # Read from stdin if no arguments
        curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'
    else
        # Read from files
        cat "$@" | curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'
    fi
}
```

**Usage:**
```bash
# From file
o secret.txt

# From stdin
echo "secret message" | o

# From command output
ps aux | o

# Multiple files
o file1.txt file2.txt
```

---

### Zsh

Add this function to your `~/.zshrc`:

```zsh
o() {
    if [ $# -eq 0 ]; then
        # Read from stdin if no arguments
        curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'
    else
        # Read from files
        cat "$@" | curl -sF 'msg=<-' https://your-domain.com/secret | jq -r .token | awk '{print "https://your-domain.com/getmsg?token="$1}'
    fi
}
```

**Advanced Zsh version with error handling:**
```zsh
o() {
    local url="https://your-domain.com"
    local response
    
    if [ $# -eq 0 ]; then
        response=$(curl -sF 'msg=<-' "$url/secret")
    else
        response=$(cat "$@" | curl -sF 'msg=<-' "$url/secret")
    fi
    
    if [ $? -eq 0 ]; then
        echo "$response" | jq -r .token | awk -v url="$url" '{print url"/getmsg?token="$1}'
    else
        echo "Error: Failed to create secure message" >&2
        return 1
    fi
}
```

---

### Fish Shell

Add this function to your `~/.config/fish/config.fish`:

```fish
function o
    set -l url "https://your-domain.com"
    
    if test (count $argv) -eq 0
        # Read from stdin
        set response (curl -sF 'msg=<-' "$url/secret")
    else
        # Read from files
        set response (cat $argv | curl -sF 'msg=<-' "$url/secret")
    end
    
    if test $status -eq 0
        echo $response | jq -r .token | awk -v url="$url" '{print url"/getmsg?token="$1}'
    else
        echo "Error: Failed to create secure message" >&2
        return 1
    end
end
```

**Fish-specific features:**
```fish
# With Fish's command substitution
set secret_url (echo "my secret" | o)
echo "Share this URL: $secret_url"

# Using Fish's pipe to variable
echo "secret data" | o | read -g secret_link
```

---

### Windows Subsystem for Linux (WSL)

For WSL (Ubuntu/Debian), add this to your `~/.bashrc`:

```bash
o() {
    local url="https://your-domain.com"
    local response
    
    # Handle Windows line endings
    if [ $# -eq 0 ]; then
        response=$(curl -sF 'msg=<-' "$url/secret")
    else
        response=$(cat "$@" | dos2unix | curl -sF 'msg=<-' "$url/secret")
    fi
    
    if [ $? -eq 0 ]; then
        token=$(echo "$response" | jq -r .token)
        echo "$url/getmsg?token=$token"
        
        # Optional: Copy to Windows clipboard
        if command -v clip.exe >/dev/null 2>&1; then
            echo "$url/getmsg?token=$token" | clip.exe
            echo "(URL copied to Windows clipboard)"
        fi
    else
        echo "Error: Failed to create secure message" >&2
        return 1
    fi
}
```

**WSL-specific usage:**
```bash
# Share a Windows file
o /mnt/c/Users/username/secret.txt

# Copy result to Windows clipboard automatically
echo "secret" | o
```

---

## Advanced Usage

### Environment Configuration

Create a configuration file `~/.sup3rsecret` to avoid hardcoding URLs:

```bash
# ~/.sup3rsecret
SUPERSECRET_URL="https://your-domain.com"
SUPERSECRET_COPY_TO_CLIPBOARD=true
SUPERSECRET_SHOW_QR=false
```

Then modify your shell function to source this config:

```bash
o() {
    # Load config
    [ -f ~/.sup3rsecret ] && source ~/.sup3rsecret
    local url="${SUPERSECRET_URL:-https://your-domain.com}"
    
    # ... rest of function
}
```

### QR Code Generation

Add QR code generation for easy mobile sharing:

```bash
o() {
    # ... existing function logic ...
    
    local secret_url="$url/getmsg?token=$token"
    echo "$secret_url"
    
    # Generate QR code if requested
    if [ "$SUPERSECRET_SHOW_QR" = "true" ] && command -v qrencode >/dev/null 2>&1; then
        echo "QR Code:"
        qrencode -t ANSIUTF8 "$secret_url"
    fi
}
```

### Expiration Time

Some deployments might support custom expiration times:

```bash
o() {
    local ttl="${1:-3600}"  # Default 1 hour
    shift
    
    if [ $# -eq 0 ]; then
        response=$(curl -sF 'msg=<-' -F "ttl=$ttl" "$url/secret")
    else
        response=$(cat "$@" | curl -sF 'msg=<-' -F "ttl=$ttl" "$url/secret")
    fi
    
    # ... rest of function
}
```

## Security Considerations

1. **HTTPS Only**: Always use HTTPS URLs to prevent interception
2. **Trusted Networks**: Avoid using on untrusted networks
3. **Shell History**: Consider using `set +o history` before running sensitive commands
4. **File Permissions**: Ensure your shell config files have appropriate permissions (`chmod 600 ~/.bashrc`)
5. **Cleanup**: The message will self-destruct after being read once

## Troubleshooting

### Common Issues

**"jq: command not found"**
```bash
# Ubuntu/Debian
sudo apt-get install jq

# macOS
brew install jq

# CentOS/RHEL
sudo yum install jq
```

**"curl: command not found"**
```bash
# Ubuntu/Debian
sudo apt-get install curl

# CentOS/RHEL
sudo yum install curl
```

**Function not found after adding to config**
```bash
# Reload your shell configuration
source ~/.bashrc  # or ~/.zshrc, ~/.config/fish/config.fish
```

**SSL certificate errors**
```bash
# For self-signed certificates (NOT recommended for production)
curl -k -sF 'msg=<-' https://your-domain.com/secret
```

### Testing Your Setup

Test your function with a simple message:

```bash
echo "test message" | o
```

You should receive a URL that you can open in your browser to verify the message appears and then self-destructs.
