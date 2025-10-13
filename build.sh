#!/bin/bash

# Sniplicity Build Script
# Run from the main project directory

set -e  # Exit on any error

echo "🔨 Building sniplicity..."

# Change to golang directory and build
cd golang
go build -o sniplicity ./cmd

echo "✅ Build complete! Binary created at: golang/sniplicity"

# Optional: Run if requested
if [ "$1" == "run" ]; then
    echo "🚀 Starting sniplicity..."
    ./sniplicity -s
elif [ "$1" == "test" ]; then
    echo "🧪 Testing sniplicity..."
    ./sniplicity -s &
    SERVER_PID=$!
    sleep 2
    echo "Testing HTTP endpoint..."
    curl -s http://127.0.0.1:3000/sniplicity/api/config > /dev/null && echo "✅ HTTP endpoint working"
    kill $SERVER_PID
    echo "✅ Test complete"
fi

echo "💡 Usage:"
echo "  ./build.sh          - Build only"
echo "  ./build.sh run       - Build and run"
echo "  ./build.sh test      - Build and quick test"