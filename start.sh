#!/bin/bash

# CSV Search Application Startup Script

echo "🚀 Starting CSV Search Application..."
echo "=================================="

# Check if data.csv exists
if [ ! -f "data.csv" ]; then
    echo "❌ data.csv not found!"
    echo "Please place your CSV file in the project root and name it 'data.csv'"
    echo "Or run 'make generate-sample' to create a sample file"
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+"
    exit 1
fi

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+"
    exit 1
fi

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "📦 Installing Node.js dependencies..."
    npm install
fi

# Build the application
echo "🔨 Building application..."
go build -o bin/csv-search cmd/server/main.go

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful!"
echo ""
echo "🌐 Starting servers..."
echo "Backend: http://localhost:8080"
echo "Frontend: http://localhost:3000"
echo ""
echo "Press Ctrl+C to stop both servers"
echo ""

# Start the backend server in background
./bin/csv-search -csv=data.csv -port=8080 &
BACKEND_PID=$!

# Wait a moment for backend to start
sleep 2

# Start the frontend development server
npm run dev &
FRONTEND_PID=$!

# Function to cleanup on exit
cleanup() {
    echo ""
    echo "🛑 Shutting down servers..."
    kill $BACKEND_PID 2>/dev/null
    kill $FRONTEND_PID 2>/dev/null
    echo "✅ Servers stopped"
    exit 0
}

# Set trap to cleanup on script exit
trap cleanup SIGINT SIGTERM

# Wait for both processes
wait
