#!/bin/bash

# Start Backend
echo "Starting Secure Backend..."
cd backend
go mod tidy
go run main.go &
BACKEND_PID=$!
cd ..

# Start Frontend
echo "Starting Secure Frontend..."
cd frontend
npm install
npm run dev &
FRONTEND_PID=$!
cd ..

echo "TitanSystem Secure Edition is running!"
echo "Backend: http://localhost:3000"
echo "Frontend: http://localhost:5173"
echo "Press Ctrl+C to stop both servers."

# Trap Ctrl+C to kill both processes
trap "kill $BACKEND_PID $FRONTEND_PID; exit" SIGINT

wait
