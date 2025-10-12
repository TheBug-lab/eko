#!/bin/bash

# Setup script for EKO v1 configuration

echo "Setting up EKO v1 configuration..."

# Create config directory
mkdir -p ~/.config/eko

# Copy dummy config if it doesn't exist
if [ ! -f ~/.config/eko/config.json ]; then
    cp config-example.json ~/.config/eko/config.json
    echo "Created ~/.config/eko/config.json with default model"
else
    echo "Configuration file already exists at ~/.config/eko/config.json"
fi

echo "Configuration setup complete!"
echo "You can edit ~/.config/eko/config.json to change the default model"
