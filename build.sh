#!/bin/bash

# Build script for EKO v1

echo "Building EKO v3..."

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed or not in PATH"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

# Clean and tidy modules
echo "Tidying Go modules..."
go mod tidy

# Build the application
echo "Building application..."
go build -o eko main.go

if [ $? -eq 0 ]; then
    echo "Build successful! Executable created: ./eko"
    echo ""
    echo "To run the application:"
    echo "  ./eko"
    echo ""
    echo "To setup configuration:"
    echo "  ./setup-config.sh"
else
    echo "Build failed!"
    exit 1
fi
